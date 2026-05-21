package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestInitCommand(t *testing.T) {
	d := t.TempDir()
	t.Chdir(d)
	r := newRoot()
	r.SetArgs([]string{"init", "--dir", "./schema", "--with-examples=false"})
	err := r.Execute()
	require.NoError(t, err)
	// Config is written at the CWD (project root), not inside --dir.
	_, err = os.Stat(filepath.Join(d, ".pg-flux.yml"))
	require.NoError(t, err)
	// Schema dir is created inside --dir.
	_, err = os.Stat(filepath.Join(d, "schema"))
	require.NoError(t, err)
	// No subdirs like tables/, functions/ etc. should be created.
	_, err = os.Stat(filepath.Join(d, "schema", "tables"))
	require.True(t, os.IsNotExist(err), "schema/tables/ should not be created by init")
	// Migrations dir is created.
	_, err = os.Stat(filepath.Join(d, "migrations"))
	require.NoError(t, err)
}

// TestInitCommand_nonInteractiveSkipsPrompts verifies that when stdin is not a
// TTY (or PGFLUX_NON_INTERACTIVE is set), init silently uses defaults instead
// of mashing the "Schema directory [...]:" prompt into stdout.
func TestInitCommand_nonInteractiveSkipsPrompts(t *testing.T) {
	d := t.TempDir()
	t.Chdir(d)
	// Force non-interactive — covers the case the test process happens to have a TTY.
	t.Setenv("PGFLUX_NON_INTERACTIVE", "1")
	// Belt-and-braces: redirect stdin from /dev/null so even if the env var were
	// honored at the wrong time, the read would EOF immediately rather than hang.
	devnull, err := os.Open(os.DevNull)
	require.NoError(t, err)
	defer devnull.Close()
	oldStdin := os.Stdin
	os.Stdin = devnull
	t.Cleanup(func() { os.Stdin = oldStdin })

	var out bytes.Buffer
	r := newRoot()
	r.SetOut(&out)
	// Intentionally don't pass --dir / --migrations-dir so the prompts would fire.
	r.SetArgs([]string{"init", "--with-examples=false"})
	require.NoError(t, r.Execute())

	require.NotContains(t, out.String(), "Schema directory [",
		"non-tty init should not surface interactive prompts; got: %s", out.String())
	require.NotContains(t, out.String(), "Migrations directory [",
		"non-tty init should not surface interactive prompts; got: %s", out.String())
	// Defaults should still produce a config and the dirs.
	_, err = os.Stat(filepath.Join(d, ".pg-flux.yml"))
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(d, "schema"))
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(d, "migrations"))
	require.NoError(t, err)
}

// TestInitCommand_skipsExampleWhenSchemaDirNonEmpty verifies that init writes
// users.sql even when the schema dir already contains other files, but does not
// overwrite a pre-existing users.sql.
func TestInitCommand_skipsExampleWhenSchemaDirNonEmpty(t *testing.T) {
	d := t.TempDir()
	t.Chdir(d)
	require.NoError(t, os.MkdirAll(filepath.Join(d, "schema"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(d, "schema", "existing.sql"),
		[]byte("CREATE TABLE x (id int);\n"), 0o644))

	t.Setenv("PGFLUX_NON_INTERACTIVE", "1")
	r := newRoot()
	// --with-examples=true is the default.
	r.SetArgs([]string{"init"})
	require.NoError(t, r.Execute())

	// users.sql should be written because it does not yet exist.
	_, err := os.Stat(filepath.Join(d, "schema", "users.sql"))
	require.NoError(t, err, "init should write users.sql when it does not already exist")
	// Existing file is untouched.
	b, err := os.ReadFile(filepath.Join(d, "schema", "existing.sql"))
	require.NoError(t, err)
	require.Equal(t, "CREATE TABLE x (id int);\n", string(b))
}

// TestInitCommand_doesNotOverwriteExistingUsersSQL verifies that init skips
// schema/users.sql when the file already exists and preserves its content.
func TestInitCommand_doesNotOverwriteExistingUsersSQL(t *testing.T) {
	d := t.TempDir()
	t.Chdir(d)

	// Pre-create config and a schema/users.sql with custom content.
	cfgContent := "version: 1\nschema_dir: ./schema\nmigrations_dir: ./migrations\ntarget_schemas: [ public ]\n"
	require.NoError(t, os.WriteFile(filepath.Join(d, ".pg-flux.yml"), []byte(cfgContent), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(d, "schema"), 0o755))
	customSQL := "-- my custom schema\nCREATE TABLE custom (id int);\n"
	require.NoError(t, os.WriteFile(filepath.Join(d, "schema", "users.sql"), []byte(customSQL), 0o644))

	t.Setenv("PGFLUX_NON_INTERACTIVE", "1")
	var out bytes.Buffer
	r := newRoot()
	r.SetOut(&out)
	r.SetArgs([]string{"init", "--dir", "./schema", "--migrations-dir", "./migrations"})
	require.NoError(t, r.Execute())

	// Output must mention the skip.
	require.Contains(t, out.String(), "skipped schema/users.sql (already exists)",
		"init should report skipped file; got: %s", out.String())

	// File content must be unchanged.
	got, err := os.ReadFile(filepath.Join(d, "schema", "users.sql"))
	require.NoError(t, err)
	require.Equal(t, customSQL, string(got),
		"init must not overwrite pre-existing schema/users.sql")
}

// TestSilenceUsage_appliedRecursively verifies that every command in the tree
// has SilenceUsage=true so that a RunE error does not dump the full --help block.
func TestSilenceUsage_appliedRecursively(t *testing.T) {
	var check func(c *cobra.Command)
	check = func(c *cobra.Command) {
		require.Truef(t, c.SilenceUsage,
			"command %q has SilenceUsage=false; expected true", c.CommandPath())
		for _, sub := range c.Commands() {
			check(sub)
		}
	}
	check(newRoot())
}

// TestSchemaDirAlias verifies --schema is still accepted (kept as a hidden alias
// for back-compat) and that --schema-dir is the canonical flag name.
func TestSchemaDirAlias(t *testing.T) {
	// Reset between subtests.
	orig := schemaPath
	t.Cleanup(func() { schemaPath = orig })

	r := newRoot()
	require.NoError(t, r.PersistentFlags().Parse([]string{"--schema-dir", "/tmp/a"}))
	require.Equal(t, "/tmp/a", schemaPath)

	r2 := newRoot()
	require.NoError(t, r2.PersistentFlags().Parse([]string{"--schema", "/tmp/b"}))
	require.Equal(t, "/tmp/b", schemaPath, "--schema alias should still set schemaPath")

	// --schema must be hidden from --help.
	f := r2.PersistentFlags().Lookup("schema")
	require.NotNil(t, f)
	require.True(t, f.Hidden, "--schema should be a hidden alias")
}
