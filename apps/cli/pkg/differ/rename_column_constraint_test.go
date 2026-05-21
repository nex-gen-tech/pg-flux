package differ

// Tests for Fix #11: after a @renamed from=<old> column rename, subsequent
// "migrate generate" runs must not emit spurious DROP/ADD CONSTRAINT
// statements that reference the old column name.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nex-gen-tech/pg-flux/pkg/plan"
	"github.com/nex-gen-tech/pg-flux/pkg/schema"
	"github.com/nex-gen-tech/pg-flux/pkg/src"
)

// TestRenameColumn_noSpuriousConstraintAfterRename verifies the two-pass scenario:
//
//  1. First diff (live DB still has old column name "username"):
//     desired has  column "handle" with @renamed from=username
//     live has column "username"
//     → expects RENAME_COLUMN; no DROP/ADD CONSTRAINT
//
//  2. Second diff (live DB now has new column name "handle" — rename was applied):
//     desired has column "handle" with @renamed from=username
//     live has column "handle"
//     → expects empty plan (no spurious DROP/ADD CONSTRAINT for users_username_unique)
func TestRenameColumn_noSpuriousConstraintAfterRename(t *testing.T) {
	dir := t.TempDir()
	// Source schema: UNIQUE constraint written with old name "username",
	// but column is now called "handle" (with @renamed hint).
	sql := `CREATE TABLE public.users (
  id bigint PRIMARY KEY,
  -- @renamed from=username
  handle text NOT NULL,
  email text NOT NULL,
  CONSTRAINT users_username_unique UNIQUE (username)
);`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "users.sql"), []byte(sql), 0o644))
	desired, err := src.LoadDesiredState(src.LoadOptions{SchemaDir: dir})
	require.NoError(t, err)

	// Verify that the source loader rewrote the UNIQUE constraint to reference
	// the new column name.
	tbl := desired.Tables[schema.TableKey("public", "users")]
	require.NotNil(t, tbl)
	require.Len(t, tbl.Uniques, 1)
	require.NotContains(t, strings.ToLower(tbl.Uniques[0].DefSQL), "username",
		"source loader must rewrite UNIQUE (username) → UNIQUE (handle) after @renamed hint")
	require.Contains(t, strings.ToLower(tbl.Uniques[0].DefSQL), "handle",
		"source loader must have UNIQUE (handle) in DefSQL after @renamed hint")

	// -----------------------------------------------------------------------
	// Pass 1: live DB still has old column name "username". Expect RENAME_COLUMN.
	// -----------------------------------------------------------------------
	live1 := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.users": {
			Schema: "public", Name: "users",
			Columns: []*schema.Column{
				{Name: "id", TypeSQL: "bigint", NotNull: true, IsPrimaryKey: true},
				{Name: "username", TypeSQL: "text", NotNull: true},
				{Name: "email", TypeSQL: "text", NotNull: true},
			},
			Uniques: []*schema.TableUnique{
				{Name: "users_username_unique", DefSQL: "UNIQUE (username)"},
			},
		},
	}}

	dr1, err := Diff(desired, live1, Options{})
	require.NoError(t, err)
	var sawRename bool
	for _, s := range dr1.Plan.Statements {
		if s.OpType == string(plan.ChangeRenameColumn) && strings.Contains(s.DDL, "username") {
			sawRename = true
		}
		// There should be no DROP or ADD constraint in the first migration, since
		// the differ will apply the rename and match the constraint.
		if s.OpType == string(plan.ChangeDropConstraint) && strings.Contains(s.DDL, "users_username_unique") {
			t.Errorf("pass 1: unexpected DROP CONSTRAINT for users_username_unique: %s", s.DDL)
		}
	}
	require.True(t, sawRename, "pass 1: expected RENAME COLUMN username → handle")

	// -----------------------------------------------------------------------
	// Pass 2: live DB now has new column name "handle" — rename has been applied.
	// Expect empty plan (no spurious DROP/ADD CONSTRAINT).
	// -----------------------------------------------------------------------
	live2 := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.users": {
			Schema: "public", Name: "users",
			Columns: []*schema.Column{
				{Name: "id", TypeSQL: "bigint", NotNull: true, IsPrimaryKey: true},
				{Name: "handle", TypeSQL: "text", NotNull: true},
				{Name: "email", TypeSQL: "text", NotNull: true},
			},
			Uniques: []*schema.TableUnique{
				{Name: "users_username_unique", DefSQL: "UNIQUE (handle)"},
			},
		},
	}}

	dr2, err := Diff(desired, live2, Options{})
	require.NoError(t, err)
	for _, s := range dr2.Plan.Statements {
		if s.DDL == "" {
			continue
		}
		if s.OpType == string(plan.ChangeDropConstraint) {
			t.Errorf("pass 2: spurious DROP CONSTRAINT: %s", s.DDL)
		}
		if s.OpType == string(plan.ChangeAddConstraint) {
			t.Errorf("pass 2: spurious ADD CONSTRAINT: %s", s.DDL)
		}
	}
}
