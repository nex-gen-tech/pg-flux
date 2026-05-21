//go:build integration

package differ

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
	"github.com/nex-gen-tech/pg-flux/pkg/src"
)

// TestProcedureNoReemit verifies that a stored procedure is not re-emitted in
// every subsequent migrate generate once it already exists in the live DB (Bug B2).
// Root cause: pg_get_functiondef emits "IN param_name type" but source SQL omits the
// "IN" mode keyword; fpFunctionSQL must normalise both forms to the same fingerprint.
func TestProcedureNoReemit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	pool, _ := setupTestDB(t, ctx, "pgflux_proc_noreemit")
	defer pool.Close()

	// Apply the procedure directly to the live DB (simulating a first migrate apply).
	_, err := pool.Exec(ctx, `
CREATE OR REPLACE PROCEDURE public.notify_order_placed(order_id integer)
LANGUAGE plpgsql
AS $$
BEGIN
  PERFORM pg_notify('order_placed', order_id::text);
END;
$$;
`)
	require.NoError(t, err)

	// Load the same source — the procedure is declared here, already in live DB.
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "schema.sql"), []byte(`
CREATE OR REPLACE PROCEDURE public.notify_order_placed(order_id integer)
LANGUAGE plpgsql
AS $$
BEGIN
  PERFORM pg_notify('order_placed', order_id::text);
END;
$$;
`), 0o644))

	des, err := src.LoadDesiredState(src.LoadOptions{SchemaDir: dir})
	require.NoError(t, err)

	live, err := inspector.Inspect(ctx, pool, inspector.Options{Schemas: []string{"public"}})
	require.NoError(t, err)

	pv, _ := pgver.Detect(ctx, pool)
	dr, err := Diff(des, live, Options{PGVersion: pv})
	require.NoError(t, err)

	// No actionable DDL should be emitted — the procedure already exists and is unchanged.
	var actionable []string
	for _, s := range dr.Plan.Statements {
		if strings.TrimSpace(s.DDL) != "" {
			actionable = append(actionable, s.DDL)
		}
	}
	require.Empty(t, actionable,
		"procedure re-emitted on second generate — Bug B2 not fixed; got: %v", actionable)
}

// TestCreateSchemaNoReemit verifies that CREATE SCHEMA IF NOT EXISTS is not re-emitted
// in every migrate generate once the schema already exists in the live DB (Bug B3).
// Root cause: diffExtraDDL had no deduplication for schema objects against live catalog.
func TestCreateSchemaNoReemit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	pool, _ := setupTestDB(t, ctx, "pgflux_schema_noreemit")
	defer pool.Close()

	// Create the schema directly in the live DB (simulating a first migrate apply).
	_, err := pool.Exec(ctx, `CREATE SCHEMA IF NOT EXISTS audit;`)
	require.NoError(t, err)

	// Also create a table that references the schema so the desired state is non-empty.
	_, err = pool.Exec(ctx, `CREATE TABLE public.items (id integer PRIMARY KEY);`)
	require.NoError(t, err)

	// Source declares the schema and a table — the schema already exists.
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "schema.sql"), []byte(`
CREATE SCHEMA IF NOT EXISTS audit;
CREATE TABLE public.items (id integer PRIMARY KEY);
`), 0o644))

	des, err := src.LoadDesiredState(src.LoadOptions{SchemaDir: dir})
	require.NoError(t, err)

	live, err := inspector.Inspect(ctx, pool, inspector.Options{Schemas: []string{"public", "audit"}})
	require.NoError(t, err)

	pv, _ := pgver.Detect(ctx, pool)
	dr, err := Diff(des, live, Options{PGVersion: pv})
	require.NoError(t, err)

	// No actionable DDL should be emitted — nothing has changed.
	var actionable []string
	for _, s := range dr.Plan.Statements {
		if strings.TrimSpace(s.DDL) != "" {
			actionable = append(actionable, s.DDL)
		}
	}
	require.Empty(t, actionable,
		"CREATE SCHEMA re-emitted on second generate — Bug B3 not fixed; got: %v", actionable)
}
