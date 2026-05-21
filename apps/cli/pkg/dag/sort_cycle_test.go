package dag

import (
	"testing"

	"github.com/nex-gen-tech/pg-flux/pkg/plan"
	"github.com/stretchr/testify/require"
)

func TestCheckCycle_fkLoop(t *testing.T) {
	a := plan.Statement{
		ID: 1, OpType: "CREATE_TABLE", Object: "public.a",
		DDL: "CREATE TABLE a (b int REFERENCES b (id))",
	}
	b := plan.Statement{
		ID: 2, OpType: "CREATE_TABLE", Object: "public.b",
		DDL: "CREATE TABLE b (a int REFERENCES a (id))",
	}
	err := checkCycle([]plan.Statement{a, b})
	require.Error(t, err)
}

func TestFKCircularDependencyError_Error(t *testing.T) {
	e := &FKCircularDependencyError{}
	require.Contains(t, e.Error(), "fk dependency cycle")
	e2 := &FKCircularDependencyError{Tables: []string{"a", "b"}}
	require.Contains(t, e2.Error(), "a")
}
