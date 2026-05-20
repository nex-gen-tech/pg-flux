package schema

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtensionKey(t *testing.T) {
	require.Equal(t, "citext", ExtensionKey("CITEXT"))
	require.Equal(t, "pgcrypto", ExtensionKey("  pgcrypto  "))
}
