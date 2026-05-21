package dag

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nex-gen-tech/pg-flux/pkg/schema"
)

func TestTableCreationRank_Linear(t *testing.T) {
	s := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.parents": {Schema: "public", Name: "parents", Columns: []*schema.Column{{Name: "id", TypeSQL: "int"}}},
		"public.children": {
			Schema: "public", Name: "children",
			Columns: []*schema.Column{{Name: "id", TypeSQL: "int"}, {Name: "pid", TypeSQL: "int"}},
			ForeignKeys: []*schema.TableForeignKey{{
				Name: "fk", DefSQL: "FOREIGN KEY (pid) REFERENCES public.parents (id)",
			}},
		},
	}}
	r, err := TableCreationRank(s)
	require.NoError(t, err)
	require.Less(t, r["public.parents"], r["public.children"])
}

func TestTableCreationRank_Cycle(t *testing.T) {
	s := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.a": {
			Schema: "public", Name: "a",
			ForeignKeys: []*schema.TableForeignKey{{
				Name: "f", DefSQL: "FOREIGN KEY (b) REFERENCES public.b (x)",
			}},
		},
		"public.b": {
			Schema: "public", Name: "b",
			ForeignKeys: []*schema.TableForeignKey{{
				Name: "f", DefSQL: "FOREIGN KEY (a) REFERENCES public.a (x)",
			}},
		},
	}}
	_, err := TableCreationRank(s)
	require.Error(t, err)
	_, ok := err.(*FKCircularDependencyError)
	require.True(t, ok)
}
