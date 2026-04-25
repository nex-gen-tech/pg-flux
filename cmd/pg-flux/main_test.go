package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInitCommand(t *testing.T) {
	d := t.TempDir()
	r := newRoot()
	r.SetArgs([]string{"init", "--dir", d, "--with-examples=false"})
	err := r.Execute()
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(d, ".pg-flux.yml"))
	require.NoError(t, err)
}
