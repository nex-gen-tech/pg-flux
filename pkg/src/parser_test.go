package src

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nexg/pg-flux/pkg/schema"
	"github.com/stretchr/testify/require"
)

func TestLoadDesiredState_CreateTable(t *testing.T) {
	dir := t.TempDir()
	sql := `CREATE TABLE items (
  id int PRIMARY KEY,
  name text NOT NULL
);
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.sql"), []byte(sql), 0o644))
	st, err := LoadDesiredState(LoadOptions{SchemaDir: dir})
	require.NoError(t, err)
	tb := st.Tables[schema.TableKey("public", "items")]
	require.NotNil(t, tb)
	require.Len(t, tb.Columns, 2)
	require.Equal(t, "id", tb.Columns[0].Name)
}

func TestRenameColumnHint(t *testing.T) {
	dir := t.TempDir()
	sql := `CREATE TABLE u (
  -- @renamed from=old_c
  new_c text
);
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.sql"), []byte(sql), 0o644))
	st, err := LoadDesiredState(LoadOptions{SchemaDir: dir})
	require.NoError(t, err)
	tb := st.Tables[schema.TableKey("public", "u")]
	require.Equal(t, "old_c", tb.Columns[0].RenameFrom)
}
