package src

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtensionVersionFromDefSQL(t *testing.T) {
	require.Equal(t, "1.2", ExtensionVersionFromDefSQL(`CREATE EXTENSION IF NOT EXISTS citext WITH VERSION 1.2`))
	require.Equal(t, "1.2.3", ExtensionVersionFromDefSQL(`CREATE EXTENSION t VERSION '1.2.3'`))
}
