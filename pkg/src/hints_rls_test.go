package src

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRlsFromCreateTableDeparse(t *testing.T) {
	en, f := rlsFromCreateTableDeparse("CREATE TABLE t (id int) ENABLE ROW LEVEL SECURITY")
	require.True(t, en)
	require.False(t, f)
	en2, f2 := rlsFromCreateTableDeparse("CREATE TABLE t (id int) FORCE ROW LEVEL SECURITY")
	require.True(t, en2)
	require.True(t, f2)
}
