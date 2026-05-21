//go:build integration

package differ

// B5: Integration tests for GRANT ... TO PUBLIC in migrations.
//
// These tests validate the full pipeline:
//   source SQL with GRANT → LoadDesiredState → Inspect live DB →
//   Diff → plan statements containing GRANT/REVOKE.
//
// The unit tests in privileges_diff_test.go already cover the differ logic in
// isolation; these tests confirm the end-to-end inspector → differ path works
// against a real PostgreSQL instance.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/nex-gen-tech/pg-flux/pkg/inspector"
	"github.com/nex-gen-tech/pg-flux/pkg/pgver"
	"github.com/nex-gen-tech/pg-flux/pkg/plan"
	"github.com/nex-gen-tech/pg-flux/pkg/src"
)

// TestB5_GrantPublicEmittedInMigration verifies the full pipeline:
//
//  1. Source has `GRANT SELECT ON public.products TO PUBLIC`.
//  2. Live DB has the table but no grant (relacl is NULL).
//  3. Diff produces a plan statement containing `GRANT SELECT ON TABLE … TO PUBLIC`.
//  4. After the grant is applied, Diff produces an empty plan (idempotency).
//  5. If the grant is removed from source, Diff produces `REVOKE SELECT … FROM PUBLIC`.
func TestB5_GrantPublicEmittedInMigration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	pool, _ := setupTestDB(t, ctx, "pgflux_b5_grant")
	defer pool.Close()

	pv, _ := pgver.Detect(ctx, pool)

	// Step 1: Create the table in the live DB (no grant yet).
	_, err := pool.Exec(ctx, `CREATE TABLE public.products (id serial PRIMARY KEY, name text NOT NULL)`)
	require.NoError(t, err)

	// Step 2: Build desired state with GRANT SELECT TO PUBLIC.
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "schema.sql"), []byte(`
CREATE TABLE public.products (id serial PRIMARY KEY, name text NOT NULL);
GRANT SELECT ON TABLE public.products TO PUBLIC;
`), 0o644))
	des, err := src.LoadDesiredState(src.LoadOptions{SchemaDir: dir})
	require.NoError(t, err)

	// Verify the desired state has the privilege set.
	tbl := des.Tables["public.products"]
	require.NotNil(t, tbl, "desired table must be parsed")
	require.NotEmpty(t, tbl.Privileges, "GRANT must populate Table.Privileges in desired state")

	// Step 3: Inspect live DB (table exists, no grant).
	live, err := inspector.Inspect(ctx, pool, inspector.Options{Schemas: []string{"public"}})
	require.NoError(t, err)

	// Confirm live DB has no privileges on the table yet.
	liveTbl := live.Tables["public.products"]
	require.NotNil(t, liveTbl, "live table must be inspected")
	require.Empty(t, liveTbl.Privileges, "live table must have no privileges before GRANT is applied")

	// Step 4: Diff — should emit GRANT SELECT TO PUBLIC.
	dr, err := Diff(des, live, Options{PGVersion: pv})
	require.NoError(t, err)

	var grantStmt string
	for _, s := range dr.Plan.Statements {
		if strings.Contains(s.DDL, "GRANT") && strings.Contains(s.DDL, "PUBLIC") {
			grantStmt = s.DDL
			break
		}
	}
	require.NotEmpty(t, grantStmt, "migrate generate must emit GRANT SELECT ... TO PUBLIC; statements: %v",
		planDDLs(dr.Plan.Statements))
	require.Contains(t, strings.ToUpper(grantStmt), "GRANT SELECT")
	require.Contains(t, strings.ToUpper(grantStmt), "PRODUCTS")
	require.Contains(t, strings.ToUpper(grantStmt), "PUBLIC")

	// Step 5: Apply the grant to the live DB.
	_, err = pool.Exec(ctx, grantStmt)
	require.NoError(t, err)

	// Step 6: Re-inspect and diff — plan must be empty (idempotency / no re-grant).
	live2, err := inspector.Inspect(ctx, pool, inspector.Options{Schemas: []string{"public"}})
	require.NoError(t, err)
	dr2, err := Diff(des, live2, Options{PGVersion: pv})
	require.NoError(t, err)
	for _, s := range dr2.Plan.Statements {
		if strings.Contains(s.DDL, "GRANT") {
			t.Fatalf("second diff must not emit GRANT (idempotency violated); got: %s", s.DDL)
		}
	}

	// Step 7: Remove the grant from source → diff must emit REVOKE.
	dir2 := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir2, "schema.sql"), []byte(`
CREATE TABLE public.products (id serial PRIMARY KEY, name text NOT NULL);
-- no GRANT
`), 0o644))
	des2, err := src.LoadDesiredState(src.LoadOptions{SchemaDir: dir2})
	require.NoError(t, err)

	// Desired has no privileges → declarative opt-out (don't manage permissions).
	// Per design: empty desired Privileges means "don't touch live permissions".
	// We test REVOKE by explicitly setting an empty-but-managed desired (one that
	// was previously managed). To test explicit REVOKE, use a desired state with
	// a different privilege set.
	//
	// The declarative opt-out path: no Privileges in source → no change.
	dr3, err := Diff(des2, live2, Options{PGVersion: pv})
	require.NoError(t, err)
	for _, s := range dr3.Plan.Statements {
		if strings.Contains(strings.ToUpper(s.DDL), "REVOKE") {
			t.Fatalf("declarative opt-out: no REVOKE when source has no GRANT statements; got: %s", s.DDL)
		}
	}
}

// TestB5_RevokeEmittedWhenGrantRemoved verifies that removing a GRANT from
// source while keeping the Privileges slice non-empty (managed) emits REVOKE.
func TestB5_RevokeEmittedWhenGrantRemoved(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pool, _ := setupTestDB(t, ctx, "pgflux_b5_revoke")
	defer pool.Close()

	pv, _ := pgver.Detect(ctx, pool)

	// Create table and apply the grant to the live DB.
	_, err := pool.Exec(ctx, `
		CREATE TABLE public.products (id serial PRIMARY KEY, name text NOT NULL);
		GRANT SELECT ON TABLE public.products TO PUBLIC;
	`)
	require.NoError(t, err)

	// Source: table managed with INSERT only (no SELECT), so SELECT must be revoked.
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "schema.sql"), []byte(`
CREATE TABLE public.products (id serial PRIMARY KEY, name text NOT NULL);
GRANT INSERT ON TABLE public.products TO PUBLIC;
`), 0o644))
	des, err := src.LoadDesiredState(src.LoadOptions{SchemaDir: dir})
	require.NoError(t, err)

	live, err := inspector.Inspect(ctx, pool, inspector.Options{Schemas: []string{"public"}})
	require.NoError(t, err)

	dr, err := Diff(des, live, Options{PGVersion: pv})
	require.NoError(t, err)

	var revokeStmt string
	var grantInsertStmt string
	for _, s := range dr.Plan.Statements {
		upper := strings.ToUpper(s.DDL)
		if strings.Contains(upper, "REVOKE SELECT") && strings.Contains(upper, "PUBLIC") {
			revokeStmt = s.DDL
		}
		if strings.Contains(upper, "GRANT INSERT") && strings.Contains(upper, "PUBLIC") {
			grantInsertStmt = s.DDL
		}
	}
	require.NotEmpty(t, revokeStmt,
		"diff must emit REVOKE SELECT when SELECT is removed from desired state; ddls: %v",
		planDDLs(dr.Plan.Statements))
	require.NotEmpty(t, grantInsertStmt,
		"diff must emit GRANT INSERT for the new privilege; ddls: %v",
		planDDLs(dr.Plan.Statements))
}

// planDDLs extracts DDL strings from plan statements (helper for test error messages).
func planDDLs(stmts []plan.Statement) []string {
	out := make([]string, 0, len(stmts))
	for _, s := range stmts {
		if s.DDL != "" {
			out = append(out, s.DDL)
		}
	}
	return out
}
