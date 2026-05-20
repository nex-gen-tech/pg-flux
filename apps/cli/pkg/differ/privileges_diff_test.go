package differ

import (
	"strings"
	"testing"

	"github.com/nexg/pg-flux/pkg/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiffPrivileges_grantToRole(t *testing.T) {
	des := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.t": {
			Schema: "public", Name: "t",
			Privileges: []schema.Privilege{
				{Grantee: "app_reader", Priv: "SELECT"},
				{Grantee: "app_writer", Priv: "SELECT"},
				{Grantee: "app_writer", Priv: "INSERT"},
			},
		},
	}}
	live := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.t": {Schema: "public", Name: "t",
			Privileges: []schema.Privilege{
				{Grantee: "app_reader", Priv: "SELECT"},
			},
		},
	}}
	dr, err := Diff(des, live, Options{})
	require.NoError(t, err)
	var grants int
	for _, s := range dr.Plan.Statements {
		if strings.HasPrefix(s.DDL, "GRANT ") {
			grants++
		}
	}
	assert.Equal(t, 2, grants, "expected GRANT for the two new privileges")
}

func TestDiffPrivileges_revoke(t *testing.T) {
	des := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.t": {Schema: "public", Name: "t",
			Privileges: []schema.Privilege{{Grantee: "app_reader", Priv: "SELECT"}},
		},
	}}
	live := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.t": {Schema: "public", Name: "t",
			Privileges: []schema.Privilege{
				{Grantee: "app_reader", Priv: "SELECT"},
				{Grantee: "app_reader", Priv: "INSERT"},
			},
		},
	}}
	dr, err := Diff(des, live, Options{})
	require.NoError(t, err)
	var saw bool
	for _, s := range dr.Plan.Statements {
		if strings.Contains(s.DDL, "REVOKE INSERT") {
			saw = true
		}
	}
	assert.True(t, saw, "expected REVOKE INSERT")
}

// If desired has no Privileges (no GRANT statements in source), don't touch live
// permissions — important to avoid accidental wipe-out.
func TestDiffPrivileges_noPrivsInSourceMeansNoChange(t *testing.T) {
	des := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.t": {Schema: "public", Name: "t"},
	}}
	live := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.t": {Schema: "public", Name: "t",
			Privileges: []schema.Privilege{{Grantee: "app_reader", Priv: "SELECT"}},
		},
	}}
	dr, err := Diff(des, live, Options{})
	require.NoError(t, err)
	for _, s := range dr.Plan.Statements {
		assert.False(t, strings.HasPrefix(s.DDL, "REVOKE"), "no REVOKE should fire when source has no GRANT statements")
	}
}

func TestDiffPrivileges_publicGrant(t *testing.T) {
	des := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.t": {Schema: "public", Name: "t",
			Privileges: []schema.Privilege{{Grantee: "", Priv: "SELECT"}},
		},
	}}
	live := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.t": {Schema: "public", Name: "t"},
	}}
	dr, err := Diff(des, live, Options{})
	require.NoError(t, err)
	var saw bool
	for _, s := range dr.Plan.Statements {
		if strings.Contains(s.DDL, "TO PUBLIC") {
			saw = true
		}
	}
	assert.True(t, saw)
}

func TestParseACLItem(t *testing.T) {
	got := schema.ParseACLItem("app=arwd/postgres")
	assert.Len(t, got, 4)
	assert.Equal(t, "app", got[0].Grantee)
	assert.Equal(t, "INSERT", got[0].Priv)
	assert.Equal(t, "SELECT", got[1].Priv)

	got2 := schema.ParseACLItem("=r/postgres")
	require.Len(t, got2, 1)
	assert.Equal(t, "", got2[0].Grantee) // PUBLIC
	assert.Equal(t, "SELECT", got2[0].Priv)

	got3 := schema.ParseACLItem("app=r*w/postgres")
	require.Len(t, got3, 2)
	assert.True(t, got3[0].WithGrantOption, "expected WGO on SELECT")
	assert.False(t, got3[1].WithGrantOption, "expected no WGO on UPDATE")
}
