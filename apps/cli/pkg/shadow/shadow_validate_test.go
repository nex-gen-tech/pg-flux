package shadow

// Issue 9: unit tests for ValidateMigrationSQL and splitSQLForShadow.
// DB-requiring tests are tagged with TestMain-based docker/embedded skip logic
// (same pattern as equivalence_test.go).

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSplitSQLForShadow_basic checks that semicolons split statements correctly.
func TestSplitSQLForShadow_basic(t *testing.T) {
	sql := `
BEGIN;

ALTER TABLE public.t ADD COLUMN IF NOT EXISTS c text;

COMMIT;
`
	stmts := splitSQLForShadow(sql)
	// Expected: BEGIN, ALTER TABLE..., COMMIT  (3 non-empty, non-comment statements)
	require.Len(t, stmts, 3)
	require.Equal(t, "BEGIN", stmts[0])
	require.Contains(t, stmts[1], "ADD COLUMN")
	require.Equal(t, "COMMIT", stmts[2])
}

// TestSplitSQLForShadow_commentsStripped checks that comment-only lines are dropped.
func TestSplitSQLForShadow_commentsStripped(t *testing.T) {
	sql := `-- pg-flux generated migration
-- DO NOT EDIT

BEGIN;

-- [1] ADD_COLUMN: public.t.c
ALTER TABLE public.t ADD COLUMN IF NOT EXISTS c text;

COMMIT;
`
	stmts := splitSQLForShadow(sql)
	for _, s := range stmts {
		require.False(t, len(s) > 0 && s[0:2] == "--", "comment-only fragment must not appear in output: %q", s)
	}
}

// TestSplitSQLForShadow_empty returns nil for empty input.
func TestSplitSQLForShadow_empty(t *testing.T) {
	require.Nil(t, splitSQLForShadow(""))
	require.Nil(t, splitSQLForShadow("   \n  "))
}

// TestSplitSQLForShadow_multipleStmts verifies multiple statements are returned in order.
func TestSplitSQLForShadow_multipleStmts(t *testing.T) {
	sql := "CREATE TABLE a (id int); CREATE TABLE b (id int);"
	stmts := splitSQLForShadow(sql)
	require.Len(t, stmts, 2)
	require.Contains(t, stmts[0], "CREATE TABLE a")
	require.Contains(t, stmts[1], "CREATE TABLE b")
}

// TestValidateMigrationSQL_nilPool errors on nil pool.
func TestValidateMigrationSQL_nilPool(t *testing.T) {
	err := ValidateMigrationSQL(nil, nil, "test.sql", []byte("SELECT 1;"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "nil pool")
}

// TestReValidateTxnControl_stripsMarkers checks that the reValidateTxnControl replacer
// correctly removes BEGIN; / COMMIT; markers from migration file content.
func TestReValidateTxnControl_stripsMarkers(t *testing.T) {
	content := "BEGIN;\nALTER TABLE t ADD COLUMN c text;\nCOMMIT;\n"
	stripped := reValidateTxnControl.Replace(content)
	require.NotContains(t, stripped, "BEGIN;")
	require.NotContains(t, stripped, "COMMIT;")
	require.Contains(t, stripped, "ALTER TABLE t")
}
