package migrate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// SplitUpDown
// ---------------------------------------------------------------------------

func TestSplitUpDown_combined(t *testing.T) {
	content := []byte(`-- +migrate Up

CREATE TABLE users (id int);

-- +migrate Down

DROP TABLE IF EXISTS users;
`)
	upSQL, downSQL, isCombined := SplitUpDown(content)
	require.True(t, isCombined)
	require.Contains(t, upSQL, "CREATE TABLE users")
	require.NotContains(t, upSQL, "DROP TABLE")
	require.Contains(t, downSQL, "DROP TABLE IF EXISTS users")
	require.NotContains(t, downSQL, "CREATE TABLE")
}

func TestSplitUpDown_legacy(t *testing.T) {
	content := []byte(`CREATE TABLE legacy (id int);
ALTER TABLE legacy ADD COLUMN name text;
`)
	upSQL, downSQL, isCombined := SplitUpDown(content)
	require.False(t, isCombined)
	require.Equal(t, string(content), upSQL)
	require.Empty(t, downSQL)
}

func TestSplitUpDown_upOnly(t *testing.T) {
	content := []byte(`-- +migrate Up

CREATE TABLE only_up (id int);
`)
	upSQL, downSQL, isCombined := SplitUpDown(content)
	require.True(t, isCombined)
	require.Contains(t, upSQL, "CREATE TABLE only_up")
	require.Empty(t, downSQL)
}

func TestSplitUpDown_empty(t *testing.T) {
	upSQL, downSQL, isCombined := SplitUpDown([]byte{})
	require.False(t, isCombined)
	require.Empty(t, upSQL)
	require.Empty(t, downSQL)
}

func TestSplitUpDown_caseInsensitive(t *testing.T) {
	content := []byte(`-- +migrate UP

CREATE TABLE ci_test (id int);

-- +MIGRATE DOWN

DROP TABLE IF EXISTS ci_test;
`)
	upSQL, downSQL, isCombined := SplitUpDown(content)
	require.True(t, isCombined)
	require.Contains(t, upSQL, "CREATE TABLE ci_test")
	require.Contains(t, downSQL, "DROP TABLE IF EXISTS ci_test")
}

// ---------------------------------------------------------------------------
// migrationFiles excludes _undo.sql
// ---------------------------------------------------------------------------

func TestMigrationFilesExcludesUndo(t *testing.T) {
	dir := t.TempDir()

	forward := "20260101_120000_foo.sql"
	undo := "20260101_120000_foo_undo.sql"
	require.NoError(t, os.WriteFile(filepath.Join(dir, forward), []byte("SELECT 1;"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, undo), []byte("-- undo"), 0o644))

	files, err := migrationFiles(dir)
	require.NoError(t, err)
	require.Len(t, files, 1)
	require.Equal(t, forward, filepath.Base(files[0]))
}

// ---------------------------------------------------------------------------
// ResolveDownSQL
// ---------------------------------------------------------------------------

func TestResolveDownSQL_combinedFile(t *testing.T) {
	dir := t.TempDir()
	content := `-- +migrate Up

CREATE TABLE users (id int);

-- +migrate Down

DROP TABLE IF EXISTS users;
`
	fname := "20260101_120000_add_users.sql"
	require.NoError(t, os.WriteFile(filepath.Join(dir, fname), []byte(content), 0o644))

	downSQL, err := ResolveDownSQL(dir, fname)
	require.NoError(t, err)
	require.Contains(t, downSQL, "DROP TABLE IF EXISTS users")
}

func TestResolveDownSQL_separateUndoFile(t *testing.T) {
	dir := t.TempDir()
	fname := "20260101_120000_add_users.sql"
	undoFname := "20260101_120000_add_users_undo.sql"
	// No combined markers in the forward file
	require.NoError(t, os.WriteFile(filepath.Join(dir, fname), []byte("CREATE TABLE users (id int);"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, undoFname), []byte("DROP TABLE IF EXISTS users;"), 0o644))

	downSQL, err := ResolveDownSQL(dir, fname)
	require.NoError(t, err)
	require.Contains(t, downSQL, "DROP TABLE IF EXISTS users")
}

func TestResolveDownSQL_neitherExists(t *testing.T) {
	dir := t.TempDir()
	fname := "20260101_120000_add_users.sql"
	// Nothing written — no forward file, no undo file

	downSQL, err := ResolveDownSQL(dir, fname)
	require.NoError(t, err)
	require.Empty(t, downSQL)
}

func TestResolveDownSQL_forwardOnlyNoMarkers(t *testing.T) {
	dir := t.TempDir()
	fname := "20260101_120000_add_users.sql"
	// Forward file exists but has no markers and no undo file
	require.NoError(t, os.WriteFile(filepath.Join(dir, fname), []byte("CREATE TABLE users (id int);"), 0o644))

	downSQL, err := ResolveDownSQL(dir, fname)
	require.NoError(t, err)
	require.Empty(t, downSQL)
}

// ---------------------------------------------------------------------------
// Rollback integration (requires DB — skip in unit mode)
// ---------------------------------------------------------------------------

func TestRollback_integration(t *testing.T) {
	t.Skip("requires DB: postgres://pgflux:pgflux@localhost:5440/pgflux")
}
