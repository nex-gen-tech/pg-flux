package differ

import (
	"testing"

	"github.com/nex-gen-tech/pg-flux/pkg/plan"
	"github.com/nex-gen-tech/pg-flux/pkg/schema"
	"github.com/stretchr/testify/require"
)

func TestFpGenericSQL(t *testing.T) {
	a := fpGenericSQL("  CREATE   EXTENSION   pgcrypto  ")
	b := fpGenericSQL("create extension pgcrypto")
	require.Equal(t, a, b)
}

func TestDiffExtraDDL_(t *testing.T) {
	require.Nil(t, diffExtraDDL(nil, nil))
	require.Nil(t, diffExtraDDL(&schema.SchemaState{}, nil))
	ch := diffExtraDDL(&schema.SchemaState{ExtraDDL: []string{"  ", "ALTER SYSTEM SET x = 1"}}, nil)
	require.Len(t, ch, 1)
	require.Equal(t, plan.ChangeRawSQL, ch[0].kind)
	require.Contains(t, ch[0].rawSQL, "ALTER SYSTEM")
}

func TestViewRank_executes(t *testing.T) {
	k1 := schema.TableKey("public", "v1")
	k2 := schema.TableKey("public", "v2")
	v1 := &schema.View{Schema: "public", Name: "v1", DefSQL: "SELECT 1 AS x"}
	v2 := &schema.View{Schema: "public", Name: "v2", DefSQL: "SELECT * FROM v1"}
	r := viewRank(&schema.SchemaState{Views: map[string]*schema.View{k1: v1, k2: v2}})
	require.NotNil(t, r)
}

func TestViewRank_nilViews(t *testing.T) {
	require.Nil(t, viewRank(nil))
	require.Nil(t, viewRank(&schema.SchemaState{}))
}

func TestDiffTableConstraints_addDropRename(t *testing.T) {
	dt := &schema.Table{
		Schema: "public", Name: "t",
		Checks: []*schema.TableCheck{{Name: "c1", DefSQL: "CHECK (id > 0)"}},
	}
	lt := &schema.Table{
		Schema: "public", Name: "t",
		Checks: []*schema.TableCheck{{Name: "c2", DefSQL: "CHECK (name IS NOT NULL)"}},
	}
	ch := diffTableConstraints(dt, lt, nil)
	require.NotEmpty(t, ch)
}

func TestDiffTableConstraints_nilTables(t *testing.T) {
	require.Empty(t, diffTableConstraints(nil, &schema.Table{}, nil))
	require.Empty(t, diffTableConstraints(&schema.Table{}, nil, nil))
}
