package differ

import (
	"testing"

	"github.com/nexg/pg-flux/pkg/plan"
	"github.com/nexg/pg-flux/pkg/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiff_AddColumn(t *testing.T) {
	des := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.t1": {Schema: "public", Name: "t1", Columns: []*schema.Column{
			{Name: "id", TypeSQL: "integer"},
			{Name: "x", TypeSQL: "text"},
		}},
	}}
	live := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.t1": {Schema: "public", Name: "t1", Columns: []*schema.Column{
			{Name: "id", TypeSQL: "integer"},
		}},
	}}
	dr, err := Diff(des, live, Options{})
	require.NoError(t, err)
	require.NotNil(t, dr.Plan)
	var hasAdd bool
	for _, s := range dr.Plan.Statements {
		if s.OpType == string(plan.ChangeAddColumn) {
			hasAdd = true
			assert.Contains(t, s.DDL, "ADD COLUMN x")
		}
	}
	assert.True(t, hasAdd, "expected ADD_COLUMN")
}

func TestDiff_RenameColumn(t *testing.T) {
	des := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.t1": {Schema: "public", Name: "t1", Columns: []*schema.Column{
			{Name: "a", TypeSQL: "text", RenameFrom: "old_a"},
		}},
	}}
	live := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.t1": {Schema: "public", Name: "t1", Columns: []*schema.Column{
			{Name: "old_a", TypeSQL: "text"},
		}},
	}}
	dr, err := Diff(des, live, Options{})
	require.NoError(t, err)
	var sawRename bool
	for _, s := range dr.Plan.Statements {
		if s.OpType == string(plan.ChangeRenameColumn) {
			sawRename = true
			assert.Contains(t, s.DDL, "RENAME COLUMN")
		}
	}
	assert.True(t, sawRename)
}

// When neither rename source nor target column exists, treat as greenfield ADD (rename hint ignored).
func TestDiff_RenameHintFallsBackToAddColumn(t *testing.T) {
	des := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.t1": {Schema: "public", Name: "t1", Columns: []*schema.Column{
			{Name: "a", TypeSQL: "text", RenameFrom: "nope"},
		}},
	}}
	live := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.t1": {Schema: "public", Name: "t1", Columns: []*schema.Column{
			{Name: "b", TypeSQL: "text"},
		}},
	}}
	dr, err := Diff(des, live, Options{})
	require.NoError(t, err)
	var sawAdd bool
	for _, s := range dr.Plan.Statements {
		if s.OpType == string(plan.ChangeAddColumn) && s.DDL != "" {
			sawAdd = true
			assert.Contains(t, s.DDL, "ADD COLUMN")
			assert.Contains(t, s.DDL, "a")
		}
	}
	assert.True(t, sawAdd)
}
