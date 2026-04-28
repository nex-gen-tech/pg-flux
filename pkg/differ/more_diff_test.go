package differ

import (
	"strings"
	"testing"

	"github.com/nexg/pg-flux/pkg/plan"
	"github.com/nexg/pg-flux/pkg/schema"
	"github.com/stretchr/testify/require"
)

// ---- diffViews ----

func TestDiffViews_create(t *testing.T) {
	des := &schema.SchemaState{Views: map[string]*schema.View{
		"public.v1": {Schema: "public", Name: "v1", DefSQL: "CREATE VIEW public.v1 AS SELECT 1"},
	}}
	live := &schema.SchemaState{}
	changes := diffViews(des, live)
	require.Len(t, changes, 1)
	require.Equal(t, plan.ChangeCreateView, changes[0].kind)
}

func TestDiffViews_drop(t *testing.T) {
	des := &schema.SchemaState{}
	live := &schema.SchemaState{Views: map[string]*schema.View{
		"public.v1": {Schema: "public", Name: "v1", DefSQL: "CREATE VIEW public.v1 AS SELECT 1"},
	}}
	changes := diffViews(des, live)
	require.Len(t, changes, 1)
	require.Equal(t, plan.ChangeDropView, changes[0].kind)
}

func TestDiffViews_unchanged(t *testing.T) {
	sql := "CREATE VIEW public.v1 AS SELECT 1"
	des := &schema.SchemaState{Views: map[string]*schema.View{
		"public.v1": {Schema: "public", Name: "v1", DefSQL: sql},
	}}
	live := &schema.SchemaState{Views: map[string]*schema.View{
		"public.v1": {Schema: "public", Name: "v1", DefSQL: sql},
	}}
	require.Empty(t, diffViews(des, live))
}

func TestDiffViews_changed(t *testing.T) {
	// Use structurally different queries — pg_query.Fingerprint normalizes constants.
	des := &schema.SchemaState{Views: map[string]*schema.View{
		"public.v1": {Schema: "public", Name: "v1", DefSQL: "CREATE VIEW public.v1 AS SELECT id FROM public.t1"},
	}}
	live := &schema.SchemaState{Views: map[string]*schema.View{
		"public.v1": {Schema: "public", Name: "v1", DefSQL: "CREATE VIEW public.v1 AS SELECT id, name FROM public.t1"},
	}}
	changes := diffViews(des, live)
	// drop old + recreate new
	require.Len(t, changes, 2)
}

// ---- diffSequences ----

func TestDiffSequences_create(t *testing.T) {
	des := &schema.SchemaState{Sequences: map[string]*schema.Sequence{
		"public.s1": {Schema: "public", Name: "s1", DefSQL: "CREATE SEQUENCE public.s1"},
	}}
	live := &schema.SchemaState{}
	changes := diffSequences(des, live)
	require.Len(t, changes, 1)
	require.Equal(t, plan.ChangeCreateSequence, changes[0].kind)
}

func TestDiffSequences_drop(t *testing.T) {
	des := &schema.SchemaState{}
	live := &schema.SchemaState{Sequences: map[string]*schema.Sequence{
		"public.s1": {Schema: "public", Name: "s1", DefSQL: "CREATE SEQUENCE public.s1"},
	}}
	changes := diffSequences(des, live)
	require.Len(t, changes, 1)
	require.Equal(t, plan.ChangeDropSequence, changes[0].kind)
}

func TestDiffSequences_unchanged(t *testing.T) {
	sql := "CREATE SEQUENCE public.s1"
	des := &schema.SchemaState{Sequences: map[string]*schema.Sequence{
		"public.s1": {Schema: "public", Name: "s1", DefSQL: sql},
	}}
	live := &schema.SchemaState{Sequences: map[string]*schema.Sequence{
		"public.s1": {Schema: "public", Name: "s1", DefSQL: sql},
	}}
	require.Empty(t, diffSequences(des, live))
}

// ---- diffTriggers ----

func TestDiffTriggers_create(t *testing.T) {
	des := &schema.SchemaState{Triggers: map[string]*schema.Trigger{
		"public.t1/trg1": {Schema: "public", Table: "t1", Name: "trg1", DefSQL: "CREATE TRIGGER trg1 AFTER INSERT ON public.t1 EXECUTE FUNCTION f()"},
	}}
	live := &schema.SchemaState{}
	changes := diffTriggers(des, live)
	require.Len(t, changes, 1)
	require.Equal(t, plan.ChangeCreateTrigger, changes[0].kind)
}

func TestDiffTriggers_drop(t *testing.T) {
	des := &schema.SchemaState{}
	live := &schema.SchemaState{Triggers: map[string]*schema.Trigger{
		"public.t1/trg1": {Schema: "public", Table: "t1", Name: "trg1", DefSQL: "CREATE TRIGGER trg1 AFTER INSERT ON public.t1 EXECUTE FUNCTION f()"},
	}}
	changes := diffTriggers(des, live)
	require.Len(t, changes, 1)
	require.Equal(t, plan.ChangeDropTrigger, changes[0].kind)
}

// ---- diffTableConstraints ----

func TestDiffTableConstraints_addCheck(t *testing.T) {
	dt := &schema.Table{Schema: "public", Name: "t1", Checks: []*schema.TableCheck{
		{Name: "chk_pos", DefSQL: "CHECK (val > 0)"},
	}}
	lt := &schema.Table{Schema: "public", Name: "t1"}
	changes := diffTableConstraints(dt, lt, nil)
	require.Len(t, changes, 1)
	require.Equal(t, plan.ChangeAddConstraint, changes[0].kind)
}

func TestDiffTableConstraints_dropCheck(t *testing.T) {
	dt := &schema.Table{Schema: "public", Name: "t1"}
	lt := &schema.Table{Schema: "public", Name: "t1", Checks: []*schema.TableCheck{
		{Name: "chk_pos", DefSQL: "CHECK (val > 0)"},
	}}
	changes := diffTableConstraints(dt, lt, nil)
	require.Len(t, changes, 1)
	require.Equal(t, plan.ChangeDropConstraint, changes[0].kind)
}

func TestDiffTableConstraints_addFK(t *testing.T) {
	dt := &schema.Table{Schema: "public", Name: "t1", ForeignKeys: []*schema.TableForeignKey{
		{Name: "fk_org", DefSQL: "FOREIGN KEY (org_id) REFERENCES orgs(id)"},
	}}
	lt := &schema.Table{Schema: "public", Name: "t1"}
	changes := diffTableConstraints(dt, lt, nil)
	require.Len(t, changes, 1)
	require.Equal(t, plan.ChangeAddConstraint, changes[0].kind)
}

func TestDiffTableConstraints_addUnique(t *testing.T) {
	dt := &schema.Table{Schema: "public", Name: "t1", Uniques: []*schema.TableUnique{
		{Name: "uq_email", DefSQL: "UNIQUE (email)"},
	}}
	lt := &schema.Table{Schema: "public", Name: "t1"}
	changes := diffTableConstraints(dt, lt, nil)
	require.Len(t, changes, 1)
	require.Equal(t, plan.ChangeAddConstraint, changes[0].kind)
}

func TestDiffTableConstraints_unchanged(t *testing.T) {
	tbl := func() *schema.Table {
		return &schema.Table{Schema: "public", Name: "t1", Checks: []*schema.TableCheck{
			{Name: "chk_pos", DefSQL: "CHECK (val > 0)"},
		}}
	}
	require.Empty(t, diffTableConstraints(tbl(), tbl(), nil))
}

func TestDiffTableConstraints_nilTablesNew(t *testing.T) {
	require.Empty(t, diffTableConstraints(nil, &schema.Table{}, nil))
	require.Empty(t, diffTableConstraints(&schema.Table{}, nil, nil))
}

// ---- diffFunctions via Diff ----

func TestDiff_createFunction(t *testing.T) {
	fnSQL := "CREATE OR REPLACE FUNCTION public.add(a int, b int) RETURNS int LANGUAGE sql AS $$ SELECT a+b $$"
	des := &schema.SchemaState{
		Functions: map[string]*schema.Function{
			"public.add(integer, integer)": {Schema: "public", Name: "add", Identity: "public.add(integer, integer)", DefSQL: fnSQL},
		},
	}
	live := &schema.SchemaState{}
	dr, err := Diff(des, live, Options{})
	require.NoError(t, err)
	var saw bool
	for _, s := range dr.Plan.Statements {
		if s.OpType == string(plan.ChangeCreateFunction) {
			saw = true
			require.Contains(t, s.DDL, "add")
		}
	}
	require.True(t, saw, "expected CREATE_FUNCTION statement")
}

func TestDiff_dropFunction(t *testing.T) {
	fnSQL := "CREATE OR REPLACE FUNCTION public.add(a int, b int) RETURNS int LANGUAGE sql AS $$ SELECT a+b $$"
	des := &schema.SchemaState{}
	live := &schema.SchemaState{
		Functions: map[string]*schema.Function{
			"public.add(integer, integer)": {Schema: "public", Name: "add", Identity: "public.add(integer, integer)", DefSQL: fnSQL},
		},
	}
	dr, err := Diff(des, live, Options{})
	require.NoError(t, err)
	var saw bool
	for _, s := range dr.Plan.Statements {
		if s.OpType == string(plan.ChangeDropFunction) {
			saw = true
		}
	}
	require.True(t, saw, "expected DROP_FUNCTION statement")
}

// ---- diffPolicies via Diff ----

func TestDiff_createPolicy(t *testing.T) {
	des := &schema.SchemaState{
		Policies: map[string]*schema.Policy{
			"public.t1/p1": {Schema: "public", Table: "t1", Name: "p1", Permissive: true, Cmd: "SELECT", UsingSQL: "true"},
		},
	}
	live := &schema.SchemaState{}
	dr, err := Diff(des, live, Options{})
	require.NoError(t, err)
	var saw bool
	for _, s := range dr.Plan.Statements {
		if s.OpType == string(plan.ChangeCreatePolicy) {
			saw = true
		}
	}
	require.True(t, saw, "expected CREATE_POLICY statement")
}

// ---- quoteSQLIdent / quoteSQLString ----

func TestQuoteSQLIdent_noQuoteNeeded(t *testing.T) {
	require.Equal(t, "myfield", quoteSQLIdent("myfield"))
}

func TestQuoteSQLIdent_needsQuote(t *testing.T) {
	got := quoteSQLIdent("My Field")
	require.True(t, strings.HasPrefix(got, `"`))
}

func TestQuoteSQLIdent_empty(t *testing.T) {
	require.Equal(t, `""`, quoteSQLIdent(""))
}

func TestQuoteSQLIdent_reservedKeyword(t *testing.T) {
	// "order" and "select" are PostgreSQL RESERVED_KEYWORD — must always be quoted.
	require.Equal(t, `"order"`, quoteSQLIdent("order"))
	require.Equal(t, `"select"`, quoteSQLIdent("select"))
	require.Equal(t, `"table"`, quoteSQLIdent("table"))
}

func TestQuoteSQLIdent_digitStart(t *testing.T) {
	// Identifiers starting with a digit must be quoted.
	require.Equal(t, `"0myfield"`, quoteSQLIdent("0myfield"))
	require.Equal(t, `"9col"`, quoteSQLIdent("9col"))
}

func TestQuoteSQLString_basic(t *testing.T) {
	require.Equal(t, "'hello'", quoteSQLString("hello"))
}

func TestQuoteSQLString_escapesSingleQuote(t *testing.T) {
	require.Equal(t, "'it''s'", quoteSQLString("it's"))
}

// ---- diffSortChangesDeterministic (covers change_sort.go) ----

func TestSortChangesDeterministic_stable(t *testing.T) {
	des := &schema.SchemaState{Tables: map[string]*schema.Table{}}
	changes := []change{
		{kind: plan.ChangeDropColumn, sch: "public", tbl: "t2"},
		{kind: plan.ChangeCreateTable, t: &schema.Table{Schema: "public", Name: "t1"}},
		{kind: plan.ChangeDropColumn, sch: "public", tbl: "t1"},
	}
	// Should not panic.
	sortChangesDeterministic(des, changes)
	// CreateTable must come before DropColumn (lower opScore).
	require.Equal(t, plan.ChangeCreateTable, changes[0].kind)
}

func TestSortChangesDeterministic_views(t *testing.T) {
	// Two ChangeCreateView changes — changeTieKey should exercise view branch.
	v1 := &schema.View{Schema: "public", Name: "v1", DefSQL: "CREATE VIEW public.v1 AS SELECT 1"}
	v2 := &schema.View{Schema: "public", Name: "v2", DefSQL: "CREATE VIEW public.v2 AS SELECT 2"}
	des := &schema.SchemaState{
		Views: map[string]*schema.View{
			"public.v1": v1, "public.v2": v2,
		},
	}
	changes := []change{
		{kind: plan.ChangeCreateView, v: v2},
		{kind: plan.ChangeCreateView, v: v1},
	}
	sortChangesDeterministic(des, changes)
	// Should not panic regardless of order.
	require.Len(t, changes, 2)
}

func TestSortChangesDeterministic_dropView(t *testing.T) {
	des := &schema.SchemaState{Views: map[string]*schema.View{}}
	changes := []change{
		{kind: plan.ChangeDropView, viewKey: "public.v2"},
		{kind: plan.ChangeDropView, viewKey: "public.v1"},
	}
	sortChangesDeterministic(des, changes)
	require.Len(t, changes, 2)
}

func TestSortChangesDeterministic_withSeqAndTrig(t *testing.T) {
	des := &schema.SchemaState{}
	seq := &schema.Sequence{Schema: "public", Name: "s1"}
	trig := &schema.Trigger{Schema: "public", Table: "t1", Name: "trg1"}
	changes := []change{
		{kind: plan.ChangeDropSequence, dropSeq: "public.s1"},
		{kind: plan.ChangeCreateSequence, seq: seq},
		{kind: plan.ChangeCreateTrigger, trig: trig},
	}
	sortChangesDeterministic(des, changes)
	require.Len(t, changes, 3)
}

// ---- diffFunctions extra paths ----

func TestDiffFunctions_dropFunction(t *testing.T) {
	fnSQL := "CREATE OR REPLACE FUNCTION public.f1(a int) RETURNS int LANGUAGE sql AS $$ SELECT a $$"
	des := &schema.SchemaState{}
	live := &schema.SchemaState{
		Functions: map[string]*schema.Function{
			"public.f1(integer)": {Schema: "public", Name: "f1", Identity: "public.f1(integer)", DefSQL: fnSQL},
		},
	}
	changes := diffFunctions(des, live)
	require.Len(t, changes, 1)
	require.Equal(t, plan.ChangeDropFunction, changes[0].kind)
}

func TestDiffFunctions_updateFunction(t *testing.T) {
	// Different return types (structural difference) triggers a change.
	// Note: function bodies are string literals in pg_query, so fingerprint normalizes
	// body differences — use structurally different functions instead.
	fnV1 := "CREATE OR REPLACE FUNCTION public.f1(a int) RETURNS int LANGUAGE sql AS $$ SELECT a $$"
	fnV2 := "CREATE OR REPLACE FUNCTION public.f1(a int) RETURNS bigint LANGUAGE sql AS $$ SELECT a $$"
	des := &schema.SchemaState{
		Functions: map[string]*schema.Function{
			"public.f1(integer)": {Schema: "public", Name: "f1", Identity: "public.f1(integer)", DefSQL: fnV2},
		},
	}
	live := &schema.SchemaState{
		Functions: map[string]*schema.Function{
			"public.f1(integer)": {Schema: "public", Name: "f1", Identity: "public.f1(integer)", DefSQL: fnV1},
		},
	}
	changes := diffFunctions(des, live)
	// Changed function => recreate
	require.NotEmpty(t, changes)
}

func TestDiffFunctions_unchanged(t *testing.T) {
	fnSQL := "CREATE OR REPLACE FUNCTION public.f1(a int) RETURNS int LANGUAGE sql AS $$ SELECT a $$"
	fn := &schema.Function{Schema: "public", Name: "f1", Identity: "public.f1(integer)", DefSQL: fnSQL}
	des := &schema.SchemaState{Functions: map[string]*schema.Function{"public.f1(integer)": fn}}
	live := &schema.SchemaState{Functions: map[string]*schema.Function{"public.f1(integer)": fn}}
	require.Empty(t, diffFunctions(des, live))
}

// ---- diffPolicies extra paths ----

func TestDiffPolicies_dropPolicy(t *testing.T) {
	des := &schema.SchemaState{}
	live := &schema.SchemaState{
		Policies: map[string]*schema.Policy{
			"public.t1/p1": {Schema: "public", Table: "t1", Name: "p1", Permissive: true, Cmd: "SELECT"},
		},
	}
	changes := diffPolicies(des, live)
	require.Len(t, changes, 1)
	require.Equal(t, plan.ChangeDropPolicy, changes[0].kind)
}

func TestDiffPolicies_unchanged(t *testing.T) {
	pol := &schema.Policy{Schema: "public", Table: "t1", Name: "p1", Permissive: true, Cmd: "SELECT", UsingSQL: "true"}
	des := &schema.SchemaState{Policies: map[string]*schema.Policy{"public.t1/p1": pol}}
	live := &schema.SchemaState{Policies: map[string]*schema.Policy{"public.t1/p1": pol}}
	require.Empty(t, diffPolicies(des, live))
}

// ---- diffExtensions via Diff ----

func TestDiff_createExtension(t *testing.T) {
	des := &schema.SchemaState{
		Extensions: map[string]*schema.Extension{
			"pgcrypto": {Name: "pgcrypto", DefSQL: "CREATE EXTENSION IF NOT EXISTS pgcrypto"},
		},
	}
	live := &schema.SchemaState{}
	dr, err := Diff(des, live, Options{})
	require.NoError(t, err)
	var saw bool
	for _, s := range dr.Plan.Statements {
		if s.OpType == string(plan.ChangeCreateExtension) {
			saw = true
		}
	}
	require.True(t, saw, "expected CREATE_EXTENSION statement")
}

func TestDiff_dropExtension(t *testing.T) {
	// diffExtensions no longer auto-drops live extensions that are absent from the desired
	// schema — those could be DBA-managed extensions. Only extensions whose DefSQL
	// changes trigger a DROP+CREATE (handled separately). A live-only extension
	// (in live but not in desired at all) should be left alone.
	des := &schema.SchemaState{
		Extensions: map[string]*schema.Extension{}, // empty but non-nil = manage extensions
	}
	live := &schema.SchemaState{
		Extensions: map[string]*schema.Extension{
			"pgcrypto": {Name: "pgcrypto", DefSQL: "CREATE EXTENSION IF NOT EXISTS pgcrypto"},
		},
	}
	dr, err := Diff(des, live, Options{})
	require.NoError(t, err)
	for _, s := range dr.Plan.Statements {
		require.NotEqual(t, string(plan.ChangeDropExtension), s.OpType,
			"should not auto-drop unmanaged live extension")
	}
}

// ---- diffIndexes extra paths ----

func TestDiffIndexes_updateIndex(t *testing.T) {
	// Different CREATE INDEX SQL → drop + recreate.
	des := &schema.SchemaState{Indexes: map[string]*schema.Index{
		"public.idx1": {Schema: "public", Name: "idx1", TableSchema: "public", Table: "t1",
			CreateSQL: "CREATE INDEX idx1 ON public.t1 (id, name)"},
	}}
	live := &schema.SchemaState{Indexes: map[string]*schema.Index{
		"public.idx1": {Schema: "public", Name: "idx1", TableSchema: "public", Table: "t1",
			CreateSQL: "CREATE INDEX idx1 ON public.t1 (id)"},
	}}
	changes := diffIndexes(des, live, nil)
	require.Len(t, changes, 2) // drop + create
}

// ---- tableConstraintDefFingerprint ----

func TestTableConstraintDefFingerprint_basic(t *testing.T) {
	fp1 := tableConstraintDefFingerprint("public", "t1", "chk_pos", "CHECK (amount > 0)")
	fp2 := tableConstraintDefFingerprint("public", "t1", "chk_pos", "CHECK (amount > 0)")
	require.Equal(t, fp1, fp2)
}

func TestTableConstraintDefFingerprint_empty(t *testing.T) {
	require.Equal(t, "", tableConstraintDefFingerprint("public", "t1", "chk", ""))
}

// ---- policiesEqual extra paths ----

func TestPoliciesEqual_byRoles(t *testing.T) {
	a := &schema.Policy{Cmd: "SELECT", Permissive: true, Roles: []string{"b", "a"}}
	b := &schema.Policy{Cmd: "SELECT", Permissive: true, Roles: []string{"a", "b"}}
	require.True(t, policiesEqual(a, b))
}

func TestPoliciesEqual_differentRoles(t *testing.T) {
	a := &schema.Policy{Cmd: "SELECT", Permissive: true, Roles: []string{"admin"}}
	b := &schema.Policy{Cmd: "SELECT", Permissive: true, Roles: []string{"app"}}
	require.False(t, policiesEqual(a, b))
}

func TestPoliciesEqual_differentCmd(t *testing.T) {
	a := &schema.Policy{Cmd: "SELECT", Permissive: true}
	b := &schema.Policy{Cmd: "INSERT", Permissive: true}
	require.False(t, policiesEqual(a, b))
}

func TestPoliciesEqual_differentUsing(t *testing.T) {
	a := &schema.Policy{Cmd: "SELECT", Permissive: true, UsingSQL: "tenant_id = 1"}
	b := &schema.Policy{Cmd: "SELECT", Permissive: true, UsingSQL: "tenant_id = 2"}
	require.False(t, policiesEqual(a, b))
}

func TestPoliciesEqual_nilBoth(t *testing.T) {
	require.True(t, policiesEqual(nil, nil))
}

func TestPoliciesEqual_oneNil(t *testing.T) {
	require.False(t, policiesEqual(&schema.Policy{}, nil))
	require.False(t, policiesEqual(nil, &schema.Policy{}))
}
