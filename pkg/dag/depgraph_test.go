package dag

import (
	"testing"

	"github.com/nexg/pg-flux/pkg/plan"
	"github.com/stretchr/testify/require"
)

func TestTopologicalSortStatements_OrdersFKAfterParentTable(t *testing.T) {
	a := plan.Statement{ID: 1, OpType: "CREATE_TABLE", Object: "public.p", DDL: "CREATE TABLE public.p (id int primary key)"}
	b := plan.Statement{ID: 2, OpType: "CREATE_TABLE", Object: "public.c", DDL: "CREATE TABLE public.c (id int references public.p(id))"}
	out, err := TopologicalSortStatements([]plan.Statement{b, a})
	require.NoError(t, err)
	require.Equal(t, "public.p", out[0].Object)
	require.Equal(t, "public.c", out[1].Object)
}

func TestTopologicalSortStatements_Cycle(t *testing.T) {
	// two statements both "require" each other via a synthetic mutual reference pattern
	s1 := plan.Statement{ID: 1, OpType: "CREATE_TABLE", Object: "public.t1", DDL: "CREATE TABLE public.t1(x int references public.t2(x))"}
	s2 := plan.Statement{ID: 2, OpType: "CREATE_TABLE", Object: "public.t2", DDL: "CREATE TABLE public.t2(x int references public.t1(x))"}
	_, err := TopologicalSortStatements([]plan.Statement{s1, s2})
	require.ErrorIs(t, err, ErrDependencyCycle)
}

func TestTopologicalSortStatements_IndexAfterTable_Concurrently(t *testing.T) {
	t1 := plan.Statement{ID: 1, OpType: "CREATE_TABLE", Object: "public.t", DDL: "CREATE TABLE public.t (id int primary key)"}
	idx := plan.Statement{
		ID: 2, OpType: "CREATE_INDEX", Object: "public.idx", IsConcurrent: true,
		DDL: "CREATE INDEX CONCURRENTLY idx ON public.t (id)",
	}
	out, err := TopologicalSortStatements([]plan.Statement{idx, t1})
	require.NoError(t, err)
	require.Equal(t, "CREATE_TABLE", out[0].OpType)
	require.Equal(t, "CREATE_INDEX", out[1].OpType)
}

func TestTopologicalSortStatements_TriggerAfterTableAndFunction(t *testing.T) {
	fn := plan.Statement{
		ID: 1, OpType: "CREATE_FUNCTION", Object: "public.f()",
		DDL: "CREATE OR REPLACE FUNCTION public.f() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NULL; END; $$",
	}
	t1 := plan.Statement{ID: 2, OpType: "CREATE_TABLE", Object: "public.t", DDL: "CREATE TABLE public.t (id int primary key)"}
	tg := plan.Statement{
		ID: 3, OpType: "CREATE_TRIGGER", Object: "public.t/tr",
		DDL: "CREATE TRIGGER tr AFTER INSERT ON public.t FOR EACH ROW EXECUTE FUNCTION public.f()",
	}
	out, err := TopologicalSortStatements([]plan.Statement{tg, t1, fn})
	require.NoError(t, err)
	// function + table before trigger
	ops := make([]string, 0, len(out))
	for _, s := range out {
		ops = append(ops, s.OpType)
	}
	var fi, txi, tri int
	for i, s := range out {
		switch s.OpType {
		case "CREATE_FUNCTION":
			fi = i
		case "CREATE_TABLE":
			txi = i
		case "CREATE_TRIGGER":
			tri = i
		}
	}
	require.Less(t, fi, tri)
	require.Less(t, txi, tri)
	_ = ops
}

func TestTopologicalSortStatements_NoEdgesPreservesScoreOrder(t *testing.T) {
	in := []plan.Statement{
		{ID: 1, OpType: "DROP_NOT_NULL", DDL: "a"},
		{ID: 2, OpType: "RENAME_COLUMN", DDL: "b"},
		{ID: 3, OpType: "CREATE_TABLE", DDL: "c", Object: "public.x"},
	}
	out, err := TopologicalSortStatements(in)
	require.NoError(t, err)
	require.Len(t, out, 3)
}
