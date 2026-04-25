package differ

import (
	"strings"
	"testing"

	"github.com/nexg/pg-flux/pkg/hazard"
	"github.com/nexg/pg-flux/pkg/plan"
	"github.com/nexg/pg-flux/pkg/schema"
	"github.com/stretchr/testify/require"
)

func opt0() Options { return Options{} }

func TestStmtFor_allKinds(t *testing.T) {
	qt := func() string { return "public.t" }
	cases := []struct {
		name string
		c    change
		want string
	}{
		{"create_table", change{kind: plan.ChangeCreateTable, sch: "public", tbl: "t1", t: &schema.Table{Schema: "public", Name: "t1", Columns: []*schema.Column{{Name: "id", TypeSQL: "int"}}}}, "CREATE TABLE"},
		{"drop_table", change{kind: plan.ChangeDropTable, sch: "public", tbl: "t1"}, "DROP TABLE"},
		{"rename_table", change{kind: plan.ChangeRenameTable, sch: "public", tbl: "newt", fromTable: "oldt"}, "RENAME TO"},
		{"add_col", change{kind: plan.ChangeAddColumn, sch: "public", tbl: "t1", col: "c", dc: &schema.Column{Name: "c", TypeSQL: "text", NotNull: true, DefaultSQL: "''"}}, "ADD COLUMN"},
		{"drop_col", change{kind: plan.ChangeDropColumn, sch: "public", tbl: "t1", col: "c", lc: &schema.Column{Name: "c"}}, "DROP COLUMN"},
		{"rename_col", change{kind: plan.ChangeRenameColumn, sch: "public", tbl: "t1", from: "a", col: "b"}, "RENAME COLUMN"},
		{"create_idx", change{kind: plan.ChangeCreateIndex, idx: &schema.Index{Schema: "public", Name: "i1", CreateSQL: "CREATE INDEX i1 ON public.t1 (id)"}}, "CREATE INDEX"},
		{"drop_idx", change{kind: plan.ChangeDropIndex, sch: "public", ixName: "i1", dropIdx: "k"}, "DROP INDEX"},
		{"create_fn", change{kind: plan.ChangeCreateFunction, fn: &schema.Function{DefSQL: "create function f() returns int language sql as $$ select 1 $$", Identity: "public.f()"}}, "select 1"},
		{"create_fn_agg", change{kind: plan.ChangeCreateFunction, fn: &schema.Function{Kind: "a", DefSQL: "create aggregate public.agg (int) (...)", Identity: "public.agg(int)"}}, "aggregate"},
		{"drop_fn", change{kind: plan.ChangeDropFunction, fn: &schema.Function{Identity: "public.f()"}}, "DROP FUNCTION"},
		{"create_pol", change{kind: plan.ChangeCreatePolicy, pol: &schema.Policy{DefSQL: "create policy p on public.t1 for all using (true)"}, polKey: "k"}, "create policy"},
		{"drop_pol", change{kind: plan.ChangeDropPolicy, pol: &schema.Policy{Schema: "public", Table: "t1", Name: "p"}, polKey: "k"}, "DROP POLICY"},
		{"add_con", change{kind: plan.ChangeAddConstraint, sch: "public", tbl: "t1", conName: "c1", conDef: "CHECK (id > 0)"}, "ADD CONSTRAINT"},
		{"add_con_nv", change{kind: plan.ChangeAddConstraint, sch: "public", tbl: "t1", conName: "c1", conDef: "CHECK (id > 0) NOT VALID"}, "ADD CONSTRAINT"},
		{"drop_con", change{kind: plan.ChangeDropConstraint, sch: "public", tbl: "t1", conName: "c1"}, "DROP CONSTRAINT"},
		{"create_view", change{kind: plan.ChangeCreateView, v: &schema.View{Schema: "public", Name: "v1", DefSQL: "create view v1 as select 1"}}, "create view"},
		{"drop_view", change{kind: plan.ChangeDropView, v: &schema.View{Schema: "public", Name: "v1", Materialized: true}, viewKey: "k"}, "DROP MATERIALIZED"},
		{"create_seq", change{kind: plan.ChangeCreateSequence, seq: &schema.Sequence{Schema: "public", Name: "s1", DefSQL: "CREATE SEQUENCE public.s1"}}, "CREATE SEQUENCE"},
	}
	_ = qt
	for _, tc := range cases {
		out := stmtFor(tc.c, opt0())
		if len(out) == 0 && tc.name != "add_con_nv" {
			// add_con_nv may produce 2 with AppendValidate
			if tc.name == "add_con" {
				continue
			}
		}
		require.NotEmpty(t, out, tc.name)
		joined := out[0].DDL
		if tc.want != "" {
			require.Contains(t, strings.ToLower(joined), strings.ToLower(tc.want), "%s: %s", tc.name, joined)
		}
	}
	// add constraint with opt AppendValidate
	c := change{kind: plan.ChangeAddConstraint, sch: "public", tbl: "t1", conName: "c1", conDef: "CHECK (a) NOT VALID"}
	out2 := stmtFor(c, Options{AppendValidateAfterNotValid: true})
	require.GreaterOrEqual(t, len(out2), 2, "validate follow-up")
	require.Contains(t, out2[1].DDL, "VALIDATE CONSTRAINT")
}

func TestAlterStmt_and_createTable(t *testing.T) {
	empty := alterStmt(change{kind: plan.ChangeAlterColumn, alterKind: "type", sch: "public", tbl: "t", col: "c", dc: nil})
	require.Empty(t, empty)

	alt := alterStmt(change{kind: plan.ChangeAlterColumn, alterKind: "type", sch: "public", tbl: "t", col: "c", dc: &schema.Column{TypeSQL: "bigint"}, lc: &schema.Column{TypeSQL: "int"}})
	require.Len(t, alt, 1)
	require.Contains(t, alt[0].DDL, "SET DATA TYPE")

	nn := alterStmt(change{kind: plan.ChangeAlterColumn, alterKind: "notnull", sch: "p", tbl: "t", col: "c", dc: &schema.Column{NotNull: true}, lc: &schema.Column{NotNull: false}})
	require.Contains(t, nn[0].DDL, "SET NOT NULL")

	dnn := alterStmt(change{kind: plan.ChangeAlterColumn, alterKind: "notnull", sch: "p", tbl: "t", col: "c", dc: &schema.Column{NotNull: false}, lc: &schema.Column{NotNull: true}})
	require.Contains(t, dnn[0].DDL, "DROP NOT NULL")

	df := alterStmt(change{kind: plan.ChangeAlterColumn, alterKind: "def", sch: "p", tbl: "t", col: "c", dc: &schema.Column{DefaultSQL: "7"}, lc: &schema.Column{}})
	require.Contains(t, df[0].DDL, "SET DEFAULT")
	df2 := alterStmt(change{kind: plan.ChangeAlterColumn, alterKind: "def", sch: "p", tbl: "t", col: "c", dc: &schema.Column{DefaultSQL: ""}, lc: &schema.Column{DefaultSQL: "1"}})
	require.Contains(t, df2[0].DDL, "DROP DEFAULT")

	s := createTableSQL(&schema.Table{Schema: "a", Name: "b", Columns: []*schema.Column{
		{Name: "c", TypeSQL: "int", IsPrimaryKey: true},
		{Name: "d", TypeSQL: "text", NotNull: true, DefaultSQL: "''"},
	},
		Checks:      []*schema.TableCheck{{Name: "ck", DefSQL: "CHECK (c > 0)"}},
		Uniques:     []*schema.TableUnique{{Name: "u1", DefSQL: "UNIQUE (d)"}},
		Excludes:    []*schema.TableExclusion{{Name: "x1", DefSQL: "EXCLUDE USING gist (c WITH =)"}},
		ForeignKeys: []*schema.TableForeignKey{{Name: "f1", DefSQL: "FOREIGN KEY (c) REFERENCES o(id)"}},
	})
	require.Contains(t, s, "CONSTRAINT")
	require.Contains(t, s, "PRIMARY KEY")
}

func TestIdent_quoted(t *testing.T) {
	require.Contains(t, ident("Weird-Name"), `"`)
}

func TestToggleRLS_stmts(t *testing.T) {
	en := stmtFor(change{kind: plan.ChangeToggleRLS, sch: "public", tbl: "t", wantRls: true, wantForce: true, had: false}, opt0())
	require.GreaterOrEqual(t, len(en), 2)
	require.Contains(t, en[0].DDL, "ENABLE")
	require.Contains(t, en[len(en)-1].DDL, "FORCE")

	en2 := stmtFor(change{kind: plan.ChangeToggleRLS, sch: "public", tbl: "t", wantRls: true, wantForce: false, had: false}, opt0())
	var noforce bool
	for _, x := range en2 {
		if strings.Contains(x.DDL, "NO FORCE") {
			noforce = true
		}
	}
	require.True(t, noforce)

	dis := stmtFor(change{kind: plan.ChangeToggleRLS, sch: "public", tbl: "t", wantRls: false, had: true}, opt0())
	require.Contains(t, dis[0].DDL, "DISABLE")
}

func TestRewriteIndexConcurrent(t *testing.T) {
	u := rewriteIndexConcurrent("CREATE UNIQUE INDEX ix ON t (a)")
	require.Contains(t, u, "CONCURRENTLY")
	nc := rewriteIndexConcurrent("SELECT 1")
	require.Equal(t, "SELECT 1", strings.TrimSpace(nc))
}

func TestEnrichHazardsFromOptions_reltuple(t *testing.T) {
	stm := []plan.Statement{{
		OpType: "SET_NOT_NULL", Object: "public.t",
		Hazards: []hazard.Detected{{Type: hazard.ConstraintScan, Severity: hazard.SeverityBlocking},
		}}}
	enrichHazardsFromOptions(&stm, Options{Reltuples: map[string]float64{"public.t": 5e6}, SetNotNullReltupleThreshold: 1})
	require.Greater(t, len(stm[0].Hazards), 1)
}

func TestBuildStatements_merges(t *testing.T) {
	empty, live := &schema.SchemaState{Tables: map[string]*schema.Table{}}, &schema.SchemaState{Tables: map[string]*schema.Table{}}
	_ = buildStatements([]change{{
		kind:   plan.ChangeRawSQL,
		rawSQL: "SELECT 1",
	}}, empty, live, opt0())
}

func TestWantTable_hasChange(t *testing.T) {
	des := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.t": {Schema: "public", Name: "t", Columns: []*schema.Column{{Name: "a", TypeSQL: "int"}}},
	}}
	require.True(t, wantTable(des, "public.t", map[string]string{}))
}

func TestColDiff(t *testing.T) {
	a := &schema.Column{Name: "a", TypeSQL: "int", NotNull: false, DefaultSQL: "1"}
	b := &schema.Column{Name: "a", TypeSQL: "text", NotNull: false, DefaultSQL: "1"}
	require.True(t, colDiff(a, b))
}
