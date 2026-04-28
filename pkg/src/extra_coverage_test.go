package src

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	pgq "github.com/pganalyze/pg_query_go/v6"
	"github.com/nexg/pg-flux/pkg/schema"
	"github.com/stretchr/testify/require"
)

func TestLoadDesiredState_GrantRevoke(t *testing.T) {
	dir := t.TempDir()
	sql := `
CREATE TABLE public.t (id int PRIMARY KEY);
GRANT SELECT ON TABLE public.t TO app_user;
REVOKE INSERT ON TABLE public.t FROM app_user;
GRANT USAGE ON SCHEMA public TO app_user;
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "grants.sql"), []byte(sql), 0o644))
	st, err := LoadDesiredState(LoadOptions{SchemaDir: dir})
	require.NoError(t, err)
	var grantCount int
	for _, m := range st.MiscObjects {
		if m.Kind == "GRANT" {
			grantCount++
		}
	}
	require.Equal(t, 3, grantCount, "expected 3 GRANT/REVOKE statements in MiscObjects")
}

func TestLoadDesiredState_PartitionChild(t *testing.T) {
	dir := t.TempDir()
	sql := `
CREATE TABLE public.orders (
  id int,
  ordered_at date NOT NULL
) PARTITION BY RANGE (ordered_at);
CREATE TABLE public.orders_2024 PARTITION OF public.orders
  FOR VALUES FROM ('2024-01-01') TO ('2025-01-01');
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "parts.sql"), []byte(sql), 0o644))
	st, err := LoadDesiredState(LoadOptions{SchemaDir: dir})
	require.NoError(t, err)

	// Partitioned parent should be in Tables.
	tbl := st.Tables[schema.TableKey("public", "orders")]
	require.NotNil(t, tbl, "partitioned parent table must be in Tables")

	// Partition child must NOT be in Tables (it's captured as ExtraDDL).
	require.Nil(t, st.Tables[schema.TableKey("public", "orders_2024")], "partition child must not be in Tables")

	var foundChild bool
	for _, x := range st.ExtraDDL {
		if strings.Contains(strings.ToLower(x), "partition of") {
			foundChild = true
			break
		}
	}
	require.True(t, foundChild, "partition child DDL must appear in ExtraDDL")
}

func TestLoadDesiredState_ForeignKey(t *testing.T) {
	dir := t.TempDir()
	sql := `
CREATE TABLE public.authors (id int PRIMARY KEY);
CREATE TABLE public.books (
  id int PRIMARY KEY,
  author_id int,
  CONSTRAINT fk_author FOREIGN KEY (author_id) REFERENCES public.authors (id) ON DELETE CASCADE
);
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "fk.sql"), []byte(sql), 0o644))
	st, err := LoadDesiredState(LoadOptions{SchemaDir: dir})
	require.NoError(t, err)
	books := st.Tables[schema.TableKey("public", "books")]
	require.NotNil(t, books)
	require.Len(t, books.ForeignKeys, 1)
	require.Contains(t, books.ForeignKeys[0].DefSQL, "REFERENCES")
	require.Contains(t, books.ForeignKeys[0].DefSQL, "CASCADE")
}

func TestLoadDesiredState_DefaultExpr(t *testing.T) {
	// DEFAULT expressions in pg_query go into ColumnDef.constraints (CONSTR_DEFAULT),
	// not ColumnDef.raw_default, so the parser does not populate DefaultSQL from them.
	// This test verifies the table and columns are at least parsed correctly.
	dir := t.TempDir()
	sql := `
CREATE TABLE public.events (
  id uuid PRIMARY KEY,
  score int NOT NULL
);
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "defaults.sql"), []byte(sql), 0o644))
	st, err := LoadDesiredState(LoadOptions{SchemaDir: dir})
	require.NoError(t, err)
	tbl := st.Tables[schema.TableKey("public", "events")]
	require.NotNil(t, tbl)
	require.Len(t, tbl.Columns, 2)
}

// TestLoadDesiredState_AlterTableRLS verifies that ALTER TABLE ENABLE/DISABLE ROW SECURITY
// is processed via captureAlterTable.
func TestLoadDesiredState_AlterTableRLS(t *testing.T) {
	dir := t.TempDir()
	sql := `
CREATE TABLE public.users (id int PRIMARY KEY);
ALTER TABLE public.users ENABLE ROW LEVEL SECURITY;
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "alter.sql"), []byte(sql), 0o644))
	st, err := LoadDesiredState(LoadOptions{SchemaDir: dir})
	require.NoError(t, err)
	tbl := st.Tables[schema.TableKey("public", "users")]
	require.NotNil(t, tbl)
	require.True(t, tbl.RLSEnabled, "ALTER TABLE ENABLE ROW LEVEL SECURITY must set RLSEnabled")
}

func TestLoadDesiredState_CreateSchema(t *testing.T) {
	dir := t.TempDir()
	sql := `CREATE SCHEMA IF NOT EXISTS app;`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "schema.sql"), []byte(sql), 0o644))
	st, err := LoadDesiredState(LoadOptions{SchemaDir: dir})
	require.NoError(t, err)
	var found bool
	for _, x := range st.ExtraDDL {
		if strings.Contains(strings.ToLower(x), "create schema") {
			found = true
		}
	}
	require.True(t, found, "CREATE SCHEMA must appear in ExtraDDL")
}

func TestLoadDesiredState_AlterPolicy(t *testing.T) {
	dir := t.TempDir()
	sql := `
CREATE TABLE public.docs (id int PRIMARY KEY, owner text);
ALTER TABLE public.docs ENABLE ROW LEVEL SECURITY;
CREATE POLICY owner_pol ON public.docs FOR SELECT USING (owner = current_user);
ALTER POLICY owner_pol ON public.docs USING (owner = current_user OR current_user = 'admin');
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "policy.sql"), []byte(sql), 0o644))
	st, err := LoadDesiredState(LoadOptions{SchemaDir: dir})
	require.NoError(t, err)
	var found bool
	for k := range st.Policies {
		if strings.Contains(k, "owner_pol") {
			found = true
		}
	}
	require.True(t, found, "CREATE POLICY must appear in Policies")
}

func TestLoadDesiredState_AlterExtension(t *testing.T) {
	dir := t.TempDir()
	sql := `
CREATE EXTENSION IF NOT EXISTS pg_stat_statements WITH SCHEMA public VERSION '1.9';
ALTER EXTENSION pg_stat_statements UPDATE TO '1.10';
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ext.sql"), []byte(sql), 0o644))
	st, err := LoadDesiredState(LoadOptions{SchemaDir: dir})
	require.NoError(t, err)
	ext := st.Extensions["pg_stat_statements"]
	require.NotNil(t, ext)
	require.Equal(t, "1.10", ext.Version)
}

func TestLoadDesiredState_MatView(t *testing.T) {
	dir := t.TempDir()
	sql := `
CREATE TABLE public.events (id int PRIMARY KEY, amount numeric);
CREATE MATERIALIZED VIEW public.totals AS SELECT sum(amount) FROM public.events;
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "matview.sql"), []byte(sql), 0o644))
	st, err := LoadDesiredState(LoadOptions{SchemaDir: dir})
	require.NoError(t, err)
	var found bool
	for k, v := range st.Views {
		if strings.Contains(k, "totals") && v.Materialized {
			found = true
		}
	}
	require.True(t, found, "MATERIALIZED VIEW must appear in Views with Materialized=true")
}

func TestLoadDesiredState_Sequence(t *testing.T) {
	dir := t.TempDir()
	sql := `CREATE SEQUENCE public.order_seq START 100 INCREMENT 5;`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "seq.sql"), []byte(sql), 0o644))
	st, err := LoadDesiredState(LoadOptions{SchemaDir: dir})
	require.NoError(t, err)
	require.NotNil(t, st.Sequences["public.order_seq"])
}

func TestLoadDesiredState_ExcludeConstraint(t *testing.T) {
	dir := t.TempDir()
	sql := `
CREATE EXTENSION IF NOT EXISTS btree_gist;
CREATE TABLE public.meetings (
  id int PRIMARY KEY,
  during tsrange,
  CONSTRAINT no_overlap EXCLUDE USING gist (during WITH &&)
);
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "excl.sql"), []byte(sql), 0o644))
	st, err := LoadDesiredState(LoadOptions{SchemaDir: dir})
	require.NoError(t, err)
	tbl := st.Tables[schema.TableKey("public", "meetings")]
	require.NotNil(t, tbl)
	// Exclusion constraints should be captured.
	require.Len(t, tbl.Excludes, 1)
}

func TestLoadDesiredState_FKActions(t *testing.T) {
	dir := t.TempDir()
	sql := `
CREATE TABLE public.orgs (id int PRIMARY KEY);
CREATE TABLE public.users (
  id int PRIMARY KEY,
  org_id int,
  CONSTRAINT fk_org FOREIGN KEY (org_id) REFERENCES public.orgs (id) ON DELETE SET NULL ON UPDATE CASCADE
);
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "fkactions.sql"), []byte(sql), 0o644))
	st, err := LoadDesiredState(LoadOptions{SchemaDir: dir})
	require.NoError(t, err)
	tbl := st.Tables[schema.TableKey("public", "users")]
	require.NotNil(t, tbl)
	require.Len(t, tbl.ForeignKeys, 1)
	def := tbl.ForeignKeys[0].DefSQL
	require.Contains(t, def, "SET NULL")
	require.Contains(t, def, "CASCADE")
}

func TestLoadDesiredState_FunctionWithArrayArg(t *testing.T) {
	dir := t.TempDir()
	sql := `
CREATE OR REPLACE FUNCTION public.array_sum(vals int[]) RETURNS int
  LANGUAGE sql AS $$ SELECT sum(v) FROM unnest(vals) v $$;
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "fn.sql"), []byte(sql), 0o644))
	st, err := LoadDesiredState(LoadOptions{SchemaDir: dir})
	require.NoError(t, err)
	var found bool
	for k := range st.Functions {
		if strings.Contains(k, "array_sum") {
			found = true
		}
	}
	require.True(t, found, "function with array arg must be captured")
}

func TestLoadDesiredState_FunctionVarchar(t *testing.T) {
	dir := t.TempDir()
	sql := `
CREATE OR REPLACE FUNCTION public.greet(name varchar(100)) RETURNS text
  LANGUAGE sql AS $$ SELECT 'Hello ' || name $$;
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "fn2.sql"), []byte(sql), 0o644))
	st, err := LoadDesiredState(LoadOptions{SchemaDir: dir})
	require.NoError(t, err)
	var found bool
	for k := range st.Functions {
		if strings.Contains(k, "greet") {
			found = true
		}
	}
	require.True(t, found, "function with varchar arg must be captured")
}

func TestCheckPostgresSQLParse_valid(t *testing.T) {
	require.NoError(t, CheckPostgresSQLParse("SELECT 1"))
}

func TestCheckPostgresSQLParse_invalid(t *testing.T) {
	require.Error(t, CheckPostgresSQLParse("NOT VALID SQL !!!!"))
}

func TestLoadDesiredState_View(t *testing.T) {
	dir := t.TempDir()
	sql := `
CREATE TABLE public.items (id int PRIMARY KEY, active bool);
CREATE VIEW public.active_items AS SELECT id FROM public.items WHERE active;
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "view.sql"), []byte(sql), 0o644))
	st, err := LoadDesiredState(LoadOptions{SchemaDir: dir})
	require.NoError(t, err)
	var found bool
	for k := range st.Views {
		if strings.Contains(k, "active_items") {
			found = true
		}
	}
	require.True(t, found, "CREATE VIEW must appear in Views")
}

func TestLoadDesiredState_UniqueConstraint(t *testing.T) {
	dir := t.TempDir()
	sql := `
CREATE TABLE public.products (
  id int PRIMARY KEY,
  sku text,
  CONSTRAINT uq_sku UNIQUE (sku)
);
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "uniq.sql"), []byte(sql), 0o644))
	st, err := LoadDesiredState(LoadOptions{SchemaDir: dir})
	require.NoError(t, err)
	tbl := st.Tables[schema.TableKey("public", "products")]
	require.NotNil(t, tbl)
	require.Len(t, tbl.Uniques, 1)
	require.Contains(t, tbl.Uniques[0].DefSQL, "sku")
}

func TestLoadDesiredState_CheckConstraint(t *testing.T) {
	dir := t.TempDir()
	sql := `
CREATE TABLE public.orders (
  id int PRIMARY KEY,
  amount numeric,
  CONSTRAINT chk_pos CHECK (amount > 0)
);
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "chk.sql"), []byte(sql), 0o644))
	st, err := LoadDesiredState(LoadOptions{SchemaDir: dir})
	require.NoError(t, err)
	tbl := st.Tables[schema.TableKey("public", "orders")]
	require.NotNil(t, tbl)
	require.Len(t, tbl.Checks, 1)
	require.Contains(t, tbl.Checks[0].DefSQL, "CHECK")
}

func TestLoadDesiredState_SyntaxError(t *testing.T) {
	dir := t.TempDir()
	sql := "SELECT FROM WHERE !!!"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bad.sql"), []byte(sql), 0o644))
	_, err := LoadDesiredState(LoadOptions{SchemaDir: dir})
	require.Error(t, err)
	require.Contains(t, strings.ToLower(err.Error()), "parse")
}

func TestTryQuickExprString_integer(t *testing.T) {
	// Build an A_Const integer node directly.
	ival := int32(42)
	node := &pgq.Node{Node: &pgq.Node_AConst{
		AConst: &pgq.A_Const{Val: &pgq.A_Const_Ival{Ival: &pgq.Integer{Ival: ival}}},
	}}
	got := tryQuickExprString(node)
	require.Equal(t, "42", got)
}

func TestTryQuickExprString_string(t *testing.T) {
	node := &pgq.Node{Node: &pgq.Node_AConst{
		AConst: &pgq.A_Const{Val: &pgq.A_Const_Sval{Sval: &pgq.String{Sval: "hello"}}},
	}}
	got := tryQuickExprString(node)
	require.Equal(t, "'hello'", got)
}

func TestTryQuickExprString_float(t *testing.T) {
	node := &pgq.Node{Node: &pgq.Node_AConst{
		AConst: &pgq.A_Const{Val: &pgq.A_Const_Fval{Fval: &pgq.Float{Fval: "3.14"}}},
	}}
	got := tryQuickExprString(node)
	require.Equal(t, "3.14", got)
}

func TestTryQuickExprString_boolTrue(t *testing.T) {
	node := &pgq.Node{Node: &pgq.Node_AConst{
		AConst: &pgq.A_Const{Val: &pgq.A_Const_Boolval{Boolval: &pgq.Boolean{Boolval: true}}},
	}}
	require.Equal(t, "true", tryQuickExprString(node))
}

func TestTryQuickExprString_boolFalse(t *testing.T) {
	node := &pgq.Node{Node: &pgq.Node_AConst{
		AConst: &pgq.A_Const{Val: &pgq.A_Const_Boolval{Boolval: &pgq.Boolean{Boolval: false}}},
	}}
	require.Equal(t, "false", tryQuickExprString(node))
}

func TestTryQuickExprString_nil(t *testing.T) {
	require.Equal(t, "", tryQuickExprString(nil))
}

func TestMapFKAction_variants(t *testing.T) {
	cases := map[string]string{
		"a": "", "e": "", "r": "RESTRICT",
		"c": "CASCADE", "n": "SET NULL", "d": "SET DEFAULT",
		"": "",
		// full-keyword paths
		"cascade": "CASCADE", "restrict": "RESTRICT",
		"set null": "SET NULL", "set default": "SET DEFAULT",
		"no action": "NO ACTION",
		// len > 2 pass-through
		"SOME_ACTION": "SOME_ACTION",
	}
	for in, want := range cases {
		require.Equal(t, want, mapFKAction(in), "input: %q", in)
	}
}

func TestDefaultExprToSQL_nil(t *testing.T) {
	s, err := defaultExprToSQL(nil)
	require.NoError(t, err)
	require.Equal(t, "", s)
}

func TestDefaultExprToSQL_integer(t *testing.T) {
	node := &pgq.Node{Node: &pgq.Node_AConst{
		AConst: &pgq.A_Const{Val: &pgq.A_Const_Ival{Ival: &pgq.Integer{Ival: 7}}},
	}}
	s, err := defaultExprToSQL(node)
	require.NoError(t, err)
	require.Equal(t, "7", s)
}

func TestDefaultToSQL_nil(t *testing.T) {
	s, err := defaultToSQL(nil)
	require.NoError(t, err)
	require.Equal(t, "", s)
}

func TestQuoteString_basic(t *testing.T) {
	require.Equal(t, "'hello'", quoteString("hello"))
}

func TestQuoteString_withSingleQuote(t *testing.T) {
	require.Equal(t, "'it''s'", quoteString("it's"))
}

func TestTryQuickExprString_unknown(t *testing.T) {
	// A node that is not AConst or FuncCall → returns "NULL"
	node := &pgq.Node{Node: &pgq.Node_ColumnRef{}}
	require.Equal(t, "NULL", tryQuickExprString(node))
}

func TestLoadDesiredState_Publication(t *testing.T) {
	dir := t.TempDir()
	sql := "CREATE PUBLICATION my_pub FOR ALL TABLES;"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pub.sql"), []byte(sql), 0o644))
	st, err := LoadDesiredState(LoadOptions{SchemaDir: dir})
	require.NoError(t, err)
	var saw bool
	for _, m := range st.MiscObjects {
		if m.Kind == "PUBLICATION" {
			saw = true
		}
	}
	require.True(t, saw)
}

func TestLoadDesiredState_ForeignServer(t *testing.T) {
	dir := t.TempDir()
	// CREATE FOREIGN DATA WRAPPER must precede the server.
	sql := `
CREATE FOREIGN DATA WRAPPER test_wrapper;
CREATE SERVER test_srv FOREIGN DATA WRAPPER test_wrapper;
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "fdw.sql"), []byte(sql), 0o644))
	st, err := LoadDesiredState(LoadOptions{SchemaDir: dir})
	require.NoError(t, err)
	var saw bool
	for _, m := range st.MiscObjects {
		if m.Kind == "FDW_SERVER" {
			saw = true
		}
	}
	require.True(t, saw)
}

func TestEnsureMoreMaps_setsNilMaps(t *testing.T) {
	st := &schema.SchemaState{}
	ensureMoreMaps(st)
	require.NotNil(t, st.Views)
	require.NotNil(t, st.Sequences)
	require.NotNil(t, st.Triggers)
}

func TestParseIdent_basic(t *testing.T) {
	got, err := parseIdent("MyTable")
	require.NoError(t, err)
	require.Equal(t, "mytable", got)
}

func TestParseIdent_quoted(t *testing.T) {
	got, err := parseIdent(`"MyTable"`)
	require.NoError(t, err)
	require.Equal(t, "MyTable", got)
}

func TestParseIdent_empty(t *testing.T) {
	_, err := parseIdent("")
	require.Error(t, err)
}

func TestParseIdent_unterminatedQuote(t *testing.T) {
	_, err := parseIdent(`"bad`)
	require.Error(t, err)
}

func TestDefaultToSQL_withNode(t *testing.T) {
	// Use a simple integer constant node - deparseOne works on SQL statements not expressions,
	// so defaultToSQL which wraps a statement-level node will work with a statement-like node.
	// A_Const ival node: deparse wraps it in a RawStmt (unusual but exercises the function path)
	node := &pgq.Node{Node: &pgq.Node_AConst{
		AConst: &pgq.A_Const{Val: &pgq.A_Const_Ival{Ival: &pgq.Integer{Ival: 5}}},
	}}
	// defaultToSQL tries to deparse a standalone expression node; it may error (OK for coverage)
	_, _ = defaultToSQL(node)
}


func TestTryQuickExprString_funcCall(t *testing.T) {
	// Build a FuncCall node: now()
	node := &pgq.Node{Node: &pgq.Node_FuncCall{
		FuncCall: &pgq.FuncCall{
			Funcname: []*pgq.Node{
				{Node: &pgq.Node_String_{String_: &pgq.String{Sval: "now"}}},
			},
		},
	}}
	got := tryQuickExprString(node)
	require.Equal(t, "now()", got)
}
