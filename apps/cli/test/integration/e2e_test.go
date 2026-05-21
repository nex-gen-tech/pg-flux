//go:build integration

package integration

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/nex-gen-tech/pg-flux/pkg/db"
	"github.com/nex-gen-tech/pg-flux/pkg/differ"
	"github.com/nex-gen-tech/pg-flux/pkg/exec"
	"github.com/nex-gen-tech/pg-flux/pkg/inspector"
	"github.com/nex-gen-tech/pg-flux/pkg/shadow"
	"github.com/nex-gen-tech/pg-flux/pkg/src"
)

func waitForPostgres(ctx context.Context, po *pgxpool.Pool) error {
	var last error
	for i := 0; i < 60; i++ {
		if err := po.Ping(ctx); err == nil {
			return nil
		} else {
			last = err
		}
		select {
		case <-ctx.Done():
			return last
		case <-time.After(500 * time.Millisecond):
		}
	}
	return last
}

// TestE2E_PlanApplyAndDrift uses a real Postgres 18 container (requires Docker + PGFLUX_E2E=1).
func TestE2E_PlanApplyAndDrift(t *testing.T) {
	if os.Getenv("PGFLUX_E2E") == "" {
		t.Skip("set PGFLUX_E2E=1 and ensure Docker is running")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	c, err := postgres.Run(ctx, "postgres:18-alpine",
		postgres.WithDatabase("appdb"),
		postgres.WithUsername("app"),
		postgres.WithPassword("app"),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Terminate(context.Background()) })

	conn, err := c.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	t.Setenv("DATABASE_URL", conn)

	po, err := db.NewPool(ctx, conn)
	require.NoError(t, err)
	t.Cleanup(func() { po.Close() })
	require.NoError(t, waitForPostgres(ctx, po), "postgres should accept connections before Inspect")

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "01_tables.sql"), []byte(`
CREATE TABLE items (
    id integer PRIMARY KEY,
    n text NOT NULL
);
CREATE INDEX idx_items_n ON public.items USING btree (n);
CREATE OR REPLACE FUNCTION public.double_it(x int) RETURNS int
    LANGUAGE sql
    IMMUTABLE
AS $$ SELECT x * 2 $$;
`), 0o644))

	des, err := src.LoadDesiredState(src.LoadOptions{SchemaDir: dir})
	require.NoError(t, err)
	live, err := inspector.Inspect(ctx, po, inspector.Options{Schemas: []string{"public"}})
	require.NoError(t, err)
	dr, err := differ.Diff(des, live, differ.Options{})
	require.NoError(t, err)
	require.NotEmpty(t, dr.Plan.Statements, "expected plan to create objects")

	err = exec.Apply(ctx, po, dr.Plan, exec.Options{})
	require.NoError(t, err)

	live2, err := inspector.Inspect(ctx, po, inspector.Options{Schemas: []string{"public"}})
	require.NoError(t, err)
	dr2, err := differ.Diff(des, live2, differ.Options{})
	require.NoError(t, err)
	require.Empty(t, dr2.Plan.Statements, "no drift after apply: desired and live should match (see pg_query fingerprint + catalog normalization)")
}

// TestE2E_RepoTestdataDir loads testdata/integration-smoke from the module root (how users use --schema).
func TestE2E_RepoTestdataDir(t *testing.T) {
	if os.Getenv("PGFLUX_E2E") == "" {
		t.Skip("set PGFLUX_E2E=1 and ensure Docker is running")
	}
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	// test/integration -> repo root
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
	schemaDir := filepath.Join(repoRoot, "testdata", "integration-smoke")
	_, err := os.Stat(schemaDir)
	require.NoError(t, err, "testdata/integration-smoke should exist in repo")

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	c, err := postgres.Run(ctx, "postgres:18-alpine",
		postgres.WithDatabase("appdb"),
		postgres.WithUsername("app"),
		postgres.WithPassword("app"),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Terminate(context.Background()) })
	conn, err := c.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	po, err := db.NewPool(ctx, conn)
	require.NoError(t, err)
	t.Cleanup(func() { po.Close() })
	require.NoError(t, waitForPostgres(ctx, po))

	des, err := src.LoadDesiredState(src.LoadOptions{SchemaDir: schemaDir})
	require.NoError(t, err)
	live, err := inspector.Inspect(ctx, po, inspector.Options{Schemas: []string{"public"}})
	require.NoError(t, err)
	dr, err := differ.Diff(des, live, differ.Options{})
	require.NoError(t, err)
	require.NotEmpty(t, dr.Plan.Statements)
	err = exec.Apply(ctx, po, dr.Plan, exec.Options{})
	require.NoError(t, err)
	live2, err := inspector.Inspect(ctx, po, inspector.Options{Schemas: []string{"public"}})
	require.NoError(t, err)
	dr2, err := differ.Diff(des, live2, differ.Options{})
	require.NoError(t, err)
	require.Empty(t, dr2.Plan.Statements, "no drift with schema loaded from testdata/integration-smoke")
}

// TestE2E_FullObjectGraph applies a_parents/b_children, FK, CHECK, index, functions, view, trigger (see testdata/integration-full).
func TestE2E_FullObjectGraph(t *testing.T) {
	if os.Getenv("PGFLUX_E2E") == "" {
		t.Skip("set PGFLUX_E2E=1 and ensure Docker is running")
	}
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
	schemaDir := filepath.Join(repoRoot, "testdata", "integration-full")
	_, err := os.Stat(schemaDir)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	c, err := postgres.Run(ctx, "postgres:18-alpine",
		postgres.WithDatabase("appdb"),
		postgres.WithUsername("app"),
		postgres.WithPassword("app"),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Terminate(context.Background()) })
	conn, err := c.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	po, err := db.NewPool(ctx, conn)
	require.NoError(t, err)
	t.Cleanup(func() { po.Close() })
	require.NoError(t, waitForPostgres(ctx, po))

	des, err := src.LoadDesiredState(src.LoadOptions{SchemaDir: schemaDir})
	require.NoError(t, err)
	live, err := inspector.Inspect(ctx, po, inspector.Options{Schemas: []string{"public"}})
	require.NoError(t, err)
	dr, err := differ.Diff(des, live, differ.Options{})
	require.NoError(t, err)
	t.Logf("full graph plan: %d statements", len(dr.Plan.Statements))
	require.NotEmpty(t, dr.Plan.Statements)
	err = exec.Apply(ctx, po, dr.Plan, exec.Options{})
	require.NoError(t, err)
	live2, err := inspector.Inspect(ctx, po, inspector.Options{Schemas: []string{"public"}})
	require.NoError(t, err)
	dr2, err := differ.Diff(des, live2, differ.Options{})
	require.NoError(t, err)
	if len(dr2.Plan.Statements) > 0 {
		for _, s := range dr2.Plan.Statements {
			t.Logf("drift: %s %q", s.OpType, s.DDL)
		}
	}
	require.Empty(t, dr2.Plan.Statements, "zero drift after full object graph apply")
}

// TestE2E_ShadowEquivalence runs ValidateStructuralEquivalence on a fresh empty DB in Docker —
// structural "apply + inspect + diff" check (not a formal proof against production data).
func TestE2E_ShadowEquivalence(t *testing.T) {
	if os.Getenv("PGFLUX_E2E") == "" {
		t.Skip("set PGFLUX_E2E=1 and ensure Docker is running")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	c, err := postgres.Run(ctx, "postgres:18-alpine",
		postgres.WithDatabase("appdb"),
		postgres.WithUsername("app"),
		postgres.WithPassword("app"),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Terminate(context.Background()) })
	conn, err := c.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	po, err := db.NewPool(ctx, conn)
	require.NoError(t, err)
	t.Cleanup(func() { po.Close() })
	require.NoError(t, waitForPostgres(ctx, po))
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.sql"), []byte(`
CREATE TABLE public.equiv_x (id int primary key, n text not null);
CREATE INDEX idx_equiv_x_n ON public.equiv_x (n);
`), 0o644))
	des, err := src.LoadDesiredState(src.LoadOptions{SchemaDir: dir})
	require.NoError(t, err)
	live, err := inspector.Inspect(ctx, po, inspector.Options{Schemas: []string{"public"}})
	require.NoError(t, err)
	dr, err := differ.Diff(des, live, differ.Options{})
	require.NoError(t, err)
	require.NotEmpty(t, dr.Plan.Statements)
	err = shadow.ValidateStructuralEquivalence(ctx, conn, des, dr.Plan, differ.Options{})
	require.NoError(t, err)
}
