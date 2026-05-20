//go:build integration

package migrate

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/nexg/pg-flux/pkg/src"
)

// dsnFromEnv returns the integration-test DSN.
func dsnFromEnv() string {
	if v := os.Getenv("PGFLUX_TEST_DSN"); v != "" {
		return v
	}
	return "postgres://pgflux:pgflux@localhost:5440/pgflux?sslmode=disable"
}

// resetDB recreates a clean test database for this test to avoid cross-test pollution.
func resetDB(t *testing.T, ctx context.Context, adminDSN, dbname string) string {
	t.Helper()
	pool, err := pgxpool.New(ctx, adminDSN)
	require.NoError(t, err)
	defer pool.Close()
	_, _ = pool.Exec(ctx, "DROP DATABASE IF EXISTS "+dbname)
	_, err = pool.Exec(ctx, "CREATE DATABASE "+dbname)
	require.NoError(t, err)
	// Build the test-specific DSN by substituting the database name.
	cfg, err := pgxpool.ParseConfig(adminDSN)
	require.NoError(t, err)
	cfg.ConnConfig.Database = dbname
	return "postgres://" + cfg.ConnConfig.User + ":" + cfg.ConnConfig.Password +
		"@" + cfg.ConnConfig.Host + ":" + itoa(int(cfg.ConnConfig.Port)) +
		"/" + dbname + "?sslmode=disable"
}

func itoa(i int) string {
	const digits = "0123456789"
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{digits[i%10]}, b...)
		i /= 10
	}
	return string(b)
}

// TestApplyRefusesOnDrift: generate a migration, mutate the live DB outside
// pg-flux, then apply. Without --force-after-drift, apply must refuse.
func TestApplyRefusesOnDrift(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	dsn := resetDB(t, ctx, dsnFromEnv(), "pgflux_drift_test")
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "schema.sql"), []byte(`
CREATE TABLE public.items (id integer PRIMARY KEY, n text NOT NULL);
`), 0o644))
	des, err := src.LoadDesiredState(src.LoadOptions{SchemaDir: dir})
	require.NoError(t, err)

	migDir := t.TempDir()
	gen, err := Generate(ctx, pool, des, GenerateOptions{
		MigrationsDir: migDir,
		Label:         "create_items",
		Schemas:       []string{"public"},
	})
	require.NoError(t, err)
	require.NotEmpty(t, gen.Filename, "expected a migration file")

	// Mutate the live DB outside pg-flux so the baseline hash no longer matches.
	_, err = pool.Exec(ctx, "CREATE TABLE public.outsider (k int)")
	require.NoError(t, err)

	// Default apply must refuse.
	_, err = Apply(ctx, pool, ApplyOptions{
		MigrationsDir:  migDir,
		TrackingSchema: "_pgflux",
		Schemas:        []string{"public"},
	})
	require.Error(t, err)
	var bde *BaselineDriftError
	require.True(t, errors.As(err, &bde), "expected *BaselineDriftError, got %T (%v)", err, err)

	// With --force-after-drift, apply proceeds.
	_, err = Apply(ctx, pool, ApplyOptions{
		MigrationsDir:   migDir,
		TrackingSchema:  "_pgflux",
		Schemas:         []string{"public"},
		ForceAfterDrift: true,
	})
	require.NoError(t, err)

	// Cleanup the test database.
	_, _ = pool.Exec(ctx, "DROP TABLE IF EXISTS public.items, public.outsider CASCADE")
}

// TestApplyAcceptsWhenLiveMatchesBaseline: happy path — no drift, apply succeeds.
func TestApplyAcceptsWhenLiveMatchesBaseline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	dsn := resetDB(t, ctx, dsnFromEnv(), "pgflux_drift_test_ok")
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "schema.sql"), []byte(`
CREATE TABLE public.things (id integer PRIMARY KEY);
`), 0o644))
	des, err := src.LoadDesiredState(src.LoadOptions{SchemaDir: dir})
	require.NoError(t, err)

	migDir := t.TempDir()
	_, err = Generate(ctx, pool, des, GenerateOptions{
		MigrationsDir: migDir,
		Label:         "create_things",
		Schemas:       []string{"public"},
	})
	require.NoError(t, err)

	_, err = Apply(ctx, pool, ApplyOptions{
		MigrationsDir:  migDir,
		TrackingSchema: "_pgflux",
		Schemas:        []string{"public"},
	})
	require.NoError(t, err, "no-drift apply should succeed")
}
