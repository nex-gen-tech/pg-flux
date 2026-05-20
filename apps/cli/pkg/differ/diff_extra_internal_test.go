package differ

import (
	"testing"

	"github.com/nexg/pg-flux/pkg/plan"
	"github.com/nexg/pg-flux/pkg/schema"
	"github.com/stretchr/testify/require"
)

func TestPoliciesEqual_and_helpers(t *testing.T) {
	require.True(t, policiesEqual(nil, nil))
	require.False(t, policiesEqual(&schema.Policy{}, nil))
	a := &schema.Policy{Cmd: "select", Permissive: true, UsingSQL: "true", WithCheck: "", Roles: []string{"b", "a"}}
	b := &schema.Policy{Cmd: "select", Permissive: true, UsingSQL: "true", WithCheck: "", Roles: []string{"a", "b"}}
	require.True(t, policiesEqual(a, b))
	require.Equal(t, "", normExpr("  \n\t"))
}

func TestDiffIndexesViewsSeqTrigFn_policies(t *testing.T) {
	d := &schema.SchemaState{
		Tables: map[string]*schema.Table{"public.t": {Schema: "public", Name: "t", Columns: []*schema.Column{{Name: "id", TypeSQL: "int"}}}},
		Indexes: map[string]*schema.Index{
			"public.i1": {Schema: "public", Name: "i1", TableSchema: "public", Table: "t", CreateSQL: "CREATE INDEX i1 ON public.t (id)"},
		},
		Views: map[string]*schema.View{
			"public.v1": {Schema: "public", Name: "v1", DefSQL: "CREATE VIEW public.v1 AS SELECT id FROM public.t"},
		},
		Sequences: map[string]*schema.Sequence{
			"public.s1": {Schema: "public", Name: "s1", DefSQL: "CREATE SEQUENCE public.s1"},
		},
		Triggers: map[string]*schema.Trigger{
			"public.t_trg": {Schema: "public", Table: "t", Name: "trg", DefSQL: "CREATE TRIGGER trg AFTER INSERT ON public.t FOR EACH ROW EXECUTE FUNCTION pg_notify('c', 'd')"},
		},
		Functions: map[string]*schema.Function{
			"public.f()": {Schema: "public", Name: "f", DefSQL: "CREATE FUNCTION public.f() RETURNS int LANGUAGE sql AS $$ SELECT 1 $$", Identity: "public.f()"},
		},
		Policies: map[string]*schema.Policy{
			"public.t/p1": {Schema: "public", Table: "t", Name: "p1", DefSQL: "CREATE POLICY p1 ON public.t FOR SELECT USING (true)", Cmd: "select", Permissive: true},
		},
	}
	l := &schema.SchemaState{
		Tables:    d.Tables,
		Indexes:   map[string]*schema.Index{},
		Views:     map[string]*schema.View{},
		Sequences: map[string]*schema.Sequence{},
		Triggers:  map[string]*schema.Trigger{},
		Functions: map[string]*schema.Function{},
		Policies:  map[string]*schema.Policy{},
	}
	ch := diffIndexes(d, l, nil)
	require.NotEmpty(t, ch)
	ch2 := diffViews(d, l)
	require.NotEmpty(t, ch2)
	ch3 := diffSequences(d, l)
	require.NotEmpty(t, ch3)
	ch4 := diffTriggers(d, l)
	require.NotEmpty(t, ch4)
	ch5 := diffFunctions(d, l)
	require.NotEmpty(t, ch5)
	ch6 := diffPolicies(d, l)
	require.NotEmpty(t, ch6)
	_ = plan.ChangeCreateIndex
	_ = ch
	_ = ch2
	_ = ch3
	_ = ch4
	_ = ch5
	_ = ch6
}

func TestIndexDefsEqual_fpHelpers(t *testing.T) {
	a := &schema.Index{CreateSQL: "CREATE INDEX a ON public.t (id)", TableSchema: "public"}
	b := &schema.Index{CreateSQL: "CREATE INDEX a ON public.t (id)", TableSchema: "public"}
	require.True(t, indexDefsEqual(a, b))
	require.NotEmpty(t, fpIndexSQL("CREATE INDEX a ON t (id)"))
	require.NotEmpty(t, fpFunctionSQL("CREATE FUNCTION f() RETURNS int language sql as $$ select 1 $$"))
}

func TestTableConstraintDefFingerprint(t *testing.T) {
	s := tableConstraintDefFingerprint("public", "t", "c1", "CHECK (x > 0)")
	require.NotEmpty(t, s)
}

func TestSortChangesDeterministic_tie(t *testing.T) {
	des := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.a": {Schema: "public", Name: "a"},
		"public.b": {Schema: "public", Name: "b"},
	}}
	ch := []change{
		{kind: plan.ChangeCreateTable, sch: "public", tbl: "b", t: &schema.Table{Schema: "public", Name: "b", Columns: []*schema.Column{{Name: "id", TypeSQL: "int"}}}},
		{kind: plan.ChangeCreateTable, sch: "public", tbl: "a", t: &schema.Table{Schema: "public", Name: "a", Columns: []*schema.Column{{Name: "id", TypeSQL: "int"}}}},
	}
	sortChangesDeterministic(des, ch)
	require.NotEmpty(t, changeTieKey(ch[0]))
}
