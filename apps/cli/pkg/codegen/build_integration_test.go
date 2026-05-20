//go:build integration

package codegen

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/nexg/pg-flux/pkg/inspector"
)

func adminDSN() string {
	if v := os.Getenv("PGFLUX_TEST_DSN"); v != "" {
		return v
	}
	return "postgres://pgflux:pgflux@localhost:5440/pgflux?sslmode=disable"
}

// TestGoOutputCompiles is the correctness gate for the Go emitter: take a
// non-trivial fixture, apply it to a fresh DB, inspect, generate Go code into
// a temp module, and run `go build` against the output. If anything fails
// to compile, the emitter is wrong.
func TestGoOutputCompiles(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	admin, err := pgxpool.New(ctx, adminDSN())
	require.NoError(t, err)
	defer admin.Close()
	dbname := "pgflux_codegen_build"
	_, _ = admin.Exec(ctx, "DROP DATABASE IF EXISTS "+dbname)
	_, err = admin.Exec(ctx, "CREATE DATABASE "+dbname)
	require.NoError(t, err)
	defer func() { _, _ = admin.Exec(context.Background(), "DROP DATABASE IF EXISTS "+dbname) }()

	cfg, _ := pgxpool.ParseConfig(adminDSN())
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.ConnConfig.User, cfg.ConnConfig.Password,
		cfg.ConnConfig.Host, cfg.ConnConfig.Port, dbname)
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	defer pool.Close()

	// Apply a fixture exercising the harder emitter branches: nullable
	// columns, arrays, enums, composite types, domains, comments.
	_, err = pool.Exec(ctx, `
		CREATE TYPE public.user_role AS ENUM ('admin','member','guest');
		CREATE DOMAIN public.short_text AS text;
		CREATE TYPE public.addr AS (street text, zip varchar(10));
		CREATE TABLE public.users (
			id bigint PRIMARY KEY,
			email text NOT NULL,
			display_name text,
			role public.user_role NOT NULL,
			created_at timestamptz NOT NULL DEFAULT now(),
			metadata jsonb,
			tags text[]
		);
		COMMENT ON COLUMN public.users.email IS 'Unique login email.';
	`)
	require.NoError(t, err)

	state, err := inspector.Inspect(ctx, pool, inspector.Options{Schemas: []string{"public"}})
	require.NoError(t, err)

	// Generate into a fresh Go module so `go build` can resolve imports.
	dir := t.TempDir()
	modPath := "pgflux/codegen_test"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"),
		[]byte("module "+modPath+"\n\ngo 1.21\n"), 0o644))

	g := NewGoGenerator()
	fs, err := g.Generate(state, Options{Package: "dbgen"})
	require.NoError(t, err)
	for path, content := range fs {
		full := filepath.Join(dir, "dbgen", path)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, content, 0o644))
	}

	// `go build` the generated package.
	cmd := exec.CommandContext(ctx, "go", "build", "./dbgen/...")
	cmd.Dir = dir
	out, berr := cmd.CombinedOutput()
	if berr != nil {
		// Dump the generated files so the failure is debuggable.
		for path, content := range fs {
			t.Logf("=== %s ===\n%s\n", path, string(content))
		}
		t.Fatalf("go build failed: %v\n%s", berr, string(out))
	}
}

// TestTSOutputCompiles uses tsc --noEmit when tsc is available. Skipped silently
// when tsc isn't installed (most CI environments don't have it). When it is
// available (dev machines), it catches TS-level type errors in the emitter.
func TestTSOutputCompiles(t *testing.T) {
	if _, err := exec.LookPath("tsc"); err != nil {
		t.Skip("tsc not on PATH; skipping TS compile gate")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	admin, err := pgxpool.New(ctx, adminDSN())
	require.NoError(t, err)
	defer admin.Close()
	dbname := "pgflux_codegen_ts_build"
	_, _ = admin.Exec(ctx, "DROP DATABASE IF EXISTS "+dbname)
	_, err = admin.Exec(ctx, "CREATE DATABASE "+dbname)
	require.NoError(t, err)
	defer func() { _, _ = admin.Exec(context.Background(), "DROP DATABASE IF EXISTS "+dbname) }()
	cfg, _ := pgxpool.ParseConfig(adminDSN())
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.ConnConfig.User, cfg.ConnConfig.Password,
		cfg.ConnConfig.Host, cfg.ConnConfig.Port, dbname)
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	defer pool.Close()
	_, err = pool.Exec(ctx, `CREATE TABLE public.t (id bigint, name text);`)
	require.NoError(t, err)
	state, err := inspector.Inspect(ctx, pool, inspector.Options{Schemas: []string{"public"}})
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte(`{
		"compilerOptions": {
			"strict": true,
			"noEmit": true,
			"target": "ES2020",
			"module": "ESNext",
			"moduleResolution": "node"
		},
		"include": ["**/*.ts"]
	}`), 0o644))
	g := NewTSGenerator()
	fs, err := g.Generate(state, Options{})
	require.NoError(t, err)
	for path, content := range fs {
		require.NoError(t, os.WriteFile(filepath.Join(dir, path), content, 0o644))
	}
	cmd := exec.CommandContext(ctx, "tsc", "--noEmit", "-p", dir)
	out, terr := cmd.CombinedOutput()
	if terr != nil {
		for path, content := range fs {
			t.Logf("=== %s ===\n%s\n", path, string(content))
		}
		t.Fatalf("tsc failed: %v\n%s", terr, string(out))
	}
}
