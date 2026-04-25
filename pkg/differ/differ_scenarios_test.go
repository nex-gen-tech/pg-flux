package differ

import (
	"strings"
	"testing"

	"github.com/nexg/pg-flux/pkg/plan"
	"github.com/nexg/pg-flux/pkg/schema"
	"github.com/stretchr/testify/require"
)

func TestDiff_dropColumn(t *testing.T) {
	des := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.t1": {Schema: "public", Name: "t1", Columns: []*schema.Column{{Name: "id", TypeSQL: "int"}}},
	}}
	live := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.t1": {Schema: "public", Name: "t1", Columns: []*schema.Column{
			{Name: "id", TypeSQL: "int"},
			{Name: "dropme", TypeSQL: "text"},
		}},
	}}
	dr, err := Diff(des, live, Options{})
	require.NoError(t, err)
	var saw bool
	for _, s := range dr.Plan.Statements {
		if s.OpType == string(plan.ChangeDropColumn) {
			saw = true
			require.Contains(t, strings.ToUpper(s.DDL), "DROP COLUMN")
		}
	}
	require.True(t, saw)
}

func TestDiff_createTable_greenfield(t *testing.T) {
	des := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.newt": {Schema: "public", Name: "newt", Columns: []*schema.Column{{Name: "id", TypeSQL: "int"}}},
	}}
	live := &schema.SchemaState{Tables: map[string]*schema.Table{}}
	dr, err := Diff(des, live, Options{})
	require.NoError(t, err)
	var saw bool
	for _, s := range dr.Plan.Statements {
		if s.OpType == string(plan.ChangeCreateTable) {
			saw = true
			require.Contains(t, s.DDL, "newt")
		}
	}
	require.True(t, saw)
}

func TestDiff_toggleRLS(t *testing.T) {
	des := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.t1": {Schema: "public", Name: "t1", RLSEnabled: true, RLSForced: true, Columns: []*schema.Column{{Name: "id", TypeSQL: "int"}}},
	}}
	live := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.t1": {Schema: "public", Name: "t1", RLSEnabled: false, Columns: []*schema.Column{{Name: "id", TypeSQL: "int"}}},
	}}
	dr, err := Diff(des, live, Options{})
	require.NoError(t, err)
	var saw bool
	for _, s := range dr.Plan.Statements {
		if s.OpType == string(plan.ChangeToggleRLS) {
			saw = true
		}
	}
	require.True(t, saw)
}

func TestDiff_createIndex(t *testing.T) {
	des := &schema.SchemaState{
		Tables: map[string]*schema.Table{
			"public.t1": {Schema: "public", Name: "t1", Columns: []*schema.Column{{Name: "id", TypeSQL: "int"}}},
		},
		Indexes: map[string]*schema.Index{
			"public.t1/idx1": {Schema: "public", Name: "idx1", TableSchema: "public", Table: "t1", CreateSQL: "CREATE INDEX idx1 ON public.t1 (id)"},
		},
	}
	live := &schema.SchemaState{Tables: des.Tables, Indexes: map[string]*schema.Index{}}
	dr, err := Diff(des, live, Options{})
	require.NoError(t, err)
	var saw bool
	for _, s := range dr.Plan.Statements {
		if s.OpType == string(plan.ChangeCreateIndex) {
			saw = true
		}
	}
	require.True(t, saw)
}

func TestDiff_setNotNull(t *testing.T) {
	des := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.t1": {Schema: "public", Name: "t1", Columns: []*schema.Column{{Name: "a", TypeSQL: "int", NotNull: true}}},
	}}
	live := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.t1": {Schema: "public", Name: "t1", Columns: []*schema.Column{{Name: "a", TypeSQL: "int", NotNull: false}}},
	}}
	dr, err := Diff(des, live, Options{})
	require.NoError(t, err)
	var saw bool
	for _, s := range dr.Plan.Statements {
		if s.OpType == "SET_NOT_NULL" {
			saw = true
		}
	}
	require.True(t, saw)
}
