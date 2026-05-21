package differ

import (
	"strings"
	"testing"

	"github.com/nex-gen-tech/pg-flux/pkg/schema"
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

func diffOwnersHas(chs []change, needle string) bool {
	for _, c := range chs {
		if strings.Contains(c.rawSQL, needle) {
			return true
		}
	}
	return false
}

func TestDiffOwners_domain(t *testing.T) {
	d := &schema.SchemaState{Domains: map[string]*schema.Domain{
		"public.email": {Schema: "public", Name: "email", BaseType: "text", Owner: "new_owner"},
	}}
	l := &schema.SchemaState{Domains: map[string]*schema.Domain{
		"public.email": {Schema: "public", Name: "email", BaseType: "text", Owner: "old_owner"},
	}}
	assert.True(t, diffOwnersHas(diffOwners(d, l), "ALTER DOMAIN public.email OWNER TO new_owner"))
}

func TestDiffOwners_compositeType(t *testing.T) {
	d := &schema.SchemaState{CompositeTypes: map[string]*schema.CompositeType{
		"public.addr": {Schema: "public", Name: "addr", Owner: "new_owner"},
	}}
	l := &schema.SchemaState{CompositeTypes: map[string]*schema.CompositeType{
		"public.addr": {Schema: "public", Name: "addr", Owner: "old_owner"},
	}}
	assert.True(t, diffOwnersHas(diffOwners(d, l), "ALTER TYPE public.addr OWNER TO new_owner"))
}

func TestDiffOwners_rangeType(t *testing.T) {
	d := &schema.SchemaState{RangeTypes: map[string]*schema.RangeType{
		"public.intr": {Schema: "public", Name: "intr", Subtype: "int4", Owner: "new_owner"},
	}}
	l := &schema.SchemaState{RangeTypes: map[string]*schema.RangeType{
		"public.intr": {Schema: "public", Name: "intr", Subtype: "int4", Owner: "old_owner"},
	}}
	assert.True(t, diffOwnersHas(diffOwners(d, l), "ALTER TYPE public.intr OWNER TO new_owner"))
}

func TestDiffOwners_foreignTable(t *testing.T) {
	d := &schema.SchemaState{ForeignTables: map[string]*schema.ForeignTable{
		"public.rdb_users": {Schema: "public", Name: "rdb_users", Owner: "new_owner"},
	}}
	l := &schema.SchemaState{ForeignTables: map[string]*schema.ForeignTable{
		"public.rdb_users": {Schema: "public", Name: "rdb_users", Owner: "old_owner"},
	}}
	assert.True(t, diffOwnersHas(diffOwners(d, l), "ALTER FOREIGN TABLE public.rdb_users OWNER TO new_owner"))
}

func TestDiffOwners_foreignServer(t *testing.T) {
	d := &schema.SchemaState{ForeignServers: map[string]*schema.ForeignServer{
		"remote_db": {Name: "remote_db", Owner: "new_owner"},
	}}
	l := &schema.SchemaState{ForeignServers: map[string]*schema.ForeignServer{
		"remote_db": {Name: "remote_db", Owner: "old_owner"},
	}}
	assert.True(t, diffOwnersHas(diffOwners(d, l), "ALTER SERVER remote_db OWNER TO new_owner"))
}
