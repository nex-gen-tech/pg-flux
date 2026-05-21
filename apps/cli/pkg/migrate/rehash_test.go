package migrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// migrationWithHash is a template for a synthetic migration file used by
// rehash tests.  The hash value is deliberately "olddeadhash" — it will not
// match the content hash after modification, which is the whole point.
const migrationWithHash = `-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.
-- pg-flux-baseline-hash: olddeadhash

BEGIN;

-- [1] CREATE_TABLE: public.orders
CREATE TABLE public.orders (id bigint PRIMARY KEY);

COMMIT;
`

// TestRehash_updatesHashLine is the canonical test described in the task:
//  1. Write a temporary migration file with a known (fake) hash.
//  2. Modify the file content (simulate a manual edit).
//  3. Run Rehash on it.
//  4. Read the file back; verify the new hash matches ContentHashOfMigration.
//  5. Verify no other lines were changed.
func TestRehash_updatesHashLine(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "20260521_123456_000.sql")

	// Step 1: write the initial file with a placeholder hash.
	require.NoError(t, os.WriteFile(fpath, []byte(migrationWithHash), 0o644))

	// Step 2: simulate a manual edit (remove the COMMIT line to represent a
	// user fixing a broken statement).
	edited := strings.ReplaceAll(migrationWithHash, "COMMIT;\n", "")
	require.NoError(t, os.WriteFile(fpath, []byte(edited), 0o644))

	// Verify the hash in the file does NOT yet match the content hash.
	currentContent, err := os.ReadFile(fpath)
	require.NoError(t, err)
	require.NotEqual(t, ContentHashOfMigration(currentContent), extractBaselineHash(currentContent),
		"precondition: old hash must differ from content hash before rehash")

	// Step 3: run Rehash.
	res, err := Rehash(fpath)
	require.NoError(t, err)
	require.True(t, res.HadHashLine, "rehash must report that a hash line was present")
	require.NotEmpty(t, res.NewHash)

	// Step 4: read the file back and verify the stored hash equals
	// ContentHashOfMigration of the current content.
	afterContent, err := os.ReadFile(fpath)
	require.NoError(t, err)

	storedHash := extractBaselineHash(afterContent)
	require.Equal(t, res.NewHash, storedHash, "stored hash must equal returned hash")
	require.Equal(t, ContentHashOfMigration(afterContent), storedHash,
		"stored hash must equal ContentHashOfMigration of the file after rehash")

	// Step 5: verify no other lines were changed — strip the hash line from
	// both the edited input and the rehashed output and compare them.
	withoutHashEdited := stripHashLine(edited)
	withoutHashAfter := stripHashLine(string(afterContent))
	require.Equal(t, withoutHashEdited, withoutHashAfter,
		"rehash must not change any content outside the hash line")
}

// TestRehash_noHashLine verifies that a file with no baseline-hash line is
// handled gracefully: no error, HadHashLine=false, file unchanged.
func TestRehash_noHashLine(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "manual.sql")

	content := "-- hand-written migration\nBEGIN;\nSELECT 1;\nCOMMIT;\n"
	require.NoError(t, os.WriteFile(fpath, []byte(content), 0o644))

	res, err := Rehash(fpath)
	require.NoError(t, err)
	require.False(t, res.HadHashLine, "file without hash line must set HadHashLine=false")
	require.Empty(t, res.NewHash)

	// File must be unchanged.
	afterContent, err := os.ReadFile(fpath)
	require.NoError(t, err)
	require.Equal(t, content, string(afterContent), "file without hash line must not be modified")
}

// TestRehash_missingFile verifies that a non-existent path returns an error.
func TestRehash_missingFile(t *testing.T) {
	_, err := Rehash("/nonexistent/path/to/migration.sql")
	require.Error(t, err)
}

// TestRehash_idempotent verifies that running rehash twice produces the same
// result. The hash after the second call must equal the hash after the first.
func TestRehash_idempotent(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "20260521_123456_001.sql")

	require.NoError(t, os.WriteFile(fpath, []byte(migrationWithHash), 0o644))

	res1, err := Rehash(fpath)
	require.NoError(t, err)
	require.True(t, res1.HadHashLine)

	res2, err := Rehash(fpath)
	require.NoError(t, err)
	require.True(t, res2.HadHashLine)

	require.Equal(t, res1.NewHash, res2.NewHash,
		"rehash must be idempotent: same hash on second run")
}

// TestContentHashOfMigration_excludesHashLine verifies that two files differing
// only in their baseline-hash value produce the same ContentHashOfMigration.
func TestContentHashOfMigration_excludesHashLine(t *testing.T) {
	base := `-- pg-flux generated migration
-- pg-flux-baseline-hash: HASH_PLACEHOLDER

BEGIN;
CREATE TABLE t (id int);
COMMIT;
`
	v1 := strings.ReplaceAll(base, "HASH_PLACEHOLDER", "aaaa")
	v2 := strings.ReplaceAll(base, "HASH_PLACEHOLDER", "bbbb")

	h1 := ContentHashOfMigration([]byte(v1))
	h2 := ContentHashOfMigration([]byte(v2))
	require.Equal(t, h1, h2,
		"two files differing only in their hash line must produce the same content hash")
}

// TestCheckBaselineDrift_acceptsRehash verifies that after Rehash, the
// checkBaselineDrift path that compares ContentHashOfMigration accepts the
// stored hash without needing to inspect the live DB.
func TestCheckBaselineDrift_acceptsRehash(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "20260521_123456_002.sql")

	// Start with a file that has an arbitrary (non-content) hash.
	require.NoError(t, os.WriteFile(fpath, []byte(migrationWithHash), 0o644))

	// Rehash it — now the file's stored hash equals its content hash.
	res, err := Rehash(fpath)
	require.NoError(t, err)
	require.True(t, res.HadHashLine)

	// Read back and verify that extractBaselineHash == ContentHashOfMigration.
	content, err := os.ReadFile(fpath)
	require.NoError(t, err)

	stored := extractBaselineHash(content)
	contentHash := ContentHashOfMigration(content)
	require.Equal(t, stored, contentHash,
		"after rehash, stored hash must equal content hash (enabling drift-check bypass)")
}
