package differ

import (
	"strings"
	"testing"

	"github.com/nexg/pg-flux/pkg/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiffOwners_tableOwnerChange(t *testing.T) {
	des := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.t": {Schema: "public", Name: "t", Owner: "app_owner"},
	}}
	live := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.t": {Schema: "public", Name: "t", Owner: "postgres"},
	}}
	dr, err := Diff(des, live, Options{})
	require.NoError(t, err)
	var saw bool
	for _, s := range dr.Plan.Statements {
		if strings.Contains(s.DDL, "ALTER TABLE public.t OWNER TO app_owner") {
			saw = true
		}
	}
	assert.True(t, saw, "expected ALTER TABLE … OWNER TO")
}

func TestDiffOwners_skipsWhenEitherSideEmpty(t *testing.T) {
	des := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.t": {Schema: "public", Name: "t", Owner: ""},
	}}
	live := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.t": {Schema: "public", Name: "t", Owner: "postgres"},
	}}
	dr, err := Diff(des, live, Options{})
	require.NoError(t, err)
	for _, s := range dr.Plan.Statements {
		assert.NotContains(t, s.DDL, "OWNER TO", "should not emit OWNER TO when desired owner unspecified")
	}
}

func TestDiffOwners_caseInsensitiveEqual(t *testing.T) {
	assert.True(t, ownerEqual("App_Owner", "app_owner"))
	assert.False(t, ownerEqual("app_owner", "other"))
}

func TestDiffOwners_functionOwnerChange(t *testing.T) {
	des := &schema.SchemaState{Functions: map[string]*schema.Function{
		"public.f()": {Schema: "public", Name: "f", Identity: "public.f()", Owner: "app_owner"},
	}}
	live := &schema.SchemaState{Functions: map[string]*schema.Function{
		"public.f()": {Schema: "public", Name: "f", Identity: "public.f()", Owner: "postgres"},
	}}
	dr, err := Diff(des, live, Options{})
	require.NoError(t, err)
	var saw bool
	for _, s := range dr.Plan.Statements {
		if strings.Contains(s.DDL, "ALTER FUNCTION public.f() OWNER TO app_owner") {
			saw = true
		}
	}
	assert.True(t, saw, "expected ALTER FUNCTION … OWNER TO")
}
