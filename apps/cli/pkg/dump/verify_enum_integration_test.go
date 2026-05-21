//go:build integration

package dump

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/nex-gen-tech/pg-flux/pkg/inspector"
	"github.com/nex-gen-tech/pg-flux/pkg/migrate"
	"github.com/nex-gen-tech/pg-flux/pkg/src"
)

// TestVerify_enumDeclaredInSourceIsClean is the regression test for issue #9:
// before the loader was taught to capture CREATE TYPE ... AS ENUM into
// SchemaState.EnumValues, `pg-flux verify` reported every live enum as an
// "undeclared live object" even when the same enum was declared in source.
// Two commands (`drift` and `verify`) used the same loader but only verify
// inspected EnumValues directly — drift compared via raw ExtraDDL regex and so
// happened to work. After the fix, verify uses the same structured set.
func TestVerify_enumDeclaredInSourceIsClean(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool := freshDB(t, ctx, "pgflux_verify_enum_clean")
	ensureRoles(t, ctx, pool)

	// Apply the schema directly via pool.Exec to mimic `pg-flux apply` having
	// already run. The same DDL is what the source declares.
	_, err := pool.Exec(ctx, `
		CREATE TYPE public.todo_priority AS ENUM ('low','normal','high','urgent');
		CREATE TABLE public.todos (
			id       bigserial PRIMARY KEY,
			title    text NOT NULL,
			priority public.todo_priority NOT NULL DEFAULT 'normal'
		);
	`)
	require.NoError(t, err)

	// Write the matching source.
	srcDir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(srcDir, "types.sql"),
		[]byte("CREATE TYPE public.todo_priority AS ENUM ('low','normal','high','urgent');\n"),
		0o644))
	require.NoError(t, os.WriteFile(
		filepath.Join(srcDir, "todos.sql"),
		[]byte(`CREATE TABLE public.todos (
			id       bigserial PRIMARY KEY,
			title    text NOT NULL,
			priority public.todo_priority NOT NULL DEFAULT 'normal'
		);`),
		0o644))

	desired, err := src.LoadDesiredState(src.LoadOptions{SchemaDir: srcDir})
	require.NoError(t, err)

	// Sanity-check: the loader populated EnumValues for the declared enum.
	require.Contains(t, desired.EnumValues, "public.todo_priority",
		"loader should populate EnumValues from CREATE TYPE ... AS ENUM in source")
	require.Equal(t,
		[]string{"low", "normal", "high", "urgent"},
		desired.EnumValues["public.todo_priority"],
		"enum value ordering must be preserved")

	live, err := inspector.Inspect(ctx, pool, inspector.Options{Schemas: []string{"public"}})
	require.NoError(t, err)

	report := Verify(desired, live)
	if report.Count() != 0 {
		t.Fatalf("expected verify to be clean for declared enum; got %d undeclared:\n"+
			"  enums=%v\n  tables=%v\n  composite=%v\n  range=%v\n",
			report.Count(), report.Enums, report.Tables, report.CompositeTypes, report.RangeTypes)
	}
}

// TestVerify_enumMissingFromSourceIsFlagged is the adversarial negative case:
// when an enum exists in live but is NOT declared in source, verify must still
// report it as undeclared. This guards against the lazy "always return clean"
// fix that would have made the positive test pass while breaking detection.
func TestVerify_enumMissingFromSourceIsFlagged(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool := freshDB(t, ctx, "pgflux_verify_enum_drift")
	ensureRoles(t, ctx, pool)

	_, err := pool.Exec(ctx, `
		CREATE TYPE public.legacy_status AS ENUM ('open','closed');
		CREATE TYPE public.tracked AS ENUM ('a','b');
	`)
	require.NoError(t, err)

	// Source declares only one of the two enums.
	srcDir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(srcDir, "types.sql"),
		[]byte("CREATE TYPE public.tracked AS ENUM ('a','b');\n"),
		0o644))

	desired, err := src.LoadDesiredState(src.LoadOptions{SchemaDir: srcDir})
	require.NoError(t, err)
	live, err := inspector.Inspect(ctx, pool, inspector.Options{Schemas: []string{"public"}})
	require.NoError(t, err)

	report := Verify(desired, live)
	require.Contains(t, report.Enums, "public.legacy_status",
		"verify must still flag the live enum that source does not declare")
	require.NotContains(t, report.Enums, "public.tracked",
		"verify must NOT flag the live enum that source DOES declare")
}

// TestVerify_compositeAndRangeDeclaredInSourceAreClean exercises the other
// type-level kinds the loader was previously dropping (range) and confirms
// composite types (which already worked) still verify clean after the
// refactor.
func TestVerify_compositeAndRangeDeclaredInSourceAreClean(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool := freshDB(t, ctx, "pgflux_verify_typekinds")
	ensureRoles(t, ctx, pool)

	_, err := pool.Exec(ctx, `
		CREATE TYPE public.addr   AS (street text, city text);
		CREATE TYPE public.float8range AS RANGE (subtype = float8);
	`)
	require.NoError(t, err)

	srcDir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(srcDir, "types.sql"),
		[]byte(`CREATE TYPE public.addr AS (street text, city text);
CREATE TYPE public.float8range AS RANGE (subtype = float8);
`),
		0o644))

	desired, err := src.LoadDesiredState(src.LoadOptions{SchemaDir: srcDir})
	require.NoError(t, err)
	require.Contains(t, desired.CompositeTypes, "public.addr")
	require.Contains(t, desired.RangeTypes, "public.float8range",
		"loader must capture CREATE TYPE ... AS RANGE into RangeTypes")

	live, err := inspector.Inspect(ctx, pool, inspector.Options{Schemas: []string{"public"}})
	require.NoError(t, err)

	report := Verify(desired, live)
	if report.Count() != 0 {
		var sb strings.Builder
		report.WriteText(&sb)
		t.Fatalf("expected clean; got:\n%s", sb.String())
	}
}

// TestVerify_fastapiTodoExampleIsClean is the end-to-end smoke for issue #9:
// apply the published example migrations to a fresh DB, load the published
// schema/ directory as desired, and verify against live. Must report clean.
// This is the exact reproduction from iteration 11 of the journey doc.
func TestVerify_fastapiTodoExampleIsClean(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	pool := freshDB(t, ctx, "pgflux_verify_fastapi_todo")
	ensureRoles(t, ctx, pool)

	repoRoot := filepath.Join("..", "..", "..", "..")
	migrationsDir := filepath.Join(repoRoot, "examples", "fastapi-todo", "migrations")
	schemaDir := filepath.Join(repoRoot, "examples", "fastapi-todo", "schema")
	if _, err := os.Stat(migrationsDir); os.IsNotExist(err) {
		t.Skipf("fastapi-todo example not present at %s", migrationsDir)
	}

	// Use the real migrate.Apply so CONCURRENTLY indexes and other
	// statement-class-specific transaction handling are respected — applying
	// the file as a single Exec() would fail on CREATE INDEX CONCURRENTLY.
	res, err := migrate.Apply(ctx, pool, migrate.ApplyOptions{
		MigrationsDir: migrationsDir,
		Schemas:       []string{"public"},
	})
	require.NoError(t, err)
	require.NotEmpty(t, res.Applied, "expected at least one migration to apply from %s", migrationsDir)

	desired, err := src.LoadDesiredState(src.LoadOptions{SchemaDir: schemaDir})
	require.NoError(t, err)
	live, err := inspector.Inspect(ctx, pool, inspector.Options{Schemas: []string{"public"}})
	require.NoError(t, err)

	report := Verify(desired, live)
	if report.Count() != 0 {
		// Dump the full report so a regression is easy to read.
		var sb strings.Builder
		report.WriteText(&sb)
		t.Fatalf("expected verify clean on fastapi-todo example; got %d undeclared:\n%s",
			report.Count(), sb.String())
	}
}
