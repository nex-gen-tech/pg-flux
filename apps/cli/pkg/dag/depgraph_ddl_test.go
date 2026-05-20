package dag

import (
	"testing"

	"github.com/nexg/pg-flux/pkg/plan"
	"github.com/stretchr/testify/require"
)

func TestTopologicalSortStatements_CompositeTypeDep(t *testing.T) {
	// t references composite field type public.u
	tTy := plan.Statement{ID: 1, OpType: "RAW_DDL", Object: "raw", DDL: "CREATE TYPE public.t AS (x public.u);"}
	u := plan.Statement{ID: 2, OpType: "RAW_DDL", Object: "raw", DDL: "CREATE TYPE public.u AS (a int);"}
	out, err := TopologicalSortStatements([]plan.Statement{tTy, u})
	require.NoError(t, err)
	require.Contains(t, out[0].DDL, "public.u")
	require.Contains(t, out[1].DDL, "public.t")
}

func TestTopologicalSortStatements_TableColumnTypeAfterType(t *testing.T) {
	tbl := plan.Statement{
		ID: 1, OpType: "CREATE_TABLE", Object: "public.t1",
		DDL: "CREATE TABLE public.t1 (id int, payload public.ctype)",
	}
	ct := plan.Statement{ID: 2, OpType: "RAW_DDL", Object: "raw", DDL: "CREATE TYPE public.ctype AS (a int);"}
	out, err := TopologicalSortStatements([]plan.Statement{tbl, ct})
	require.NoError(t, err)
	require.Contains(t, out[0].DDL, "CREATE TYPE")
}

func TestTopologicalSortStatements_DomainASBaseType(t *testing.T) {
	base := plan.Statement{ID: 1, OpType: "RAW_DDL", Object: "raw", DDL: "CREATE TYPE public.money_t AS (cents bigint);"}
	dom := plan.Statement{ID: 2, OpType: "CREATE_DOMAIN", Object: "public.d", DDL: "CREATE DOMAIN public.d AS public.money_t"}
	out, err := TopologicalSortStatements([]plan.Statement{dom, base})
	require.NoError(t, err)
	require.Contains(t, out[0].DDL, "public.money_t")
}

func TestTopologicalSortStatements_RawDDLCreateTableColumnType(t *testing.T) {
	tbl := plan.Statement{ID: 1, OpType: "RAW_DDL", Object: "raw", DDL: "CREATE TABLE public.x (id int, v public.ut);"}
	ty := plan.Statement{ID: 2, OpType: "RAW_DDL", Object: "raw", DDL: "CREATE TYPE public.ut AS (n int);"}
	out, err := TopologicalSortStatements([]plan.Statement{tbl, ty})
	require.NoError(t, err)
	require.Contains(t, out[0].DDL, "CREATE TYPE")
}
