//go:build integration

package differ

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/nex-gen-tech/pg-flux/pkg/inspector"
	"github.com/nex-gen-tech/pg-flux/pkg/pgver"
	"github.com/nex-gen-tech/pg-flux/pkg/src"
)

// adminDSN returns the test container DSN.
func adminDSN() string {
	if v := os.Getenv("PGFLUX_TEST_DSN"); v != "" {
		return v
	}
	return "postgres://pgflux:pgflux@localhost:5440/pgflux?sslmode=disable"
}

func setupTestDB(t *testing.T, ctx context.Context, dbname string) (*pgxpool.Pool, string) {
	t.Helper()
	pool, err := pgxpool.New(ctx, adminDSN())
	require.NoError(t, err)
	defer pool.Close()
	_, _ = pool.Exec(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %s", dbname))
	_, err = pool.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s", dbname))
	require.NoError(t, err)
	cfg, err := pgxpool.ParseConfig(adminDSN())
	require.NoError(t, err)
	cfg.ConnConfig.Database = dbname
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.ConnConfig.User, cfg.ConnConfig.Password,
		cfg.ConnConfig.Host, cfg.ConnConfig.Port, dbname)
	tdb, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	return tdb, dsn
}

// TestIndexFingerprint_includeColumns: an INCLUDE index round-trips through
// CREATE → inspect → no-drift on identical desired, and emits DROP+CREATE when
// the INCLUDE list changes.
func TestIndexFingerprint_includeColumns(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool, _ := setupTestDB(t, ctx, "pgflux_idx_include")
	defer pool.Close()

	// Apply baseline directly.
	_, err := pool.Exec(ctx, `
		CREATE TABLE public.users (id bigserial PRIMARY KEY, email text, name text);
		CREATE INDEX users_email_inc ON public.users (email) INCLUDE (name);
	`)
	require.NoError(t, err)

	// Round-trip: same source should produce no diff.
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "s.sql"), []byte(`
CREATE TABLE public.users (id bigserial PRIMARY KEY, email text, name text);
CREATE INDEX users_email_inc ON public.users (email) INCLUDE (name);
`), 0o644))
	des, err := src.LoadDesiredState(src.LoadOptions{SchemaDir: dir})
	require.NoError(t, err)
	live, err := inspector.Inspect(ctx, pool, inspector.Options{Schemas: []string{"public"}})
	require.NoError(t, err)
	pv, _ := pgver.Detect(ctx, pool)
	dr, err := Diff(des, live, Options{PGVersion: pv})
	require.NoError(t, err)
	for _, s := range dr.Plan.Statements {
		if strings.TrimSpace(s.DDL) != "" {
			t.Fatalf("unexpected diff for identical INCLUDE index: %s", s.DDL)
		}
	}

	// Now change INCLUDE list: drop name from INCLUDE.
	dir2 := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir2, "s.sql"), []byte(`
CREATE TABLE public.users (id bigserial PRIMARY KEY, email text, name text);
CREATE INDEX users_email_inc ON public.users (email);
`), 0o644))
	des2, err := src.LoadDesiredState(src.LoadOptions{SchemaDir: dir2})
	require.NoError(t, err)
	dr2, err := Diff(des2, live, Options{PGVersion: pv})
	require.NoError(t, err)
	gotDropAndCreate := false
	for _, s := range dr2.Plan.Statements {
		if strings.Contains(s.DDL, "DROP INDEX") && strings.Contains(s.DDL, "users_email_inc") {
			gotDropAndCreate = true
		}
	}
	if !gotDropAndCreate {
		var got []string
		for _, s := range dr2.Plan.Statements {
			got = append(got, s.DDL)
		}
		t.Fatalf("expected DROP INDEX users_email_inc when INCLUDE list changes; got:\n%s", strings.Join(got, "\n"))
	}
}

// TestIndexFingerprint_nullsNotDistinct: NULLS NOT DISTINCT (PG15+) round-trips
// and a change to the flag emits DROP+CREATE.
func TestIndexFingerprint_nullsNotDistinct(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool, _ := setupTestDB(t, ctx, "pgflux_idx_nulls")
	defer pool.Close()

	pv, _ := pgver.Detect(ctx, pool)
	if !pv.Supports(pgver.FeatureNullsNotDistinct) {
		t.Skipf("PG %s does not support NULLS NOT DISTINCT", pv.String())
	}

	_, err := pool.Exec(ctx, `
		CREATE TABLE public.t (id int, code text);
		CREATE UNIQUE INDEX t_code_uniq ON public.t (code) NULLS NOT DISTINCT;
	`)
	require.NoError(t, err)

	// Round-trip.
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "s.sql"), []byte(`
CREATE TABLE public.t (id int, code text);
CREATE UNIQUE INDEX t_code_uniq ON public.t (code) NULLS NOT DISTINCT;
`), 0o644))
	des, err := src.LoadDesiredState(src.LoadOptions{SchemaDir: dir})
	require.NoError(t, err)
	live, err := inspector.Inspect(ctx, pool, inspector.Options{Schemas: []string{"public"}})
	require.NoError(t, err)
	dr, err := Diff(des, live, Options{PGVersion: pv})
	require.NoError(t, err)
	for _, s := range dr.Plan.Statements {
		if strings.TrimSpace(s.DDL) != "" {
			t.Fatalf("unexpected diff for identical NULLS NOT DISTINCT index: %s", s.DDL)
		}
	}

	// Remove NULLS NOT DISTINCT (default behavior).
	dir2 := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir2, "s.sql"), []byte(`
CREATE TABLE public.t (id int, code text);
CREATE UNIQUE INDEX t_code_uniq ON public.t (code);
`), 0o644))
	des2, err := src.LoadDesiredState(src.LoadOptions{SchemaDir: dir2})
	require.NoError(t, err)
	dr2, err := Diff(des2, live, Options{PGVersion: pv})
	require.NoError(t, err)
	gotChange := false
	for _, s := range dr2.Plan.Statements {
		if strings.Contains(s.DDL, "DROP INDEX") && strings.Contains(s.DDL, "t_code_uniq") {
			gotChange = true
		}
	}
	if !gotChange {
		var got []string
		for _, s := range dr2.Plan.Statements {
			got = append(got, s.DDL)
		}
		t.Fatalf("expected DROP INDEX when NULLS NOT DISTINCT removed; got:\n%s", strings.Join(got, "\n"))
	}
}

// TestIndexFingerprint_descNullsFirst: a DESC NULLS FIRST column order must round-trip.
func TestIndexFingerprint_descNullsFirst(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool, _ := setupTestDB(t, ctx, "pgflux_idx_desc")
	defer pool.Close()

	_, err := pool.Exec(ctx, `
		CREATE TABLE public.posts (id int, score int);
		CREATE INDEX posts_score_desc ON public.posts (score DESC NULLS FIRST);
	`)
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "s.sql"), []byte(`
CREATE TABLE public.posts (id int, score int);
CREATE INDEX posts_score_desc ON public.posts (score DESC NULLS FIRST);
`), 0o644))
	des, err := src.LoadDesiredState(src.LoadOptions{SchemaDir: dir})
	require.NoError(t, err)
	live, err := inspector.Inspect(ctx, pool, inspector.Options{Schemas: []string{"public"}})
	require.NoError(t, err)
	pv, _ := pgver.Detect(ctx, pool)
	dr, err := Diff(des, live, Options{PGVersion: pv})
	require.NoError(t, err)
	for _, s := range dr.Plan.Statements {
		if strings.TrimSpace(s.DDL) != "" {
			t.Fatalf("unexpected diff for identical DESC NULLS FIRST index: %s", s.DDL)
		}
	}
}

// TestIndexFingerprint_opclass: a non-default opclass (text_pattern_ops) must round-trip.
func TestIndexFingerprint_opclass(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool, _ := setupTestDB(t, ctx, "pgflux_idx_opclass")
	defer pool.Close()

	_, err := pool.Exec(ctx, `
		CREATE TABLE public.docs (id int, body text);
		CREATE INDEX docs_body_pattern ON public.docs (body text_pattern_ops);
	`)
	require.NoError(t, err)

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "s.sql"), []byte(`
CREATE TABLE public.docs (id int, body text);
CREATE INDEX docs_body_pattern ON public.docs (body text_pattern_ops);
`), 0o644))
	des, err := src.LoadDesiredState(src.LoadOptions{SchemaDir: dir})
	require.NoError(t, err)
	live, err := inspector.Inspect(ctx, pool, inspector.Options{Schemas: []string{"public"}})
	require.NoError(t, err)
	pv, _ := pgver.Detect(ctx, pool)
	dr, err := Diff(des, live, Options{PGVersion: pv})
	require.NoError(t, err)
	for _, s := range dr.Plan.Statements {
		if strings.TrimSpace(s.DDL) != "" {
			t.Fatalf("unexpected diff for identical opclass index: %s", s.DDL)
		}
	}
}
