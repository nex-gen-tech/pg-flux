package hashstate

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nex-gen-tech/pg-flux/pkg/schema"
)

func TestOfSchemaState_AndNil(t *testing.T) {
	require.Equal(t, "0", OfSchemaState(nil))
	s := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.t": {Schema: "public", Name: "t", Columns: []*schema.Column{{Name: "a", TypeSQL: "int"}}},
	}}
	h1 := OfSchemaState(s)
	require.NotEqual(t, "0", h1)
	h2 := OfSchemaState(s)
	require.Equal(t, h1, h2)
}

func TestOfSchemaState_fullShape(t *testing.T) {
	s := &schema.SchemaState{
		Tables: map[string]*schema.Table{
			"public.p": {Schema: "public", Name: "p", ForeignKeys: []*schema.TableForeignKey{{Name: "f", DefSQL: "x"}}},
		},
		Indexes:   map[string]*schema.Index{"public.i1": {CreateSQL: "create index i on p (a)"}},
		Functions: map[string]*schema.Function{"public.f()": {Kind: "f", DefSQL: "y"}},
		Extensions: map[string]*schema.Extension{"e": {Version: "1", DefSQL: "z"}},
		ExtraDDL:  []string{"ANALYZE t"},
		MiscObjects: []*schema.MiscObject{{Kind: "e", DefSQL: "g"}},
		Policies: map[string]*schema.Policy{
			"public.p/p1": {DefSQL: "y", Roles: []string{"r1"}},
		},
		Views: map[string]*schema.View{
			"public.v": {DefSQL: "select 1"},
		},
		Sequences: map[string]*schema.Sequence{
			"public.s": {DefSQL: "seq"},
		},
		Triggers: map[string]*schema.Trigger{
			"public.p/tr": {DefSQL: "t"},
		},
	}
	h := OfSchemaState(s)
	require.Len(t, h, 64)
}
