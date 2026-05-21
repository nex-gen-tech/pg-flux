package codegen

import (
	"testing"

	"github.com/nex-gen-tech/pg-flux/pkg/schema"
)

func TestFilter_emptyPassesEverything(t *testing.T) {
	f := Filter{}
	if !f.Empty() {
		t.Fatal("Empty() must be true on zero-value filter")
	}
	s := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.users": {Schema: "public", Name: "users"},
	}}
	got := f.ApplyToState(s)
	if got != s {
		t.Fatal("empty filter must return the input unmodified")
	}
}

func TestFilter_includeAllowlist(t *testing.T) {
	f := Filter{IncludeTables: []string{"users", "public.posts"}}
	s := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.users":   {Schema: "public", Name: "users"},
		"public.posts":   {Schema: "public", Name: "posts"},
		"public.secrets": {Schema: "public", Name: "secrets"},
	}}
	got := f.ApplyToState(s)
	if _, ok := got.Tables["public.users"]; !ok {
		t.Error("users should be kept")
	}
	if _, ok := got.Tables["public.posts"]; !ok {
		t.Error("posts should be kept")
	}
	if _, ok := got.Tables["public.secrets"]; ok {
		t.Error("secrets should be filtered out")
	}
}

func TestFilter_excludeDenylist(t *testing.T) {
	f := Filter{ExcludeTables: []string{"_pgflux_*"}}
	s := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.users":            {Schema: "public", Name: "users"},
		"public._pgflux_migrations": {Schema: "public", Name: "_pgflux_migrations"},
	}}
	got := f.ApplyToState(s)
	if _, ok := got.Tables["public.users"]; !ok {
		t.Error("users should be kept")
	}
	if _, ok := got.Tables["public._pgflux_migrations"]; ok {
		t.Error("_pgflux_migrations should be filtered out by wildcard")
	}
}

func TestFilter_excludeSchema(t *testing.T) {
	f := Filter{ExcludeSchemas: []string{"_pgflux"}}
	s := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.users":          {Schema: "public", Name: "users"},
		"_pgflux.migrations":    {Schema: "_pgflux", Name: "migrations"},
	}}
	got := f.ApplyToState(s)
	if _, ok := got.Tables["_pgflux.migrations"]; ok {
		t.Error("_pgflux schema should be excluded entirely")
	}
}

func TestFilter_indexesFollowParentTable(t *testing.T) {
	f := Filter{ExcludeTables: []string{"secrets"}}
	s := &schema.SchemaState{
		Tables: map[string]*schema.Table{
			"public.users":   {Schema: "public", Name: "users"},
			"public.secrets": {Schema: "public", Name: "secrets"},
		},
		Indexes: map[string]*schema.Index{
			"public.users_email_idx":  {Schema: "public", Name: "users_email_idx", TableSchema: "public", Table: "users"},
			"public.secrets_pin_idx":  {Schema: "public", Name: "secrets_pin_idx", TableSchema: "public", Table: "secrets"},
		},
	}
	got := f.ApplyToState(s)
	if _, ok := got.Indexes["public.users_email_idx"]; !ok {
		t.Error("user index should be kept")
	}
	if _, ok := got.Indexes["public.secrets_pin_idx"]; ok {
		t.Error("secrets index should be filtered with its parent table")
	}
}

func TestFilter_matchPatternGlob(t *testing.T) {
	if !matchPattern("user_*", "user_profiles") {
		t.Error("prefix glob should match")
	}
	if matchPattern("user_*", "secrets") {
		t.Error("no false positives")
	}
	if !matchPattern("[ab]_*", "a_thing") {
		t.Error("char class should match")
	}
}
