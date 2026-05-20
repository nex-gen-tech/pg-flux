package main

import (
	"os"
	"path/filepath"
	"testing"

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
