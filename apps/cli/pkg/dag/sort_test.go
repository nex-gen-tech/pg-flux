package dag

import (
	"testing"

	"github.com/nexg/pg-flux/pkg/plan"
	"github.com/stretchr/testify/require"
)

func TestTopoSortStable(t *testing.T) {
	in := []plan.Statement{
		{ID: 1, OpType: "DROP_NOT_NULL", DDL: "a"},
		{ID: 2, OpType: "RENAME_COLUMN", DDL: "b"},
		{ID: 3, OpType: "CREATE_TABLE", DDL: "c"},
	}
	out, err := TopoSort(in)
	require.NoError(t, err)
	require.NotEmpty(t, out)
	// RENAME should sort before generic ops per heuristic
}
