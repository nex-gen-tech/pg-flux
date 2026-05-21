package differ

import (
	"strings"
	"testing"

	"github.com/nex-gen-tech/pg-flux/pkg/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiffCompositeTypes_addAttribute(t *testing.T) {
	des := &schema.SchemaState{CompositeTypes: map[string]*schema.CompositeType{
		"public.addr": {Schema: "public", Name: "addr", Attributes: []schema.CompositeAttribute{
			{Name: "street", Type: "text"}, {Name: "city", Type: "text"}, {Name: "zip", Type: "text"},
		}},
	}}
	live := &schema.SchemaState{CompositeTypes: map[string]*schema.CompositeType{
		"public.addr": {Schema: "public", Name: "addr", Attributes: []schema.CompositeAttribute{
			{Name: "street", Type: "text"}, {Name: "city", Type: "text"},
		}},
	}}
	dr, err := Diff(des, live, Options{})
	require.NoError(t, err)
	var saw bool
	for _, s := range dr.Plan.Statements {
		if strings.Contains(s.DDL, "ALTER TYPE public.addr ADD ATTRIBUTE zip text") {
			saw = true
		}
	}
	assert.True(t, saw)
}

func TestDiffCompositeTypes_dropAttribute(t *testing.T) {
	des := &schema.SchemaState{CompositeTypes: map[string]*schema.CompositeType{
		"public.a": {Schema: "public", Name: "a", Attributes: []schema.CompositeAttribute{
			{Name: "x", Type: "int"},
		}},
	}}
	live := &schema.SchemaState{CompositeTypes: map[string]*schema.CompositeType{
		"public.a": {Schema: "public", Name: "a", Attributes: []schema.CompositeAttribute{
			{Name: "x", Type: "int"}, {Name: "old", Type: "text"},
		}},
	}}
	dr, err := Diff(des, live, Options{})
	require.NoError(t, err)
	var saw bool
	for _, s := range dr.Plan.Statements {
		if strings.Contains(s.DDL, "DROP ATTRIBUTE old") {
			saw = true
		}
	}
	assert.True(t, saw)
}

func TestDiffCompositeTypes_alterType(t *testing.T) {
	des := &schema.SchemaState{CompositeTypes: map[string]*schema.CompositeType{
		"public.a": {Schema: "public", Name: "a", Attributes: []schema.CompositeAttribute{
			{Name: "x", Type: "bigint"},
		}},
	}}
	live := &schema.SchemaState{CompositeTypes: map[string]*schema.CompositeType{
		"public.a": {Schema: "public", Name: "a", Attributes: []schema.CompositeAttribute{
			{Name: "x", Type: "integer"},
		}},
	}}
	dr, err := Diff(des, live, Options{})
	require.NoError(t, err)
	var saw bool
	for _, s := range dr.Plan.Statements {
		if strings.Contains(s.DDL, "ALTER ATTRIBUTE x SET DATA TYPE bigint") {
			saw = true
		}
	}
	assert.True(t, saw)
}

func TestDiffCompositeTypes_dropType(t *testing.T) {
	des := &schema.SchemaState{}
	live := &schema.SchemaState{CompositeTypes: map[string]*schema.CompositeType{
		"public.legacy": {Schema: "public", Name: "legacy"},
	}}
	dr, err := Diff(des, live, Options{})
	require.NoError(t, err)
	var saw bool
	for _, s := range dr.Plan.Statements {
		if strings.Contains(s.DDL, "DROP TYPE IF EXISTS public.legacy") {
			saw = true
		}
	}
	assert.True(t, saw)
}
