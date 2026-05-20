package differ

import (
	"strings"
	"testing"

	"github.com/nexg/pg-flux/pkg/plan"
	"github.com/nexg/pg-flux/pkg/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Fix #1: when rename hint references a column that no longer exists in live
// (because the rename was already applied), subsequent column-level diffs must
// still be detected. Previously the differ silently dropped them.
func TestRenameHintPersistence_detectsLaterColumnChanges(t *testing.T) {
	des := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.users": {Schema: "public", Name: "users", Columns: []*schema.Column{
			{Name: "handle", TypeSQL: "text", RenameFrom: "username", NotNull: true, DefaultSQL: "'unknown'"},
		}},
	}}
	// Rename already applied: live has "handle", no "username".
	live := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.users": {Schema: "public", Name: "users", Columns: []*schema.Column{
			{Name: "handle", TypeSQL: "text", NotNull: false, DefaultSQL: ""},
		}},
	}}
	dr, err := Diff(des, live, Options{})
	require.NoError(t, err)
	var sawSetNotNull, sawSetDefault bool
	for _, s := range dr.Plan.Statements {
		if s.OpType == "SET_NOT_NULL" {
			sawSetNotNull = true
		}
		if s.OpType == "ALTER_DEFAULT" && strings.Contains(s.DDL, "SET DEFAULT") {
			sawSetDefault = true
		}
	}
	assert.True(t, sawSetNotNull, "expected SET NOT NULL on the renamed column")
	assert.True(t, sawSetDefault, "expected SET DEFAULT on the renamed column")
}

// Fix #2: ALTER COLUMN TYPE must not silently lose the column's DEFAULT.
// When defs match (both desired and live have DEFAULT 0), the engine drops the
// default to allow the type change — it must restore it afterwards.
func TestAlterColumnType_restoresDefault(t *testing.T) {
	des := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.t": {Schema: "public", Name: "t", Columns: []*schema.Column{
			{Name: "n", TypeSQL: "bigint", DefaultSQL: "0"},
		}},
	}}
	live := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.t": {Schema: "public", Name: "t", Columns: []*schema.Column{
			{Name: "n", TypeSQL: "integer", DefaultSQL: "0"},
		}},
	}}
	dr, err := Diff(des, live, Options{})
	require.NoError(t, err)
	var dropDef, setType, setDef int
	for _, s := range dr.Plan.Statements {
		switch {
		case s.OpType == "ALTER_DEFAULT" && strings.Contains(s.DDL, "DROP DEFAULT"):
			dropDef++
		case s.OpType == "ALTER_COLUMN_TYPE":
			setType++
		case s.OpType == "ALTER_DEFAULT" && strings.Contains(s.DDL, "SET DEFAULT 0"):
			setDef++
		}
	}
	assert.Equal(t, 1, dropDef, "expected one DROP DEFAULT")
	assert.Equal(t, 1, setType, "expected one SET DATA TYPE")
	assert.Equal(t, 1, setDef, "expected one SET DEFAULT to restore")
}

// Fix #3: ALTER SEQUENCE must include START WITH so seqstart converges. Without
// it, repeated drift runs keep emitting the same ALTER.
func TestAlterSequenceIncludesStart(t *testing.T) {
	desired := &schema.Sequence{
		Schema: "public", Name: "s",
		DefSQL: "CREATE SEQUENCE public.s START 100 INCREMENT 5 CACHE 10",
	}
	got := buildAlterSequenceSQL(desired)
	assert.Contains(t, got, "START WITH 100")
	assert.Contains(t, got, "INCREMENT BY 5")
	assert.Contains(t, got, "CACHE 10")
}

// Fix #5: ADD CONSTRAINT for CHECK/FK with --auto-not-valid (default) should
// rewrite to NOT VALID + follow-up VALIDATE CONSTRAINT.
func TestAutoNotValid_checkConstraint(t *testing.T) {
	c := change{
		kind: plan.ChangeAddConstraint,
		sch:  "public", tbl: "t",
		conName: "t_pos", conKind: "c",
		conDef: "CHECK (n >= 0)",
	}
	stmts := stmtFor(c, Options{AutoConstraintNotValid: true})
	require.Len(t, stmts, 2)
	assert.Contains(t, stmts[0].DDL, "ADD CONSTRAINT t_pos CHECK (n >= 0) NOT VALID")
	assert.Equal(t, "VALIDATE_TABLE_CONSTRAINT", stmts[1].OpType)
	assert.Contains(t, stmts[1].DDL, "VALIDATE CONSTRAINT t_pos")
}

func TestAutoNotValid_disabled_emitsPlainAdd(t *testing.T) {
	c := change{
		kind: plan.ChangeAddConstraint,
		sch:  "public", tbl: "t",
		conName: "t_pos", conKind: "c",
		conDef: "CHECK (n >= 0)",
	}
	stmts := stmtFor(c, Options{AutoConstraintNotValid: false})
	require.Len(t, stmts, 1)
	assert.NotContains(t, stmts[0].DDL, "NOT VALID")
}

// Fix #5: ADD CONSTRAINT for FOREIGN KEY with --auto-not-valid (default) should
// rewrite to NOT VALID + follow-up VALIDATE CONSTRAINT, same as CHECK.
func TestAutoNotValid_foreignKey(t *testing.T) {
	c := change{
		kind: plan.ChangeAddConstraint,
		sch:  "public", tbl: "orders",
		conName: "orders_user_fk", conKind: "f",
		conDef: "FOREIGN KEY (user_id) REFERENCES public.users(id)",
	}
	stmts := stmtFor(c, Options{AutoConstraintNotValid: true})
	require.Len(t, stmts, 2)
	assert.Contains(t, stmts[0].DDL, "ADD CONSTRAINT orders_user_fk FOREIGN KEY")
	assert.Contains(t, stmts[0].DDL, "NOT VALID")
	assert.Equal(t, "VALIDATE_TABLE_CONSTRAINT", stmts[1].OpType)
	assert.Contains(t, stmts[1].DDL, "VALIDATE CONSTRAINT orders_user_fk")
	assert.True(t, stmts[1].IsConcurrent, "VALIDATE CONSTRAINT must run outside main txn")
}

// Fix #5: UNIQUE / PRIMARY KEY constraints are not eligible for NOT VALID rewrite —
// the catalog does not support NOT VALID on unique/PK in PostgreSQL.
func TestAutoNotValid_doesNotApplyToUnique(t *testing.T) {
	c := change{
		kind: plan.ChangeAddConstraint,
		sch:  "public", tbl: "t",
		conName: "t_uq", conKind: "u",
		conDef: "UNIQUE (email)",
	}
	stmts := stmtFor(c, Options{AutoConstraintNotValid: true})
	require.Len(t, stmts, 1)
	assert.NotContains(t, stmts[0].DDL, "NOT VALID")
}

// Fix #7: when only USING / WITH CHECK / role list of a policy differ, emit
// ALTER POLICY rather than DROP+CREATE (closes the RLS-window security gap).
func TestPolicyChange_emitsAlterPolicy(t *testing.T) {
	des := &schema.SchemaState{Policies: map[string]*schema.Policy{
		"public.users/p1": {
			Schema: "public", Table: "users", Name: "p1",
			Cmd: "SELECT", Permissive: true, Roles: []string{"public"},
			UsingSQL: "id = current_setting('app.user_id', true)::bigint",
			DefSQL:   "CREATE POLICY p1 ON public.users FOR SELECT TO public USING (id = current_setting('app.user_id', true)::bigint)",
		},
	}}
	live := &schema.SchemaState{Policies: map[string]*schema.Policy{
		"public.users/p1": {
			Schema: "public", Table: "users", Name: "p1",
			Cmd: "SELECT", Permissive: true, Roles: []string{"public"},
			UsingSQL: "true",
			DefSQL:   "CREATE POLICY p1 ON public.users FOR SELECT TO public USING (true)",
		},
	}}
	dr, err := Diff(des, live, Options{})
	require.NoError(t, err)
	var sawAlter, sawDropCreate bool
	for _, s := range dr.Plan.Statements {
		if s.OpType == string(plan.ChangeAlterPolicy) {
			sawAlter = true
			assert.Contains(t, s.DDL, "ALTER POLICY p1 ON public.users")
			assert.Contains(t, s.DDL, "USING (")
		}
		if s.OpType == string(plan.ChangeDropPolicy) || s.OpType == string(plan.ChangeCreatePolicy) {
			sawDropCreate = true
		}
	}
	assert.True(t, sawAlter, "expected ALTER POLICY")
	assert.False(t, sawDropCreate, "expected no DROP/CREATE POLICY when only USING differs")
}

// Fix #7: Cmd change forces DROP+CREATE — ALTER POLICY cannot change FOR ...
func TestPolicyCmdChange_fallsBackToDropCreate(t *testing.T) {
	des := &schema.SchemaState{Policies: map[string]*schema.Policy{
		"public.users/p1": {
			Schema: "public", Table: "users", Name: "p1",
			Cmd: "UPDATE", Permissive: true,
			DefSQL: "CREATE POLICY p1 ON public.users FOR UPDATE USING (true)",
		},
	}}
	live := &schema.SchemaState{Policies: map[string]*schema.Policy{
		"public.users/p1": {
			Schema: "public", Table: "users", Name: "p1",
			Cmd: "SELECT", Permissive: true,
			DefSQL: "CREATE POLICY p1 ON public.users FOR SELECT USING (true)",
		},
	}}
	dr, err := Diff(des, live, Options{})
	require.NoError(t, err)
	var drops, creates, alters int
	for _, s := range dr.Plan.Statements {
		switch s.OpType {
		case string(plan.ChangeDropPolicy):
			drops++
		case string(plan.ChangeCreatePolicy):
			creates++
		case string(plan.ChangeAlterPolicy):
			alters++
		}
	}
	assert.Equal(t, 1, drops)
	assert.Equal(t, 1, creates)
	assert.Equal(t, 0, alters)
}

// Fix #8: when a constraint has been renamed in source but the underlying
// definition is unchanged (after applying any column renames), emit a single
// RENAME CONSTRAINT instead of DROP + ADD.
func TestConstraintRename_collapsesDropAdd(t *testing.T) {
	dt := &schema.Table{
		Schema: "public", Name: "users",
		Uniques: []*schema.TableUnique{
			{Name: "users_handle_unique", DefSQL: "UNIQUE (handle)"},
		},
	}
	lt := &schema.Table{
		Schema: "public", Name: "users",
		Uniques: []*schema.TableUnique{
			{Name: "users_username_unique", DefSQL: "UNIQUE (handle)"},
		},
	}
	out := diffTableConstraints(dt, lt, nil)
	var sawRename bool
	for _, c := range out {
		if c.kind == plan.ChangeRenameConstraint {
			sawRename = true
			assert.Equal(t, "users_username_unique", c.from)
			assert.Equal(t, "users_handle_unique", c.conName)
		}
		assert.NotEqual(t, plan.ChangeDropConstraint, c.kind, "should not DROP when only the name changed")
		assert.NotEqual(t, plan.ChangeAddConstraint, c.kind, "should not ADD when only the name changed")
	}
	assert.True(t, sawRename, "expected RENAME CONSTRAINT")
}

// Fix #10: PG inserts type-coercion casts like ::character varying into pg_get_expr
// output for generated columns whose inputs are different types. The expression
// fingerprint must canonicalize these so the differ does not see a spurious change.
func TestNormExpr_stripsMultiWordTypeCasts(t *testing.T) {
	desired := "upper(coalesce(full_name, handle))"
	// Live form from PG (full_name=varchar, handle=text) — note multi-word cast.
	live := "upper(COALESCE(full_name, handle::character varying)::text)"
	assert.Equal(t, normExprForCompare(desired), normExprForCompare(live),
		"normExprForCompare must collapse ::character varying and ::text casts")
}
