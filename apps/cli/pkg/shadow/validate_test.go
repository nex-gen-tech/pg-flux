package shadow

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nexg/pg-flux/pkg/plan"
)

func TestValidateSyntaxInTxn_NilOrEmpty(t *testing.T) {
	err := ValidateSyntaxInTxn(context.Background(), nil, nil)
	require.NoError(t, err)
	err = ValidateSyntaxInTxn(context.Background(), nil, &plan.ExecutionPlan{})
	require.NoError(t, err)
}

func TestValidateSyntaxOnDatabase_EmptyConnString(t *testing.T) {
	err := ValidateSyntaxOnDatabase(context.Background(), "", &plan.ExecutionPlan{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty connection string")
}

func TestValidateSemanticOnDatabase_EmptyConnString(t *testing.T) {
	err := ValidateSemanticOnDatabase(context.Background(), "", &plan.ExecutionPlan{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty connection string")
}

func TestValidateSemanticApply_NilOrEmpty(t *testing.T) {
	err := ValidateSemanticApply(context.Background(), nil, nil, "")
	require.NoError(t, err)
	err = ValidateSemanticApply(context.Background(), nil, &plan.ExecutionPlan{}, "")
	require.NoError(t, err)
}

func TestShadowLockID_deterministic(t *testing.T) {
	id1 := shadowLockID("postgres://host1/db1")
	id2 := shadowLockID("postgres://host1/db1")
	require.Equal(t, id1, id2)

	id3 := shadowLockID("postgres://host2/db1")
	require.NotEqual(t, id1, id3, "different connections must produce different lock IDs")
}

func TestShadowLockID_nonZero(t *testing.T) {
	id := shadowLockID("postgres://localhost/testdb")
	require.NotEqual(t, int64(0), id)
}


func TestSplitSQLForShadow_trailingNoSemicolon(t *testing.T) {
	stmts := splitSQLForShadow("SELECT 1")
	require.Len(t, stmts, 1)
	require.Equal(t, "SELECT 1", stmts[0])
}

func TestSplitSQLForShadow_singleQuote(t *testing.T) {
	stmts := splitSQLForShadow("INSERT INTO t VALUES ('hello; world'); SELECT 2;")
	require.Len(t, stmts, 2)
	require.Contains(t, stmts[0], "hello; world")
}

func TestSplitSQLForShadow_doubleQuotedIdent(t *testing.T) {
	stmts := splitSQLForShadow(`SELECT "weird;ident"; SELECT 2;`)
	require.Len(t, stmts, 2)
}

func TestSplitSQLForShadow_dollarQuote(t *testing.T) {
	sql := `CREATE FUNCTION f() RETURNS void AS $$
BEGIN
  PERFORM 1; -- semicolon inside
END;
$$; SELECT 2;`
	stmts := splitSQLForShadow(sql)
	require.Len(t, stmts, 2)
	require.Contains(t, stmts[0], "CREATE FUNCTION")
	require.Contains(t, stmts[1], "SELECT 2")
}

func TestSplitSQLForShadow_blockComment(t *testing.T) {
	stmts := splitSQLForShadow("/* comment; with semicolon */ SELECT 1;")
	require.Len(t, stmts, 1)
}

func TestSplitSQLForShadow_lineComment(t *testing.T) {
	// A statement that is purely a line comment should be omitted.
	stmts := splitSQLForShadow("-- just a comment\nSELECT 1;")
	require.Len(t, stmts, 1)
	require.Contains(t, stmts[0], "SELECT 1")
}

func TestSplitSQLForShadow_escapedSingleQuote(t *testing.T) {
	// Verify '' escape within a string literal doesn't break the splitter.
	stmts := splitSQLForShadow(`SELECT 'it''s fine'; SELECT 2;`)
	require.Len(t, stmts, 2)
}

func TestSplitSQLForShadow_consecutiveSemicolons(t *testing.T) {
	// Empty statements between semicolons should be skipped.
	stmts := splitSQLForShadow("SELECT 1;; SELECT 2;")
	require.Len(t, stmts, 2)
}

// TestReValidateTxnControl_lowercaseBeginCommit verifies lowercase begin/commit are stripped.
func TestReValidateTxnControl_lowercase(t *testing.T) {
	input := "begin;\nSELECT 1;\ncommit;"
	output := reValidateTxnControl.Replace(input)
	require.NotContains(t, output, "begin;")
	require.NotContains(t, output, "commit;")
	require.Contains(t, output, "SELECT 1")
}
