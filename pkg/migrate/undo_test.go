package migrate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nexg/pg-flux/pkg/plan"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// GenerateUndoSQL
// ---------------------------------------------------------------------------

func TestGenerateUndoSQL_nilPlan(t *testing.T) {
	sql := GenerateUndoSQL(nil)
	require.Contains(t, sql, "nothing to undo")
}

func TestGenerateUndoSQL_emptyPlan(t *testing.T) {
	sql := GenerateUndoSQL(&plan.ExecutionPlan{})
	require.Contains(t, sql, "nothing to undo")
}

func TestGenerateUndoSQL_createTable(t *testing.T) {
	p := &plan.ExecutionPlan{Statements: []plan.Statement{
		{ID: 1, OpType: "CREATE_TABLE", Object: "public.users",
			DDL: `CREATE TABLE IF NOT EXISTS "public"."users" (id bigint)`},
	}}
	sql := GenerateUndoSQL(p)
	require.Contains(t, sql, "DROP TABLE IF EXISTS")
	require.Contains(t, sql, `"public"."users"`)
}

func TestGenerateUndoSQL_addColumn(t *testing.T) {
	p := &plan.ExecutionPlan{Statements: []plan.Statement{
		{ID: 1, OpType: "ADD_COLUMN", Object: "public.users.email",
			DDL: `ALTER TABLE "public"."users" ADD COLUMN IF NOT EXISTS "email" text`},
	}}
	sql := GenerateUndoSQL(p)
	require.Contains(t, sql, "DROP COLUMN IF EXISTS")
	require.Contains(t, sql, `"email"`)
}

func TestGenerateUndoSQL_dropTable_isManual(t *testing.T) {
	p := &plan.ExecutionPlan{Statements: []plan.Statement{
		{ID: 1, OpType: "DROP_TABLE", Object: "public.old_table",
			DDL: `DROP TABLE IF EXISTS "public"."old_table" CASCADE`},
	}}
	sql := GenerateUndoSQL(p)
	require.Contains(t, sql, "MANUAL")
	require.NotContains(t, sql, "BEGIN;", "no transaction block when only manual undos")
}

func TestGenerateUndoSQL_createIndex(t *testing.T) {
	p := &plan.ExecutionPlan{Statements: []plan.Statement{
		{ID: 1, OpType: "CREATE_INDEX", Object: "public.idx_users_email",
			DDL: `CREATE INDEX idx_users_email ON "public"."users" (email)`},
	}}
	sql := GenerateUndoSQL(p)
	require.Contains(t, sql, "DROP INDEX IF EXISTS")
	require.Contains(t, sql, "idx_users_email")
}

func TestGenerateUndoSQL_reverseOrder(t *testing.T) {
	// Verify that undo statements appear in reverse order of the forward plan.
	p := &plan.ExecutionPlan{Statements: []plan.Statement{
		{ID: 1, OpType: "CREATE_TABLE", Object: "public.a", DDL: `CREATE TABLE "public"."a" (id int)`},
		{ID: 2, OpType: "ADD_COLUMN", Object: "public.a.col", DDL: `ALTER TABLE "public"."a" ADD COLUMN "col" text`},
	}}
	sql := GenerateUndoSQL(p)
	dropColIdx := indexOf(sql, "DROP COLUMN")
	dropTblIdx := indexOf(sql, "DROP TABLE")
	// DROP COLUMN (undo of ADD_COLUMN, step 2) must come before DROP TABLE (undo of CREATE_TABLE, step 1)
	require.Greater(t, dropTblIdx, dropColIdx, "DROP COLUMN must precede DROP TABLE in undo script")
}

func TestGenerateUndoSQL_renameColumn(t *testing.T) {
	p := &plan.ExecutionPlan{Statements: []plan.Statement{
		{ID: 1, OpType: "RENAME_COLUMN", Object: "public.users.handle",
			DDL: `ALTER TABLE "public"."users" RENAME COLUMN "username" TO "handle"`},
	}}
	sql := GenerateUndoSQL(p)
	require.Contains(t, sql, "RENAME COLUMN")
	require.Contains(t, sql, `"handle"`)
	require.Contains(t, sql, `"username"`)
}

func TestGenerateUndoSQL_createExtension(t *testing.T) {
	p := &plan.ExecutionPlan{Statements: []plan.Statement{
		{ID: 1, OpType: "CREATE_EXTENSION", Object: "pg_trgm",
			DDL: `CREATE EXTENSION IF NOT EXISTS pg_trgm`},
	}}
	sql := GenerateUndoSQL(p)
	require.Contains(t, sql, "DROP EXTENSION IF EXISTS pg_trgm")
}

func TestGenerateUndoSQL_createSequence(t *testing.T) {
	p := &plan.ExecutionPlan{Statements: []plan.Statement{
		{ID: 1, OpType: "CREATE_SEQUENCE", Object: "public.invoice_seq",
			DDL: `CREATE SEQUENCE "public"."invoice_seq" START 1`},
	}}
	sql := GenerateUndoSQL(p)
	require.Contains(t, sql, "DROP SEQUENCE IF EXISTS")
	require.Contains(t, sql, "invoice_seq")
}

func TestGenerateUndoSQL_toggleRLSEnable(t *testing.T) {
	p := &plan.ExecutionPlan{Statements: []plan.Statement{
		{ID: 1, OpType: "TOGGLE_RLS", Object: "public.users",
			DDL: `ALTER TABLE "public"."users" ENABLE ROW LEVEL SECURITY`},
	}}
	sql := GenerateUndoSQL(p)
	require.Contains(t, sql, "DISABLE ROW LEVEL SECURITY")
}

func TestGenerateUndoSQL_createPolicy(t *testing.T) {
	p := &plan.ExecutionPlan{Statements: []plan.Statement{
		{ID: 1, OpType: "CREATE_POLICY", Object: "public.users.users_select",
			DDL: `CREATE POLICY users_select ON "public"."users" FOR SELECT USING (true)`},
	}}
	sql := GenerateUndoSQL(p)
	require.Contains(t, sql, "DROP POLICY IF EXISTS")
	require.Contains(t, sql, `"users_select"`)
}

// ---------------------------------------------------------------------------
// WriteUndoFile
// ---------------------------------------------------------------------------

func TestWriteUndoFile_writesFile(t *testing.T) {
	dir := t.TempDir()
	p := &plan.ExecutionPlan{Statements: []plan.Statement{
		{ID: 1, OpType: "CREATE_TABLE", Object: "public.t",
			DDL: `CREATE TABLE "public"."t" (id int)`},
	}}
	res := &GenerateResult{
		Filename:   filepath.Join(dir, "20260101_000000.sql"),
		Statements: p.Statements,
	}
	undoPath, err := WriteUndoFile(res)
	require.NoError(t, err)
	require.Contains(t, undoPath, "_undo.sql")
	content, err := os.ReadFile(undoPath)
	require.NoError(t, err)
	require.Contains(t, string(content), "DROP TABLE IF EXISTS")
}

func TestWriteUndoFile_nilResult(t *testing.T) {
	path, err := WriteUndoFile(nil)
	require.NoError(t, err)
	require.Empty(t, path)
}

// ---------------------------------------------------------------------------
// Repair / Baseline (unit, no DB — test helpers only)
// ---------------------------------------------------------------------------

func TestRepairOptions_emptyDir(t *testing.T) {
	// migrationFiles on a non-existent dir returns nil, no error.
	files, err := migrationFiles("/tmp/pg-flux-no-such-dir-abc123")
	require.NoError(t, err)
	require.Nil(t, files)
}

func TestBaselineOptions_upToFiltering(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"001.sql", "002.sql", "003.sql"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("SELECT 1;"), 0o644))
	}
	files, err := migrationFiles(dir)
	require.NoError(t, err)
	require.Len(t, files, 3)
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func indexOf(s, sub string) int {
	idx := 0
	for i := range s {
		if len(s[i:]) >= len(sub) && s[i:i+len(sub)] == sub {
			return idx
		}
		idx++
	}
	return -1
}
