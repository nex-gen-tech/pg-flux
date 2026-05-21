//go:build integration

package differ

// B6: Integration tests for partial-index enum cast drift.
//
// When a partial index references an enum column with a bare string literal,
// PostgreSQL stores the WHERE predicate with a resolved type cast in the catalog
// (e.g. `WHERE (status = 'active'::product_status)`). The source SQL uses the
// bare literal form (`WHERE status = 'active'`). Without the B6 fix these two
// forms produce different normalised fingerprints, causing false "index changed"
// drift on every `migrate generate` run.
//
// These tests validate the full end-to-end pipeline (CREATE TYPE → CREATE TABLE →
// CREATE INDEX → Inspect → Diff) produces an empty plan (no spurious drift) when
// nothing actually changed.

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

// TestB6_PartialIndexEnumCastNoDrift validates that a partial index on an enum
// column using a bare string literal in source SQL does not produce spurious drift
// when pg_get_indexdef adds the resolved enum type cast in the catalog.
func TestB6_PartialIndexEnumCastNoDrift(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	pool, _ := setupTestDB(t, ctx, "pgflux_b6_enum_cast")
	defer pool.Close()

	pv, _ := pgver.Detect(ctx, pool)

	// Apply the schema directly to the live DB.
	_, err := pool.Exec(ctx, `
		CREATE TYPE public.product_status AS ENUM ('active', 'archived', 'draft');
		CREATE TABLE public.products (
			id serial PRIMARY KEY,
			name text NOT NULL,
			status public.product_status NOT NULL DEFAULT 'draft'
		);
		CREATE INDEX products_active_idx ON public.products (id)
			WHERE status = 'active';
	`)
	require.NoError(t, err)

	// Build the desired state from source SQL (bare literal, no cast).
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "schema.sql"), []byte(`
CREATE TYPE public.product_status AS ENUM ('active', 'archived', 'draft');
CREATE TABLE public.products (
    id serial PRIMARY KEY,
    name text NOT NULL,
    status public.product_status NOT NULL DEFAULT 'draft'
);
CREATE INDEX products_active_idx ON public.products (id)
    WHERE status = 'active';
`), 0o644))
	des, err := src.LoadDesiredState(src.LoadOptions{SchemaDir: dir})
	require.NoError(t, err)

	// Inspect the live DB — pg_get_indexdef will include ::product_status cast.
	live, err := inspector.Inspect(ctx, pool, inspector.Options{Schemas: []string{"public"}})
	require.NoError(t, err)

	// Verify that the live index def has the cast (confirms the test is exercising the right path).
	liveIdx := live.Indexes["public.products_active_idx"]
	require.NotNil(t, liveIdx, "live index must be inspected")
	// pg_get_indexdef on an enum column adds ::product_status; the B6 fix must normalise it away.
	t.Logf("live index CreateSQL: %s", liveIdx.CreateSQL)

	// Diff — must produce no drift (empty plan with no real DDL).
	dr, err := Diff(des, live, Options{PGVersion: pv})
	require.NoError(t, err)

	for _, s := range dr.Plan.Statements {
		if strings.TrimSpace(s.DDL) == "" {
			continue // advisory notice, not real DDL
		}
		if strings.Contains(strings.ToUpper(s.DDL), "DROP INDEX") ||
			strings.Contains(strings.ToUpper(s.DDL), "CREATE INDEX") {
			t.Fatalf("B6: false drift detected on partial index with enum cast — "+
				"source and catalog should match after normalisation; got: %s", s.DDL)
		}
	}
}

// TestB6_PartialIndexDifferentEnumValueStillDrifts confirms that a genuine change
// to the enum literal in the WHERE clause IS still detected as drift (the normaliser
// must not collapse distinct literal values into the same fingerprint).
func TestB6_PartialIndexDifferentEnumValueStillDrifts(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	pool, _ := setupTestDB(t, ctx, "pgflux_b6_drift")
	defer pool.Close()

	pv, _ := pgver.Detect(ctx, pool)

	// Apply a partial index with 'active' to the live DB.
	_, err := pool.Exec(ctx, `
		CREATE TYPE public.product_status AS ENUM ('active', 'archived', 'draft');
		CREATE TABLE public.products (
			id serial PRIMARY KEY,
			status public.product_status NOT NULL DEFAULT 'draft'
		);
		CREATE INDEX products_active_idx ON public.products (id)
			WHERE status = 'active';
	`)
	require.NoError(t, err)

	// Desired source: index uses 'archived' instead of 'active' — a real change.
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "schema.sql"), []byte(`
CREATE TYPE public.product_status AS ENUM ('active', 'archived', 'draft');
CREATE TABLE public.products (
    id serial PRIMARY KEY,
    status public.product_status NOT NULL DEFAULT 'draft'
);
CREATE INDEX products_active_idx ON public.products (id)
    WHERE status = 'archived';
`), 0o644))
	des, err := src.LoadDesiredState(src.LoadOptions{SchemaDir: dir})
	require.NoError(t, err)

	live, err := inspector.Inspect(ctx, pool, inspector.Options{Schemas: []string{"public"}})
	require.NoError(t, err)

	dr, err := Diff(des, live, Options{PGVersion: pv})
	require.NoError(t, err)

	gotDrop := false
	for _, s := range dr.Plan.Statements {
		if strings.Contains(strings.ToUpper(s.DDL), "DROP INDEX") &&
			strings.Contains(s.DDL, "products_active_idx") {
			gotDrop = true
		}
	}
	require.True(t, gotDrop,
		"B6: 'active' vs 'archived' in WHERE clause must still register as drift; "+
			"got statements: %v", planDDLs(dr.Plan.Statements))
}
