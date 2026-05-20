package schema

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestKeyHelpers(t *testing.T) {
	require.Equal(t, "public.f()", FunctionKey("Public.F()"))
	require.Equal(t, "public.t/p", PolicyKey("", "T", "p"))
	require.Equal(t, "public.t1/tr", TriggerKey("", "t1", "TR"))
	require.Equal(t, "a.b", SeqKey("a", "b"))
	require.True(t, strings.HasPrefix(IndexKey("public", "Ix"), "public."))
}

func TestSchemaStateClone_roundTrip(t *testing.T) {
	s := &SchemaState{
		Tables: map[string]*Table{
			"public.t": {
				Schema: "public", Name: "t",
				Columns: []*Column{{Name: "id", TypeSQL: "int"}, {Name: "n", TypeSQL: "text"}},
				Checks:  []*TableCheck{{Name: "c1", DefSQL: "CHECK (id > 0)"}},
			},
		},
		ExtraDDL:    []string{"ATTACH x"},
		Extensions:  map[string]*Extension{"p": {Name: "p", Version: "1.0"}},
		MiscObjects: []*MiscObject{{Kind: "fdw", Name: "f"}},
	}
	c := s.Clone()
	require.NotNil(t, c)
	c.ExtraDDL = append(c.ExtraDDL, "second")
	require.Len(t, s.ExtraDDL, 1, "clone must not alias ExtraDDL backing array")
	require.Equal(t, "ATTACH x", c.ExtraDDL[0])
	c.Tables["public.t"].Columns[0].Name = "changed"
	require.Equal(t, "id", s.Tables["public.t"].Columns[0].Name)
}

func TestTableColumnByName(t *testing.T) {
	var nilT *Table
	require.Nil(t, nilT.ColumnByName("x"))
	tb := &Table{Columns: []*Column{{Name: "a", TypeSQL: "int"}, nil, {Name: "b", TypeSQL: "text"}}}
	require.Equal(t, "int", tb.ColumnByName("a").TypeSQL)
	require.Nil(t, tb.ColumnByName("z"))
}

func TestTableColumnNames(t *testing.T) {
	var nilT *Table
	require.Nil(t, nilT.ColumnNames())
	tb := &Table{Columns: []*Column{{Name: "a"}, nil, {Name: "b"}}}
	require.Equal(t, []string{"a", "b"}, tb.ColumnNames())
}
