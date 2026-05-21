//go:build integration

package dump

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

	"github.com/nex-gen-tech/pg-flux/pkg/differ"
	"github.com/nex-gen-tech/pg-flux/pkg/inspector"
	"github.com/nex-gen-tech/pg-flux/pkg/pgver"
	"github.com/nex-gen-tech/pg-flux/pkg/src"
)

func adminDSN() string {
	if v := os.Getenv("PGFLUX_TEST_DSN"); v != "" {
		return v
	}
	return "postgres://pgflux:pgflux@localhost:5440/pgflux?sslmode=disable"
}

// freshDB creates an empty test database and returns a pool connected to it.
// The caller is responsible for closing the pool. The database is dropped on
// test cleanup.
func freshDB(t *testing.T, ctx context.Context, name string) *pgxpool.Pool {
	t.Helper()
	admin, err := pgxpool.New(ctx, adminDSN())
	require.NoError(t, err)
	defer admin.Close()
	_, _ = admin.Exec(ctx, "DROP DATABASE IF EXISTS "+name)
	_, err = admin.Exec(ctx, "CREATE DATABASE "+name)
	require.NoError(t, err)
	_, err = admin.Exec(ctx, fmt.Sprintf(
		"GRANT ALL ON DATABASE %s TO pgflux", name))
	require.NoError(t, err)
	cfg, err := pgxpool.ParseConfig(adminDSN())
	require.NoError(t, err)
	cfg.ConnConfig.Database = name
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.ConnConfig.User, cfg.ConnConfig.Password,
		cfg.ConnConfig.Host, cfg.ConnConfig.Port, name)
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(func() {
		pool.Close()
		_, _ = admin.Exec(context.Background(), "DROP DATABASE IF EXISTS "+name)
	})
	return pool
}

// ensureRoles creates the roles the matrix fixtures grant to. Roles are cluster-wide,
// so we use CREATE ROLE IF NOT EXISTS (PG16+) — for older versions use DO blocks.
func ensureRoles(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(ctx, `
		DO $$BEGIN CREATE ROLE app_reader NOLOGIN; EXCEPTION WHEN duplicate_object THEN NULL; END$$;
		DO $$BEGIN CREATE ROLE app_writer NOLOGIN; EXCEPTION WHEN duplicate_object THEN NULL; END$$;
		DO $$BEGIN CREATE ROLE app_owner;          EXCEPTION WHEN duplicate_object THEN NULL; END$$;
	`)
	require.NoError(t, err)
}

// TestDump_RoundTrip is the correctness gate: for a representative set of
// matrix fixtures, apply the SQL to a fresh DB, dump it back, then run the
// differ between the dumped source and live. The expected outcome is ZERO
// differences — if anything diffs, the dump emitter is wrong.
//
// Build tag: integration. Run with:
//   go test -tags=integration ./pkg/dump/ -run TestDump_RoundTrip -v
func TestDump_RoundTrip(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Pick a few representative fixtures from test/matrix that exercise different
	// renderers. Step 26 includes everything from all earlier steps.
	fixtures := []string{"01_baseline", "10_index_add", "16_trigger_redef", "26_alter_owner"}
	for _, fx := range fixtures {
		t.Run(fx, func(t *testing.T) {
			roundTripFixture(t, ctx, fx)
		})
	}
}

func roundTripFixture(t *testing.T, ctx context.Context, fxName string) {
	t.Helper()
	pool := freshDB(t, ctx, "pgflux_dump_"+fxName)
	ensureRoles(t, ctx, pool)

	fxPath := filepath.Join("..", "..", "test", "matrix", fxName+".sql")
	sqlBytes, err := os.ReadFile(fxPath)
	require.NoError(t, err, "read fixture %s", fxPath)
	// Strip BEGIN/COMMIT markers if any; the matrix fixtures are plain SQL.
	if _, err := pool.Exec(ctx, string(sqlBytes)); err != nil {
		t.Fatalf("apply fixture %s: %v", fxName, err)
	}

	// Dump live to a temp directory.
	dumpDir := t.TempDir()
	res, err := Dump(ctx, pool, Options{
		OutputDir: dumpDir,
		Layout:    LayoutPerKind,
		Schemas:   []string{"public"},
		Force:     true,
	})
	require.NoError(t, err)
	require.Greater(t, res.Objects, 0, "expected non-zero objects dumped")

	// Reload from disk via the source parser.
	desired, err := src.LoadDesiredState(src.LoadOptions{SchemaDir: dumpDir})
	require.NoError(t, err, "reload dumped source")

	// Inspect live again to get a fresh model snapshot.
	live, err := inspector.Inspect(ctx, pool, inspector.Options{Schemas: []string{"public"}})
	require.NoError(t, err)
	pv, _ := pgver.Detect(ctx, pool)

	// Diff: expect zero actionable statements.
	dr, err := differ.Diff(desired, live, differ.Options{
		PGVersion:     pv,
		AllowMassDrop: true, // dump may shape a small subset; not a real concern here
	})
	require.NoError(t, err)
	var problems []string
	for _, s := range dr.Plan.Statements {
		if strings.TrimSpace(s.DDL) == "" {
			continue
		}
		problems = append(problems, fmt.Sprintf("[%s] %s", s.OpType, s.DDL))
	}
	if len(problems) > 0 {
		t.Fatalf("round-trip dirty for fixture %s — dump produced source that diffs against live:\n  %s",
			fxName, strings.Join(problems, "\n  "))
	}
}

// TestVerify_emptyOnPerfectMatch confirms a perfect dump → reload → verify cycle
// reports zero undeclared objects.
func TestVerify_emptyOnPerfectMatch(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool := freshDB(t, ctx, "pgflux_verify_clean")
	ensureRoles(t, ctx, pool)
	_, err := pool.Exec(ctx, `
		CREATE TABLE public.users (id bigserial PRIMARY KEY, email text NOT NULL);
		CREATE INDEX users_email_idx ON public.users (email);
	`)
	require.NoError(t, err)

	dumpDir := t.TempDir()
	_, err = Dump(ctx, pool, Options{OutputDir: dumpDir, Force: true, Schemas: []string{"public"}})
	require.NoError(t, err)
	desired, err := src.LoadDesiredState(src.LoadOptions{SchemaDir: dumpDir})
	require.NoError(t, err)
	live, err := inspector.Inspect(ctx, pool, inspector.Options{Schemas: []string{"public"}})
	require.NoError(t, err)

	report := Verify(desired, live)
	if report.Count() != 0 {
		var problems []string
		problems = append(problems, fmt.Sprintf("tables:%v", report.Tables))
		problems = append(problems, fmt.Sprintf("indexes:%v", report.Indexes))
		problems = append(problems, fmt.Sprintf("seqs:%v", report.Sequences))
		t.Fatalf("expected zero undeclared after dump→load; got:\n  %s", strings.Join(problems, "\n  "))
	}
}

// TestVerify_findsLiveOnlyObject confirms the asymmetric diff catches an object
// added to live after the schema was dumped (the manual-hotfix scenario).
func TestVerify_findsLiveOnlyObject(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool := freshDB(t, ctx, "pgflux_verify_drift")
	ensureRoles(t, ctx, pool)

	_, err := pool.Exec(ctx, `CREATE TABLE public.t (id int);`)
	require.NoError(t, err)
	dumpDir := t.TempDir()
	_, err = Dump(ctx, pool, Options{OutputDir: dumpDir, Force: true, Schemas: []string{"public"}})
	require.NoError(t, err)

	// Live drift: add a table outside the source.
	_, err = pool.Exec(ctx, `CREATE TABLE public.hotfix (k int);`)
	require.NoError(t, err)

	desired, err := src.LoadDesiredState(src.LoadOptions{SchemaDir: dumpDir})
	require.NoError(t, err)
	live, err := inspector.Inspect(ctx, pool, inspector.Options{Schemas: []string{"public"}})
	require.NoError(t, err)

	report := Verify(desired, live)
	require.Equal(t, []string{"public.hotfix"}, report.Tables)
	require.Equal(t, 1, report.Count())
}

// TestPull_quarantinesLiveOnlyObjects: same drift scenario, but pull collects
// the live-only objects into a quarantine file.
func TestPull_quarantinesLiveOnlyObjects(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool := freshDB(t, ctx, "pgflux_pull_drift")
	ensureRoles(t, ctx, pool)
	_, err := pool.Exec(ctx, `
		CREATE TABLE public.t (id int);
		CREATE TABLE public.hotfix (k int);
	`)
	require.NoError(t, err)

	// Source has only t, not hotfix.
	srcDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "s.sql"),
		[]byte("CREATE TABLE public.t (id int);"), 0o644))
	desired, err := src.LoadDesiredState(src.LoadOptions{SchemaDir: srcDir})
	require.NoError(t, err)
	live, err := inspector.Inspect(ctx, pool, inspector.Options{Schemas: []string{"public"}})
	require.NoError(t, err)

	// Dry-run: expect SQL to contain CREATE TABLE public.hotfix.
	r, err := Pull(desired, live, PullOptions{DryRun: true})
	require.NoError(t, err)
	require.Equal(t, 1, r.ObjectCount)
	require.Contains(t, r.SQL, "public.hotfix")
	require.NotContains(t, r.SQL, "public.t ") // trailing space avoids matching "hotfix"

	// Write mode: file should be created.
	outDir := t.TempDir()
	r2, err := Pull(desired, live, PullOptions{OutputDir: outDir})
	require.NoError(t, err)
	require.FileExists(t, r2.Filename)
}
