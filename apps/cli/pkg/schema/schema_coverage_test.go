package schema

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// ---- ConstraintKey ----

func TestConstraintKey_basic(t *testing.T) {
	require.Equal(t, "public.users/fk_org", ConstraintKey("public", "users", "fk_org"))
}

func TestConstraintKey_uppercased(t *testing.T) {
	// ConstraintKey lowercases the constraint name.
	require.Equal(t, "public.users/fk_org", ConstraintKey("public", "users", "FK_ORG"))
}

func TestConstraintKey_emptySchema(t *testing.T) {
	// Empty schema defaults to public.
	require.Equal(t, "public.users/chk", ConstraintKey("", "users", "chk"))
}

// ---- ViewKey ----

func TestViewKey_basic(t *testing.T) {
	require.Equal(t, "myschema.v1", ViewKey("myschema", "v1"))
}

func TestViewKey_emptySchema(t *testing.T) {
	require.Equal(t, "public.v1", ViewKey("", "v1"))
}

// ---- NormalizeTypeForCompare ----

func TestNormalizeTypeForCompare_aliases(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"int", "integer"},
		{"int4", "integer"},
		{"serial", "integer"},
		{"int8", "bigint"},
		{"bigserial", "bigint"},
		{"int2", "smallint"},
		{"smallserial", "smallint"},
		{"float4", "real"},
		{"float8", "double precision"},
		{"bool", "boolean"},
		{"pg_catalog.integer", "integer"},
		{"  INTEGER  ", "integer"},
		{"text", "text"},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			require.Equal(t, c.want, NormalizeTypeForCompare(c.in))
		})
	}
}

func TestNormalizeTypeForCompare_empty(t *testing.T) {
	require.Equal(t, "", NormalizeTypeForCompare(""))
	require.Equal(t, "", NormalizeTypeForCompare("   "))
}

// ---- BuildFunctionIdentity ----

func TestBuildFunctionIdentity_basic(t *testing.T) {
	got := BuildFunctionIdentity("public", "my_fn", "int, text")
	require.Equal(t, "public.my_fn(integer, text)", got)
}

func TestBuildFunctionIdentity_emptySchema(t *testing.T) {
	got := BuildFunctionIdentity("", "fn", "bigint")
	require.Equal(t, "public.fn(bigint)", got)
}

func TestBuildFunctionIdentity_noArgs(t *testing.T) {
	got := BuildFunctionIdentity("app", "fn", "")
	require.Equal(t, "app.fn()", got)
}

func TestBuildFunctionIdentity_nested(t *testing.T) {
	// splitTopLevelCommaTypes must not split inside parens.
	got := BuildFunctionIdentity("public", "f", "numeric(10,2), text")
	require.Equal(t, "public.f(numeric(10,2), text)", got)
}

// ---- Clone ----

func TestClone_nil(t *testing.T) {
	var s *SchemaState
	require.Nil(t, s.Clone())
}

func TestClone_empty(t *testing.T) {
	s := &SchemaState{}
	c := s.Clone()
	require.NotNil(t, c)
	require.NotNil(t, c.Tables)
	require.NotNil(t, c.Indexes)
}

func TestClone_tables(t *testing.T) {
	s := &SchemaState{
		Tables: map[string]*Table{
			"public.t1": {
				Schema: "public", Name: "t1",
				Columns: []*Column{{Name: "id", TypeSQL: "integer", NotNull: true}},
				Checks:  []*TableCheck{{Name: "chk1", DefSQL: "CHECK (id > 0)"}},
			},
		},
		Indexes: map[string]*Index{
			"public.t1/ix1": {Schema: "public", Name: "ix1", CreateSQL: "CREATE INDEX ix1 ON public.t1 (id)"},
		},
		ExtraDDL:    []string{"CREATE SCHEMA extra"},
		MiscObjects: []*MiscObject{{Kind: "GRANT", DefSQL: "GRANT SELECT ON ALL TABLES IN SCHEMA public TO r"}},
		Extensions: map[string]*Extension{"pgcrypto": {Name: "pgcrypto"}},
	}
	c := s.Clone()
	require.Equal(t, "integer", c.Tables["public.t1"].Columns[0].TypeSQL)
	require.Equal(t, "CHECK (id > 0)", c.Tables["public.t1"].Checks[0].DefSQL)
	require.Equal(t, "CREATE SCHEMA extra", c.ExtraDDL[0])
	require.Equal(t, "GRANT", c.MiscObjects[0].Kind)
	require.NotNil(t, c.Extensions["pgcrypto"])

	// Mutation of clone must not affect original.
	c.Tables["public.t1"].Columns[0].TypeSQL = "bigint"
	require.Equal(t, "integer", s.Tables["public.t1"].Columns[0].TypeSQL)
}

// ---- IndexKey / TableKey ----

func TestTableKey_emptySchema(t *testing.T) {
	require.Equal(t, "public.foo", TableKey("", "foo"))
}

func TestIndexKey_basic(t *testing.T) {
	// IndexKey is schema.name (two args).
	require.Equal(t, "public.idx1", IndexKey("public", "idx1"))
}

// ---- ReferenceTableKeyFromDefSQL ----

func TestReferenceTableKeyFromDefSQL_basic(t *testing.T) {
	got := ReferenceTableKeyFromDefSQL("FOREIGN KEY (org_id) REFERENCES public.orgs (id)")
	require.Equal(t, "public.orgs", got)
}

func TestReferenceTableKeyFromDefSQL_noSchema(t *testing.T) {
	got := ReferenceTableKeyFromDefSQL("FOREIGN KEY (org_id) REFERENCES orgs (id)")
	require.Equal(t, "public.orgs", got)
}

func TestReferenceTableKeyFromDefSQL_empty(t *testing.T) {
	require.Equal(t, "", ReferenceTableKeyFromDefSQL(""))
}

func TestReferenceTableKeyFromDefSQL_noMatch(t *testing.T) {
	require.Equal(t, "", ReferenceTableKeyFromDefSQL("CHECK (x > 0)"))
}

// ---- IndexKey empty schema ----

func TestIndexKey_emptySchema(t *testing.T) {
	require.Equal(t, "public.idx1", IndexKey("", "idx1"))
}

// ---- FunctionDependencyKey ----

func TestFunctionDependencyKey_withArgs(t *testing.T) {
	got := FunctionDependencyKey("public.my_fn(integer, text)")
	require.Contains(t, got, "my_fn")
}

// ---- Clone with functions, policies, views, sequences, triggers ----

func TestClone_allMaps(t *testing.T) {
	s := &SchemaState{
		Functions: map[string]*Function{
			"public.f(integer)": {Schema: "public", Name: "f", Identity: "public.f(integer)", DefSQL: "SELECT 1"},
		},
		Policies: map[string]*Policy{
			"public.t1/p1": {Schema: "public", Table: "t1", Name: "p1", Permissive: true},
		},
		Views: map[string]*View{
			"public.v1": {Schema: "public", Name: "v1", DefSQL: "CREATE VIEW public.v1 AS SELECT 1"},
		},
		Sequences: map[string]*Sequence{
			"public.s1": {Schema: "public", Name: "s1", DefSQL: "CREATE SEQUENCE public.s1"},
		},
		Triggers: map[string]*Trigger{
			"public.t1/trg1": {Schema: "public", Table: "t1", Name: "trg1"},
		},
	}
	c := s.Clone()
	require.NotNil(t, c.Functions["public.f(integer)"])
	require.NotNil(t, c.Policies["public.t1/p1"])
	require.NotNil(t, c.Views["public.v1"])
	require.NotNil(t, c.Sequences["public.s1"])
	require.NotNil(t, c.Triggers["public.t1/trg1"])
}

func TestStripOneOuterParensType_withInnerParens(t *testing.T) {
	// Outer parens but inner also has parens → not stripped, returned as-is.
	got := stripOneOuterParensType("(foo(int))")
	require.Equal(t, "(foo(int))", got)
}

func TestStripOneOuterParensType_noParens(t *testing.T) {
	require.Equal(t, "integer", stripOneOuterParensType("integer"))
}

func TestBuildFunctionIdentity_outerParens(t *testing.T) {
	got := BuildFunctionIdentity("public", "f", "(integer)")
	require.Equal(t, "public.f(integer)", got)
}
