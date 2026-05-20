package differ

// Issue coverage matrix — unit-level regression tests for Issues 5-8.
// Issue 9 (shadow DB) has its own package-level tests in pkg/shadow.

import (
	"strings"
	"testing"

	"github.com/nexg/pg-flux/pkg/plan"
	"github.com/nexg/pg-flux/pkg/schema"
	"github.com/stretchr/testify/require"
)

// ─── Issue 5: Default value drift ─────────────────────────────────────────────

// TestNormExprForCompare_ATTimeZone verifies that AT TIME ZONE expressions are
// normalised identically whether they come from the desired schema (pg_query
// canonical) or from the live catalog (pg_catalog.timezone()).
func TestNormExprForCompare_ATTimeZone(t *testing.T) {
	// The desired side is deparser output; the live side is what pg_get_expr returns.
	desired := `timezone('UTC', now())`
	live := `pg_catalog.timezone('utc'::text, now())`
	nd := normExprForCompare(desired)
	nl := normExprForCompare(live)
	require.Equal(t, nd, nl, "AT TIME ZONE should normalise to the same value on both sides")
}

// TestNormExprForCompare_PGCatalogPrefix verifies that pg_catalog. prefix stripping
// does not falsely mutate a plain non-qualified expression.
func TestNormExprForCompare_PGCatalogPrefix(t *testing.T) {
	s := `lower(email)`
	require.Equal(t, normExprForCompare(s), normExprForCompare(s))
	// pg_catalog.lower and lower should be considered equal.
	require.Equal(t, normExprForCompare(`lower(x)`), normExprForCompare(`pg_catalog.lower(x)`))
}

// ─── Issue 7: IF NOT EXISTS guards ────────────────────────────────────────────

// TestDiff_AddColumn_IfNotExists checks that ADD COLUMN generates the IF NOT EXISTS guard.
func TestDiff_AddColumn_IfNotExists(t *testing.T) {
	des := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.t": {Schema: "public", Name: "t", Columns: []*schema.Column{
			{Name: "id", TypeSQL: "int"},
			{Name: "newcol", TypeSQL: "text"},
		}},
	}}
	live := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.t": {Schema: "public", Name: "t", Columns: []*schema.Column{
			{Name: "id", TypeSQL: "int"},
		}},
	}}
	dr, err := Diff(des, live, Options{})
	require.NoError(t, err)
	var ddl string
	for _, s := range dr.Plan.Statements {
		if s.OpType == string(plan.ChangeAddColumn) {
			ddl = s.DDL
		}
	}
	require.NotEmpty(t, ddl, "expected ADD COLUMN statement")
	require.Contains(t, strings.ToUpper(ddl), "IF NOT EXISTS",
		"ADD COLUMN must use IF NOT EXISTS guard")
}

// TestRewriteIndexConcurrent_IfNotExists checks that CREATE INDEX CONCURRENTLY
// acquires the IF NOT EXISTS guard via ensureIndexIfNotExists.
func TestRewriteIndexConcurrent_IfNotExists(t *testing.T) {
	in := "CREATE INDEX my_idx ON public.t (col)"
	out := rewriteIndexConcurrent(in)
	require.Contains(t, strings.ToUpper(out), "IF NOT EXISTS",
		"concurrent-rewritten index DDL must contain IF NOT EXISTS")
	require.Contains(t, strings.ToUpper(out), "CONCURRENTLY",
		"concurrent-rewritten index DDL must contain CONCURRENTLY")
}

// TestEnsureIndexIfNotExists_idempotent checks that already-present IF NOT EXISTS
// is not duplicated.
func TestEnsureIndexIfNotExists_idempotent(t *testing.T) {
	in := "CREATE INDEX IF NOT EXISTS my_idx ON public.t (col)"
	out := ensureIndexIfNotExists(in)
	count := strings.Count(strings.ToUpper(out), "IF NOT EXISTS")
	require.Equal(t, 1, count, "IF NOT EXISTS must appear exactly once")
}

// TestEnsureIndexIfNotExists_unique checks that UNIQUE INDEX also gets the guard.
func TestEnsureIndexIfNotExists_unique(t *testing.T) {
	in := "CREATE UNIQUE INDEX my_idx ON public.t (col)"
	out := ensureIndexIfNotExists(in)
	require.Contains(t, strings.ToUpper(out), "IF NOT EXISTS")
	require.Contains(t, strings.ToUpper(out), "UNIQUE")
}

// ─── Issue 8: ALTER DOMAIN constraint diffing ─────────────────────────────────

// TestDiffDomains_addConstraint checks that a new constraint in desired emits ADD CONSTRAINT.
func TestDiffDomains_addConstraint(t *testing.T) {
	des := &schema.SchemaState{Domains: map[string]*schema.Domain{
		"public.mydom": {Schema: "public", Name: "mydom", BaseType: "text",
			Constraints: []schema.DomainConstraint{{Name: "chk_nonempty", Expr: "VALUE <> ''"}}},
	}}
	live := &schema.SchemaState{Domains: map[string]*schema.Domain{
		"public.mydom": {Schema: "public", Name: "mydom", BaseType: "text",
			Constraints: []schema.DomainConstraint{}},
	}}
	ch := diffDomains(des, live)
	require.Len(t, ch, 1)
	require.Contains(t, ch[0].rawSQL, "ADD CONSTRAINT")
	require.Contains(t, ch[0].rawSQL, "chk_nonempty")
}

// TestDiffDomains_dropConstraint checks that a removed constraint emits DROP CONSTRAINT.
func TestDiffDomains_dropConstraint(t *testing.T) {
	des := &schema.SchemaState{Domains: map[string]*schema.Domain{
		"public.mydom": {Schema: "public", Name: "mydom", BaseType: "text",
			Constraints: []schema.DomainConstraint{}},
	}}
	live := &schema.SchemaState{Domains: map[string]*schema.Domain{
		"public.mydom": {Schema: "public", Name: "mydom", BaseType: "text",
			Constraints: []schema.DomainConstraint{{Name: "old_chk", Expr: "VALUE IS NOT NULL"}}},
	}}
	ch := diffDomains(des, live)
	require.Len(t, ch, 1)
	require.Contains(t, ch[0].rawSQL, "DROP CONSTRAINT IF EXISTS")
	require.Contains(t, ch[0].rawSQL, "old_chk")
}

// TestDiffDomains_noChange verifies no DDL is emitted when desired == live.
func TestDiffDomains_noChange(t *testing.T) {
	expr := "value like '%@%'"
	des := &schema.SchemaState{Domains: map[string]*schema.Domain{
		"public.email": {Schema: "public", Name: "email", BaseType: "text",
			Constraints: []schema.DomainConstraint{{Name: "chk_at", Expr: expr}}},
	}}
	live := &schema.SchemaState{Domains: map[string]*schema.Domain{
		"public.email": {Schema: "public", Name: "email", BaseType: "text",
			Constraints: []schema.DomainConstraint{{Name: "chk_at", Expr: expr}}},
	}}
	ch := diffDomains(des, live)
	require.Empty(t, ch)
}

// TestDiffDomains_likeVsTilde checks that the ~~ → LIKE normalisation prevents
// spurious constraint re-adds (the live catalog stores ~~ instead of LIKE).
func TestDiffDomains_likeVsTilde(t *testing.T) {
	// desired side comes from deparser: "LIKE" keyword.
	// live side comes from pg_get_constraintdef: "~~" operator.
	des := &schema.SchemaState{Domains: map[string]*schema.Domain{
		"public.email": {Schema: "public", Name: "email", BaseType: "text",
			Constraints: []schema.DomainConstraint{{Name: "chk_at", Expr: "value like '%@%'"}}},
	}}
	live := &schema.SchemaState{Domains: map[string]*schema.Domain{
		"public.email": {Schema: "public", Name: "email", BaseType: "text",
			Constraints: []schema.DomainConstraint{{Name: "chk_at", Expr: "value ~~ '%@%'"}}},
	}}
	ch := diffDomains(des, live)
	require.Empty(t, ch, "~~ and LIKE must compare equal to avoid false churn")
}

// TestDiffDomains_newDomain checks that a new desired domain (not in live) emits no
// domain-level changes from diffDomains (domain creation is handled by ExtraDDL).
func TestDiffDomains_newDomain(t *testing.T) {
	des := &schema.SchemaState{Domains: map[string]*schema.Domain{
		"public.newdom": {Schema: "public", Name: "newdom", BaseType: "int",
			Constraints: []schema.DomainConstraint{{Name: "chk_pos", Expr: "value > 0"}}},
	}}
	live := &schema.SchemaState{Domains: map[string]*schema.Domain{}}
	// diffDomains should not error and should emit no domain-level changes
	// (CREATE DOMAIN is handled by ExtraDDL).
	ch := diffDomains(des, live)
	require.Empty(t, ch)
}

// TestDiffDomains_nilDesired and nil live should both return empty slices.
func TestDiffDomains_nilBothSides(t *testing.T) {
	require.Empty(t, diffDomains(nil, nil))
	require.Empty(t, diffDomains(&schema.SchemaState{}, nil))
	require.Empty(t, diffDomains(nil, &schema.SchemaState{}))
}

// ─── Issue 5: normalizePGCatalogConstraintText ─────────────────────────────────

// TestNormalizePGCatalog_tildeToLike ensures ~~ is converted to like.
func TestNormalizePGCatalog_tildeToLike(t *testing.T) {
	require.Equal(t, "value like '%@%'", normalizePGCatalogConstraintText("value ~~ '%@%'"))
}

// TestNormalizePGCatalog_notTildeToNotLike ensures !~~ is converted to not like.
func TestNormalizePGCatalog_notTildeToNotLike(t *testing.T) {
	require.Equal(t, "value not like '% %'", normalizePGCatalogConstraintText("value !~~ '% %'"))
}

// TestNormalizePGCatalog_betweenExpansion ensures BETWEEN is expanded to >= AND <=.
func TestNormalizePGCatalog_betweenExpansion(t *testing.T) {
	out := normalizePGCatalogConstraintText("length(value) between 3 and 254")
	require.Contains(t, out, ">=")
	require.Contains(t, out, "<=")
	require.NotContains(t, strings.ToUpper(out), "BETWEEN")
}
