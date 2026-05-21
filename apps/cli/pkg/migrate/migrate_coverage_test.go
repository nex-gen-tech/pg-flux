package migrate

// Issue 6: unit tests for buildMigrationSQL transaction wrapping.

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/nex-gen-tech/pg-flux/pkg/hazard"
	"github.com/nex-gen-tech/pg-flux/pkg/plan"
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
	// New format: YYYYMMDD_HHMMSS_mmm.sql -> 18 chars timestamp + ".sql".
	require.True(t, len(name) >= 22, "filename too short: %s", name)
	require.True(t, name[len(name)-4:] == ".sql", "must end in .sql: %s", name)
	// The current format always emits a 3-digit millisecond field.
	parsed, ok := ParseMigrationFilename(name)
	require.True(t, ok, "generated name must match pattern: %s", name)
	require.Len(t, parsed.Millis, 3, "millis must be 3 digits: %s", name)

	named := TimestampFilename("my label!")
	require.Contains(t, named, "my_label_")
	require.True(t, named[len(named)-4:] == ".sql")
}

// TestTimestampFilename_subSecondSortOrder is the regression test for issue #6:
// two calls within the same second (but different millis) must still sort in
// generation order lexically. Without millis, the label suffix decides order,
// which silently breaks apply order.
func TestTimestampFilename_subSecondSortOrder(t *testing.T) {
	base := time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC)
	// Three labels chosen so alphabetical-by-label disagrees with generation
	// order (z < y < x would sort backwards if millis were absent).
	first := timestampFilenameAt(base.Add(1*time.Millisecond), "zebra")
	second := timestampFilenameAt(base.Add(50*time.Millisecond), "yak")
	third := timestampFilenameAt(base.Add(271*time.Millisecond), "xenon")

	got := []string{third, first, second} // deliberately shuffled
	sort.Strings(got)
	require.Equal(t, []string{first, second, third}, got,
		"sub-second filenames must sort in generation order, got %v", got)

	// Confirm all three share the same HHMMSS but differ only in the millis
	// field — i.e. the millisecond field is what's tie-breaking.
	for _, f := range []string{first, second, third} {
		require.Contains(t, f, "20260521_120000_")
	}
}

// TestMigrationFilenamePattern_acceptsLegacy verifies that filenames written
// by older versions of pg-flux (no millisecond component) still match the
// shared pattern — the _pgflux.migrations tracking table stores raw filenames,
// so this is a hard backwards-compat requirement.
func TestMigrationFilenamePattern_acceptsLegacy(t *testing.T) {
	cases := []struct {
		name  string
		label string
		date  string
		clock string
	}{
		{"20260101_120000.sql", "", "20260101", "120000"},
		{"20260521_153045_initial.sql", "initial", "20260521", "153045"},
		{"20240307_000000_add_users_table.sql", "add_users_table", "20240307", "000000"},
	}
	for _, tc := range cases {
		p, ok := ParseMigrationFilename(tc.name)
		require.True(t, ok, "legacy filename must parse: %s", tc.name)
		require.Equal(t, tc.date, p.Date)
		require.Equal(t, tc.clock, p.Clock)
		require.Empty(t, p.Millis, "legacy filenames must have no millis: %s", tc.name)
		require.Equal(t, tc.label, p.Label)
	}
}

// TestMigrationFilenamePattern_acceptsCurrent verifies that the new
// millisecond-bearing filenames parse and round-trip through the helper.
func TestMigrationFilenamePattern_acceptsCurrent(t *testing.T) {
	cases := []struct {
		name   string
		label  string
		millis string
	}{
		{"20260101_120000_001.sql", "", "001"},
		{"20260521_153045_271_initial.sql", "initial", "271"},
		{"20240307_000000_999_add_users_table.sql", "add_users_table", "999"},
	}
	for _, tc := range cases {
		p, ok := ParseMigrationFilename(tc.name)
		require.True(t, ok, "current filename must parse: %s", tc.name)
		require.Equal(t, tc.millis, p.Millis)
		require.Equal(t, tc.label, p.Label)
	}
}

// TestMigrationFilenamePattern_rejectsGarbage guards against false positives.
func TestMigrationFilenamePattern_rejectsGarbage(t *testing.T) {
	bad := []string{
		"",
		"not_a_migration.sql",
		"20260101.sql",                       // no clock
		"20260101_120000.txt",                // wrong ext
		"20260101_120000_271_bad-label.sql",  // hyphen disallowed in label
		"20260101_12000.sql",                 // clock too short
		"2026010_120000.sql",                 // date too short
	}
	for _, n := range bad {
		_, ok := ParseMigrationFilename(n)
		require.False(t, ok, "must reject: %q", n)
	}
}

// TestMigrationFilenamePattern_ambiguousNumericPrefix documents that a 3-digit
// numeric prefix on the label is consumed as the millisecond field. This is
// intentional and matches the new-format generator output; legacy callers
// never produced filenames of the form "...HHMMSS_<3-digits>_<rest>" because
// labels weren't generated programmatically with that exact shape.
func TestMigrationFilenamePattern_ambiguousNumericPrefix(t *testing.T) {
	p, ok := ParseMigrationFilename("20260101_120000_271_initial.sql")
	require.True(t, ok)
	require.Equal(t, "271", p.Millis)
	require.Equal(t, "initial", p.Label)

	// A non-3-digit numeric prefix is just part of the label (no millis).
	p, ok = ParseMigrationFilename("20260101_120000_12_label.sql")
	require.True(t, ok)
	require.Empty(t, p.Millis)
	require.Equal(t, "12_label", p.Label)
}

// TestTimestampFilename_uniqueWithinMillisecond exercises the live clock to
// make sure back-to-back calls produce strictly non-decreasing filenames
// (i.e. the wall clock + millis combo is monotonic over a tight loop).
func TestTimestampFilename_uniqueWithinMillisecond(t *testing.T) {
	const n = 50
	names := make([]string, n)
	for i := range names {
		names[i] = TimestampFilename("loop")
	}
	for i := 1; i < n; i++ {
		require.True(t, names[i] >= names[i-1],
			"generation order must be monotonic: %s then %s", names[i-1], names[i])
	}
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
