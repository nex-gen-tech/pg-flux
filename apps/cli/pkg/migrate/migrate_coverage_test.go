package migrate

// Issue 6: unit tests for buildMigrationSQL transaction wrapping.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nexg/pg-flux/pkg/hazard"
	"github.com/nexg/pg-flux/pkg/plan"
	"github.com/stretchr/testify/require"
)

// TestBuildMigrationSQL_transactionWrapping verifies that regular (non-concurrent) DDL
// is wrapped in BEGIN; / COMMIT; while CONCURRENT statements appear after COMMIT.
func TestBuildMigrationSQL_transactionWrapping(t *testing.T) {
	p := &plan.ExecutionPlan{
		Statements: []plan.Statement{
			{ID: 1, OpType: "ADD_COLUMN", Object: "public.t.c", DDL: "ALTER TABLE public.t ADD COLUMN c text"},
		},
	}
	sql := buildMigrationSQL(p, "")
	require.Contains(t, sql, "BEGIN;")
	require.Contains(t, sql, "COMMIT;")
	idx := func(s string) int { return strings.Index(sql, s) }
	require.Less(t, idx("BEGIN;"), idx("ADD COLUMN"), "BEGIN must precede DDL")
	require.Greater(t, idx("COMMIT;"), idx("ADD COLUMN"), "COMMIT must follow DDL")
}

// TestBuildMigrationSQL_concurrentAfterCommit verifies that CONCURRENT statements
// appear after the COMMIT block, outside the transaction.
func TestBuildMigrationSQL_concurrentAfterCommit(t *testing.T) {
	p := &plan.ExecutionPlan{
		Statements: []plan.Statement{
			{ID: 1, OpType: "ADD_COLUMN", Object: "public.t.c", DDL: "ALTER TABLE public.t ADD COLUMN c text"},
			{ID: 2, OpType: "CREATE_INDEX", Object: "public.idx", DDL: "CREATE INDEX CONCURRENTLY idx ON public.t (c)", IsConcurrent: true},
		},
	}
	sql := buildMigrationSQL(p, "")
	require.Contains(t, sql, "BEGIN;")
	require.Contains(t, sql, "COMMIT;")
	require.Contains(t, sql, "CONCURRENTLY")
	commitIdx := strings.Index(sql, "COMMIT;")
	concIdx := strings.Index(sql, "CONCURRENTLY")
	require.Less(t, commitIdx, concIdx, "CONCURRENT DDL must appear after COMMIT")
}

// TestBuildMigrationSQL_advisoryOnly verifies that advisory-only (DDL="" ) statements
// generate no BEGIN/COMMIT block.
func TestBuildMigrationSQL_advisoryOnly(t *testing.T) {
	p := &plan.ExecutionPlan{
		Statements: []plan.Statement{
			{ID: 1, OpType: "ADVISORY", Object: "public.t", DDL: "",
				Hazards: []hazard.Detected{{Severity: hazard.SeverityAdvisory, Type: hazard.ColumnReorder, Message: "advisory note"}}},
		},
	}
	sql := buildMigrationSQL(p, "")
	require.Contains(t, sql, "ADVISORY")
	require.NotContains(t, sql, "BEGIN;", "advisory-only plan must not emit BEGIN/COMMIT")
	require.NotContains(t, sql, "COMMIT;")
}

// TestBuildMigrationSQL_emptyPlan returns the header but no transaction block.
func TestBuildMigrationSQL_emptyPlan(t *testing.T) {
	p := &plan.ExecutionPlan{Statements: []plan.Statement{}}
	sql := buildMigrationSQL(p, "")
	require.NotContains(t, sql, "BEGIN;")
	require.NotContains(t, sql, "COMMIT;")
}

// TestSplitSQLStatements_basic verifies the migrator's statement splitter handles
// dollar-quoted functions and regular semicolons.
func TestSplitSQLStatements_basic(t *testing.T) {
	sql := `ALTER TABLE t ADD COLUMN c text;
CREATE FUNCTION f() RETURNS int LANGUAGE sql AS $$ SELECT 1 $$;
`
	stmts := splitSQLStatements(sql)
	require.Len(t, stmts, 2)
	require.Contains(t, stmts[0], "ADD COLUMN")
	require.Contains(t, stmts[1], "CREATE FUNCTION")
}

// TestReTransactionControl_filtersBeginCommit verifies that the regex used by applyOne
// strips BEGIN / COMMIT lines from migration file statements before execution
// (the Go transaction handles atomicity, so these markers are redundant and would error).
func TestReTransactionControl_filtersBeginCommit(t *testing.T) {
	cases := []struct {
		stmt  string
		match bool
	}{
		{"BEGIN", true},
		{"begin", true},
		{"COMMIT", true},
		{"commit", true},
		{"  BEGIN  ", true},
		{"ALTER TABLE t ADD COLUMN c text", false},
		{"BEGIN TRANSACTION", false}, // only bare BEGIN
	}
	for _, tc := range cases {
		got := reTransactionControl.MatchString(tc.stmt)
		require.Equal(t, tc.match, got, "stmt=%q", tc.stmt)
	}
}

// TestChecksum returns deterministic SHA-256 hex for the same input.
func TestChecksum(t *testing.T) {
	c1 := Checksum([]byte("hello"))
	c2 := Checksum([]byte("hello"))
	require.Equal(t, c1, c2)
	c3 := Checksum([]byte("world"))
	require.NotEqual(t, c1, c3)
	require.Len(t, c1, 64, "SHA-256 hex is 64 chars")
}

// TestTimestampFilename verifies filename format and label sanitisation.
func TestTimestampFilename(t *testing.T) {
	name := TimestampFilename("")
	require.True(t, len(name) >= 16, "filename too short: %s", name)
	require.True(t, name[len(name)-4:] == ".sql", "must end in .sql: %s", name)

	named := TimestampFilename("my label!")
	require.Contains(t, named, "my_label_")
	require.True(t, named[len(named)-4:] == ".sql")
}

// TestIsConcurrent matches CONCURRENTLY keyword only.
func TestIsConcurrent(t *testing.T) {
	require.True(t, isConcurrent("CREATE INDEX CONCURRENTLY idx ON t(c)"))
	require.True(t, isConcurrent("create index concurrently idx on t(c)"))
	require.False(t, isConcurrent("CREATE INDEX idx ON t(c)"))
	require.False(t, isConcurrent("ALTER TABLE t ADD COLUMN c text"))
}

// TestSplitSQLStatements_dollarQuote verifies nested dollar-quoting is not split.
func TestSplitSQLStatements_dollarQuote(t *testing.T) {
	sql := `CREATE FUNCTION f() RETURNS void LANGUAGE plpgsql AS $body$
BEGIN
  PERFORM 1;
END;
$body$;
ALTER TABLE t ADD COLUMN x text;
`
	stmts := splitSQLStatements(sql)
	require.Len(t, stmts, 2)
	require.Contains(t, stmts[0], "CREATE FUNCTION")
	require.Contains(t, stmts[1], "ADD COLUMN")
}

// TestSplitSQLStatements_singleQuote verifies semicolons inside string literals are ignored.
func TestSplitSQLStatements_singleQuote(t *testing.T) {
	sql := `INSERT INTO t VALUES ('hello; world'); ALTER TABLE t ADD COLUMN c text;`
	stmts := splitSQLStatements(sql)
	require.Len(t, stmts, 2)
	require.Contains(t, stmts[0], "hello; world")
	require.Contains(t, stmts[1], "ADD COLUMN")
}

// TestSplitSQLStatements_blockComment verifies block comments do not confuse the splitter.
func TestSplitSQLStatements_blockComment(t *testing.T) {
	sql := `/* this is a comment */ ALTER TABLE t ADD COLUMN c text; /* another */ SELECT 1;`
	stmts := splitSQLStatements(sql)
	require.Len(t, stmts, 2)
}

// TestSplitSQLStatements_emptyInput returns nil on empty input.
func TestSplitSQLStatements_emptyInput(t *testing.T) {
	stmts := splitSQLStatements("")
	require.Empty(t, stmts)
	stmts = splitSQLStatements("   \n  ")
	require.Empty(t, stmts)
}

// TestSplitSQLStatements_commentOnlyNoStmt verifies comment-only content produces no statements.
func TestSplitSQLStatements_commentOnlyNoStmt(t *testing.T) {
	sql := "-- just a comment\n-- another comment"
	stmts := splitSQLStatements(sql)
	require.Empty(t, stmts)
}

// TestSplitSQLStatements_doubleQuotedIdent verifies identifiers with semicolons don't split.
func TestSplitSQLStatements_doubleQuotedIdent(t *testing.T) {
	sql := `CREATE TABLE "weird;name" (id int); ALTER TABLE t ADD COLUMN c text;`
	stmts := splitSQLStatements(sql)
	require.Len(t, stmts, 2)
}

// TestBuildMigrationSQL_hazardComments verifies that blocking hazards appear as comments.
func TestBuildMigrationSQL_hazardComments(t *testing.T) {
	p := &plan.ExecutionPlan{
		Statements: []plan.Statement{
			{
				ID: 1, OpType: "ALTER_COLUMN", Object: "public.t.c",
				DDL: "ALTER TABLE public.t ALTER COLUMN c SET DATA TYPE bigint",
				Hazards: []hazard.Detected{{Severity: hazard.SeverityBlocking, Type: hazard.ColumnTypeChange, Message: "may cause cast failure"}},
			},
		},
	}
	sql := buildMigrationSQL(p, "")
	require.Contains(t, sql, "[HAZARD")
	require.Contains(t, sql, "cast failure")
}

// TestMigrationFiles_nonExistentDir returns nil error and nil slice.
func TestMigrationFiles_nonExistentDir(t *testing.T) {
	files, err := migrationFiles("/tmp/pg-flux-definitely-does-not-exist-xyz")
	require.NoError(t, err)
	require.Nil(t, files)
}

// TestMigrationFiles_emptyDir returns empty slice.
func TestMigrationFiles_emptyDir(t *testing.T) {
	dir := t.TempDir()
	files, err := migrationFiles(dir)
	require.NoError(t, err)
	require.Empty(t, files)
}

// TestMigrationFiles_sorted verifies .sql files are returned in lexical order.
func TestMigrationFiles_sorted(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"20260101.sql", "20260103.sql", "20260102.sql"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("SELECT 1;"), 0o644))
	}
	files, err := migrationFiles(dir)
	require.NoError(t, err)
	require.Len(t, files, 3)
	require.Contains(t, files[0], "20260101")
	require.Contains(t, files[1], "20260102")
	require.Contains(t, files[2], "20260103")
}

// TestMigrationFiles_skipsNonSQL verifies non-.sql files are ignored.
func TestMigrationFiles_skipsNonSQL(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "migration.sql"), []byte("SELECT 1;"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("# docs"), 0o644))
	files, err := migrationFiles(dir)
	require.NoError(t, err)
	require.Len(t, files, 1)
	require.Contains(t, files[0], "migration.sql")
}
