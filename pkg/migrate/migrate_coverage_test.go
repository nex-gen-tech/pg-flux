package migrate

// Issue 6: unit tests for buildMigrationSQL transaction wrapping.

import (
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
	sql := buildMigrationSQL(p)
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
	sql := buildMigrationSQL(p)
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
	sql := buildMigrationSQL(p)
	require.Contains(t, sql, "ADVISORY")
	require.NotContains(t, sql, "BEGIN;", "advisory-only plan must not emit BEGIN/COMMIT")
	require.NotContains(t, sql, "COMMIT;")
}

// TestBuildMigrationSQL_emptyPlan returns the header but no transaction block.
func TestBuildMigrationSQL_emptyPlan(t *testing.T) {
	p := &plan.ExecutionPlan{Statements: []plan.Statement{}}
	sql := buildMigrationSQL(p)
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
