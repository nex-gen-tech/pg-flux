package differ

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	pgq "github.com/pganalyze/pg_query_go/v6"

	"github.com/nexg/pg-flux/pkg/plan"
	"github.com/nexg/pg-flux/pkg/schema"
)

var reMultiSpace = regexp.MustCompile(`\s+`)
var reDefaultPublicSchemaDot = regexp.MustCompile(`(?i)\bpublic\.`)

// reAnyArray matches a PostgreSQL catalog "= ANY(ARRAY[...::type, ...])" pattern produced
// by pg_get_constraintdef for IN-list CHECK constraints.
var reAnyArray = regexp.MustCompile(`= any\(array\[([^\]]+)\]\)`)

// reCastSuffix strips a trailing "::typename" cast from a quoted literal, e.g.  'pending'::text  → 'pending'.
var reCastSuffix = regexp.MustCompile(`('+[^']*'+)::[a-z][a-z0-9 ]*`)

// reTextCast strips explicit "::text" casts added by PostgreSQL when a varchar/char column is used
// in a constraint expression (e.g. email::text like '%@%' → email like '%@%').
var reTextCast = regexp.MustCompile(`::\s*text\b`)

// reAnyCast strips any PostgreSQL type cast (::typename or ::schema.typename) from constraint
// expressions. PostgreSQL adds casts like ::integer, ::bigint, ::numeric when columns of those
// types appear in pg_get_constraintdef output, while source SQL omits them.
var reAnyCast = regexp.MustCompile(`::\s*[\w.]+`)

// reBetween matches "expr BETWEEN a AND b" (simple operands: word chars, digits, or quotes).
// PostgreSQL expands BETWEEN to (col >= a) AND (col <= b) in pg_get_constraintdef,
// so we normalise source SQL to the same expanded form.
// reBetween handles simple identifiers; reBetweenFuncCall handles func(arg) style.
var reBetween = regexp.MustCompile(`(?i)\b([\w"']+)\s+between\s+([\w']+)\s+and\s+([\w']+)\b`)
var reBetweenFuncCall = regexp.MustCompile(`(?i)\b(\w+\([^)]*\))\s+between\s+(\S+)\s+and\s+(\S+)`)

// normalizePGCatalogConstraintText normalises common PostgreSQL catalog transformations so that
// source SQL and pg_get_constraintdef output round-trip to the same fingerprint:
//
//   - ~~ / !~~ / ~~* / !~~* operators → LIKE / NOT LIKE / ILIKE / NOT ILIKE
//   - "= ANY(ARRAY['a'::text, 'b'::text])" → "in ('a', 'b')" (IN-list equivalence)
//   - "x BETWEEN a AND b" → "(x >= a) and (x <= b)"  (catalog expansion)
func normalizePGCatalogConstraintText(s string) string {
	s = strings.ReplaceAll(s, " ~~ ", " like ")
	s = strings.ReplaceAll(s, " !~~ ", " not like ")
	s = strings.ReplaceAll(s, " ~~* ", " ilike ")
	s = strings.ReplaceAll(s, " !~~* ", " not ilike ")
	// Strip ALL type casts (::text, ::integer, ::bigint, ::character varying, etc.) added by
	// PostgreSQL in pg_get_constraintdef for typed columns in constraint expressions.
	// Issue 8: previously only ::text was stripped; now all ::typename casts are stripped.
	s = reAnyCast.ReplaceAllString(s, "")
	s = reAnyArray.ReplaceAllStringFunc(s, func(m string) string {
		inner := reAnyArray.FindStringSubmatch(m)[1]
		// strip trailing ::type casts from individual elements so 'x'::text → 'x'
		inner = reCastSuffix.ReplaceAllString(inner, "$1")
		// also strip any remaining casts after the reCastSuffix pass (e.g. integer literals)
		inner = reAnyCast.ReplaceAllString(inner, "")
		return "in (" + inner + ")"
	})
	// Expand "x BETWEEN a AND b" → "x >= a and x <= b" so source SQL matches catalog form.
	// Apply function-call form first (more specific), then simple identifier form.
	s = reBetweenFuncCall.ReplaceAllString(s, "$1 >= $2 and $1 <= $3")
	s = reBetween.ReplaceAllString(s, "$1 >= $2 and $1 <= $3")
	return s
}

// stripDefaultPublicSchemaQualifiers removes a redundant "public." so catalog output
// and schema files match when both refer to objects in the public schema.
func stripDefaultPublicSchemaQualifiers(s string) string {
	return reDefaultPublicSchemaDot.ReplaceAllString(s, "")
}

func ensureMaps(s *schema.SchemaState) {
	if s == nil {
		return
	}
	if s.Indexes == nil {
		s.Indexes = make(map[string]*schema.Index)
	}
	if s.Functions == nil {
		s.Functions = make(map[string]*schema.Function)
	}
	if s.Policies == nil {
		s.Policies = make(map[string]*schema.Policy)
	}
}

func diffIndexes(d, l *schema.SchemaState, tableColRenames map[string]map[string]string) []change {
	var out []change
	ensureMaps(d)
	ensureMaps(l)
	for k, di := range d.Indexes {
		if di == nil {
			continue
		}
		li := l.Indexes[k]
		// Determine whether the index's table is partitioned (CONCURRENTLY not supported).
		tblKey := schema.TableKey(di.TableSchema, di.Table)
		isPartitioned := d.Tables != nil && d.Tables[tblKey] != nil && d.Tables[tblKey].PartitionBy != ""
		if li == nil {
			out = append(out, change{kind: plan.ChangeCreateIndex, idx: di, skipConcurrent: isPartitioned})
			continue
		}
		renames := tableColRenames[tblKey]
		if !indexDefsEqualWithRenames(di, li, renames) {
			out = append(out, change{kind: plan.ChangeDropIndex, dropIdx: k, sch: li.Schema, ixName: li.Name})
			out = append(out, change{kind: plan.ChangeCreateIndex, idx: di, skipConcurrent: isPartitioned})
		}
	}
	for k, li := range l.Indexes {
		if li == nil {
			continue
		}
		if d.Indexes[k] == nil {
			out = append(out, change{kind: plan.ChangeDropIndex, dropIdx: k, sch: li.Schema, ixName: li.Name, idx: li})
		}
	}
	return out
}

func fpSQL(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	f, err := pgq.Fingerprint(s)
	if err != nil {
		return s
	}
	return f
}

// quoteSQLString quotes a single-quoted SQL string literal.
func quoteSQLString(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// quoteSQLIdent quotes a PostgreSQL identifier when quoting is required:
//   - contains non-lowercase-ASCII, non-digit, non-underscore characters
//   - starts with a digit (invalid as an unquoted identifier)
//   - is a PostgreSQL reserved keyword (RESERVED_KEYWORD classification)
func quoteSQLIdent(s string) string {
	if s == "" {
		return `""`
	}
	needQuote := false
	for i, c := range s {
		if i == 0 && c >= '0' && c <= '9' {
			// Identifiers must not start with a digit unless quoted.
			needQuote = true
			break
		}
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_') {
			needQuote = true
			break
		}
	}
	if !needQuote {
		// Check whether the word is a PostgreSQL reserved keyword, which always requires quoting
		// when used as an identifier (e.g. "order", "table", "select").
		if result, err := pgq.Scan(s); err == nil && len(result.GetTokens()) > 0 {
			if result.GetTokens()[0].GetKeywordKind() == pgq.KeywordKind_RESERVED_KEYWORD {
				needQuote = true
			}
		}
	}
	if needQuote {
		return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
	}
	return s
}

// tableConstraintDefFingerprint compares CHECK / FOREIGN KEY def texts by round-tripping
// "ALTER TABLE ... ADD CONSTRAINT name <def>" so pg_get_constraintdef and parser output agree.
func tableConstraintDefFingerprint(schema, table, conName, def string) string {
	def = strings.TrimSpace(def)
	if def == "" {
		return ""
	}
	alter := fmt.Sprintf("ALTER TABLE %s.%s ADD CONSTRAINT %s %s",
		quoteSQLIdent(schema), quoteSQLIdent(table), quoteSQLIdent(conName), def)
	dep, ok := deparseOneStmt(alter)
	if !ok {
		return fpGenericSQL(def)
	}
	dep = strings.ToLower(strings.TrimSpace(dep))
	dep = reMultiSpace.ReplaceAllString(dep, " ")
	dep = stripDefaultPublicSchemaQualifiers(dep)
	dep = normalizePGCatalogConstraintText(dep)
	return fpSQL(dep)
}
// createStmtDefFingerprint returns a canonical form of a CREATE VIEW / CREATE TRIGGER
// statement suitable for equality comparison.
// It round-trips through the pg_query deparser (for syntactic normalization),
// lowercases, collapses whitespace, strips the default "public." schema qualifier,
// and strips type casts added by pg_get_viewdef/pg_get_triggerdef (e.g. 'archived'::post_status)
// that the source SQL does not include.
// Unlike fpSQL / pgq.Fingerprint, it does NOT replace literal values with $n — a view
// whose WHERE clause changes from 'published' to 'archived' must be treated as different.
func createStmtDefFingerprint(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	dep, ok := deparseOneStmt(s)
	if !ok {
		return fpGenericSQL(s)
	}
	dep = strings.ToLower(strings.TrimSpace(dep))
	dep = reMultiSpace.ReplaceAllString(dep, " ")
	dep = stripDefaultPublicSchemaQualifiers(dep)
	// Strip type casts (::typename, ::schema.typename) added by pg_get_viewdef to match
	// the source SQL which typically omits them (e.g. 'archived'::post_status → 'archived').
	dep = reAnyCast.ReplaceAllString(dep, "")
	return strings.TrimSpace(reMultiSpace.ReplaceAllString(dep, " "))
}

func deparseOneStmt(sql string) (string, bool) {
	pr, err := pgq.Parse(sql)
	if err != nil || len(pr.GetStmts()) != 1 {
		return "", false
	}
	dep, err := pgq.Deparse(pr)
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(dep), true
}

func indexDefsEqual(di, li *schema.Index) bool {
	return indexDefsEqualWithRenames(di, li, nil)
}

func indexDefsEqualWithRenames(di, li *schema.Index, colRenames map[string]string) bool {
	if di == nil || li == nil {
		return di == li
	}
	a := indexFingerprintNormalizers(canonIndexSQL(di.CreateSQL), di.TableSchema)
	liSQL := applyColRenames(li.CreateSQL, colRenames)
	b := indexFingerprintNormalizers(canonIndexSQL(liSQL), li.TableSchema)
	return fpSQL(a) == fpSQL(b)
}

// applyColRenames substitutes renamed column identifiers in a SQL snippet using
// word-boundary matching. This prevents spurious diffs when a column is renamed:
// the live DB still has the old column name in constraint/index definitions.
func applyColRenames(s string, renames map[string]string) string {
	if len(renames) == 0 || s == "" {
		return s
	}
	for old, newName := range renames {
		re := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(old) + `\b`)
		s = re.ReplaceAllString(s, newName)
	}
	return s
}

// indexFingerprintNormalizers collapse harmless differences in CREATE INDEX.
func indexFingerprintNormalizers(s string, tableSchema string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "concurrently", "")
	// pg_get_indexdef never includes IF NOT EXISTS; strip it for comparison.
	s = strings.ReplaceAll(s, "if not exists", "")
	// pg_get_indexdef uses "ON ONLY" for indexes on partitioned table parents;
	// source SQL omits "ONLY". Normalize both to "ON".
	s = strings.ReplaceAll(s, " on only ", " on ")
	ts := strings.TrimSpace(strings.ToLower(tableSchema))
	if ts != "" {
		s = strings.ReplaceAll(s, "on "+ts+".", "on ")
	}
	s = reMultiSpace.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// canonIndexSQL re-roundtrips through pg_query parse+deparse so desired DDL and pg_get_indexdef() agree.
func canonIndexSQL(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	pr, err := pgq.Parse(s)
	if err != nil || len(pr.Stmts) != 1 {
		return s
	}
	dep, err := pgq.Deparse(pr)
	if err != nil {
		return s
	}
	return strings.TrimSpace(dep)
}

// fpIndexSQL normalizes a single def when table schema is unknown; prefer indexDefsEqual for comparisons.
func fpIndexSQL(s string) string {
	return fpSQL(indexFingerprintNormalizers(canonIndexSQL(s), "public"))
}

var reDollarDelim = regexp.MustCompile(`\$[A-Za-z0-9_]*\$`)

// fpFunctionSQL normalizes delimiters/whitespace before comparison, so deparse and catalog can match.
// NOTE: We deliberately do NOT use pgq.Fingerprint here because it normalizes string literals,
// causing different function bodies to hash the same.
// reWordBoundary builds a word-boundary regex for a SQL keyword/type name.
// Only alphabetic/underscore identifiers need this; numeric tokens are matched as-is.
var reFnTypeNorm = []*[2]string{}

func init() {
	// Table of type alias → canonical form (what pg_get_functiondef returns).
	// Word-boundary wrapped replacements are applied to the full function SQL.
	// Ordering matters: longer strings first to avoid partial matches.
	normPairs := [][2]string{
		{"timestamp with time zone", "timestamptz"},
		{"time with time zone", "timetz"},
		{"timestamp without time zone", "timestamp"},
		{"time without time zone", "time"},
		{"double precision", "float8"},
		{"character varying", "varchar"},
		{"boolean", "bool"},
		{"smallint", "int2"},
		{"bigint", "int8"},
		{"integer", "int4"},
		{"real", "float4"},
		{"int8", "int8"},   // no-op sentinels to force canonical form
		{"int4", "int4"},
		{"int2", "int2"},
		{"float4", "float4"},
		{"float8", "float8"},
		{"bool", "bool"},
		{"varchar", "varchar"},
	}
	_ = normPairs // used via fnNormType below
}

// fnNormType normalises a SQL type token in a function signature to a canonical short form.
// This mirrors what pg_get_functiondef uses so desired and live definitions compare equal.
func fnNormType(s string) string {
	switch s {
	case "integer", "int", "int4", "serial", "serial4":
		return "int4"
	case "bigint", "int8", "serial8", "bigserial":
		return "int8"
	case "smallint", "int2":
		return "int2"
	case "boolean":
		return "bool"
	case "real", "float4":
		return "float4"
	case "double precision", "float8", "float":
		return "float8"
	case "character varying", "varchar":
		return "varchar"
	case "timestamptz", "timestamp with time zone":
		return "timestamptz"
	case "timestamp without time zone", "timestamp":
		return "timestamp"
	case "timetz", "time with time zone":
		return "timetz"
	case "time without time zone", "time":
		return "time"
	case "numeric", "decimal":
		return "numeric"
	default:
		return s
	}
}

// reFnTypeTokens matches SQL type tokens in a function definition header.
// We normalise only the header (before AS $$) to avoid modifying function bodies.
var reFnTypeTokens = regexp.MustCompile(`\b(timestamp with time zone|time with time zone|timestamp without time zone|time without time zone|double precision|character varying|bigint|smallint|integer|boolean|varchar|float8|float4|float|real|int8|int4|int2|int|bool|numeric|decimal|timestamptz|timetz)\b`)

func fpFunctionSQL(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = reDollarDelim.ReplaceAllLiteralString(s, "$$")
	s = reMultiSpace.ReplaceAllString(s, " ")
	// Normalize type aliases in the function header (before the function body).
	// Split on $$ to isolate the header from the body — avoid modifying body literals.
	parts := strings.SplitN(s, "$$", 2)
	parts[0] = reFnTypeTokens.ReplaceAllStringFunc(parts[0], fnNormType)
	s = strings.Join(parts, "$$")
	return s
}

func diffFunctions(d, l *schema.SchemaState) []change {
	var out []change
	ensureMaps(d)
	ensureMaps(l)
	for k, df := range d.Functions {
		if df == nil {
			continue
		}
		lf := l.Functions[k]
		if lf == nil {
			out = append(out, change{kind: plan.ChangeCreateFunction, fn: df})
			continue
		}
		if fpFunctionSQL(df.DefSQL) != fpFunctionSQL(lf.DefSQL) {
			out = append(out, change{kind: plan.ChangeCreateFunction, fn: df})
		}
	}
	for k, lf := range l.Functions {
		if lf == nil {
			continue
		}
		if d.Functions[k] == nil {
			out = append(out, change{kind: plan.ChangeDropFunction, dropFn: k, fn: lf})
		}
	}
	return out
}

func diffPolicies(d, l *schema.SchemaState) []change {
	var out []change
	ensureMaps(d)
	ensureMaps(l)
	for k, dp := range d.Policies {
		if dp == nil {
			continue
		}
		lp := l.Policies[k]
		if lp == nil {
			out = append(out, change{kind: plan.ChangeCreatePolicy, pol: dp, polKey: k})
			continue
		}
		if !policiesEqual(dp, lp) {
			out = append(out, change{kind: plan.ChangeDropPolicy, polKey: k, pol: lp})
			out = append(out, change{kind: plan.ChangeCreatePolicy, pol: dp, polKey: k})
		}
	}
	for k, lp := range l.Policies {
		if lp == nil {
			continue
		}
		if d.Policies[k] == nil {
			out = append(out, change{kind: plan.ChangeDropPolicy, polKey: k, pol: lp})
		}
	}
	return out
}

func policiesEqual(a, b *schema.Policy) bool {
	if a == nil || b == nil {
		return a == b
	}
	if strings.TrimSpace(a.DefSQL) != "" && strings.TrimSpace(b.DefSQL) != "" {
		return fpSQL(a.DefSQL) == fpSQL(b.DefSQL)
	}
	if !policyCmd(a.Cmd).equals(policyCmd(b.Cmd)) || a.Permissive != b.Permissive {
		return false
	}
	if normExprForCompare(a.UsingSQL) != normExprForCompare(b.UsingSQL) {
		return false
	}
	if normExprForCompare(a.WithCheck) != normExprForCompare(b.WithCheck) {
		return false
	}
	return stringSliceEqual(sortedCopy(a.Roles), sortedCopy(b.Roles))
}

// reTypeCast matches a trailing ::typename (including schema-qualified names).
var reTypeCast = regexp.MustCompile(`::[\w.]+`)

// normExprForCompare normalises a standalone SQL expression for semantic comparison.
// Used for policy USING / WITH CHECK fields where both the desired expression (from
// pg_query deparser) and the live expression (from pg_get_expr) need to be in the
// same canonical form.  We round-trip through the pg_query deparser to normalise
// both sides identically, then strip type casts so that inferred-cast differences
// (e.g. 'active'::user_status added by pg_get_expr) do not cause false diffs.
func normExprForCompare(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	// Round-trip through pg_query deparser for a canonical form.
	// Wrapping in SELECT(...) makes it a valid statement.
	dep, ok := deparseOneStmt("SELECT (" + s + ")")
	if ok {
		dep = strings.TrimSpace(dep)
		// Strip the leading "SELECT " token.
		dep = strings.TrimPrefix(strings.ToLower(dep), "select ")
		dep = strings.TrimSpace(dep)
		// Strip the single outer paren we added in SELECT(...).
		if len(dep) >= 2 && dep[0] == '(' && dep[len(dep)-1] == ')' {
			dep = dep[1 : len(dep)-1]
		}
		s = strings.TrimSpace(dep)
	}
	// Strip type casts: ::typename and ::schema.typename
	s = reTypeCast.ReplaceAllString(s, "")
	// Strip pg_catalog. qualifier from built-in function names (pg_query deparser may add this
	// when converting AT TIME ZONE to timezone() call, while the catalog stores the bare name).
	s = strings.ReplaceAll(s, "pg_catalog.", "")
	// Apply catalog normalization: ~~ → like, BETWEEN expansion, etc.
	s = normalizePGCatalogConstraintText(s)
	// Normalise whitespace (already lowercased above, but handle fallback path).
	return strings.TrimSpace(strings.ToLower(reMultiSpace.ReplaceAllString(s, " ")))
}

// policyCmd normalizes a policy command string for comparison.
// PostgreSQL uses '*' for ALL; the deparser uses 'all'; treat them the same.
type policyCmd string

// seqParams holds the meaningful attributes of a CREATE SEQUENCE statement.
// Missing parameters are filled with PostgreSQL defaults (ascending bigint sequence).
type seqParams struct {
	increment int64
	minvalue  int64
	maxvalue  int64
	start     int64
	cache     int64
	cycle     bool
}

// seqDefElemInt64 extracts the int64 value from a DefElem argument.
// pg_query represents small integers as Node_Integer and large integers (> int32 max)
// as Node_Float (string-encoded), so we must handle both.
func seqDefElemInt64(n *pgq.Node) int64 {
	if n == nil {
		return 0
	}
	if iv := n.GetInteger(); iv != nil {
		return int64(iv.GetIval())
	}
	if fv := n.GetFloat(); fv != nil {
		v, _ := strconv.ParseInt(fv.GetFval(), 10, 64)
		return v
	}
	return 0
}

// seqParamsFromSQL parses a CREATE SEQUENCE SQL string into a seqParams struct.
// Missing options are filled with PostgreSQL defaults for ascending bigint sequences.
func seqParamsFromSQL(sql string) (seqParams, bool) {
	const (
		defaultIncrement = int64(1)
		defaultMinvalue  = int64(1)
		defaultMaxvalue  = int64(9223372036854775807)
		defaultCache     = int64(1)
	)
	pr, err := pgq.Parse(sql)
	if err != nil || len(pr.GetStmts()) != 1 {
		return seqParams{}, false
	}
	stmt := pr.GetStmts()[0].GetStmt().GetCreateSeqStmt()
	if stmt == nil {
		return seqParams{}, false
	}
	p := seqParams{
		increment: defaultIncrement,
		minvalue:  defaultMinvalue,
		maxvalue:  defaultMaxvalue,
		start:     defaultMinvalue, // default start = default minvalue
		cache:     defaultCache,
		cycle:     false,
	}
	startExplicit := false
	for _, opt := range stmt.GetOptions() {
		el := opt.GetDefElem()
		if el == nil {
			continue
		}
		name := strings.ToLower(el.GetDefname())
		switch name {
		case "increment":
			p.increment = seqDefElemInt64(el.GetArg())
		case "start":
			p.start = seqDefElemInt64(el.GetArg())
			startExplicit = true
		case "minvalue":
			if el.GetArg() == nil {
				p.minvalue = defaultMinvalue // NO MINVALUE
			} else {
				p.minvalue = seqDefElemInt64(el.GetArg())
			}
		case "maxvalue":
			if el.GetArg() == nil {
				p.maxvalue = defaultMaxvalue // NO MAXVALUE
			} else {
				p.maxvalue = seqDefElemInt64(el.GetArg())
			}
		case "cache":
			p.cache = seqDefElemInt64(el.GetArg())
		case "cycle":
			// pg_query uses defname="cycle" with a Boolean arg: true=CYCLE, false=NO CYCLE.
			if bv := el.GetArg().GetBoolean(); bv != nil {
				p.cycle = bv.GetBoolval()
			} else {
				p.cycle = true // bare "cycle" without arg = CYCLE
			}
		case "nocycle":
			p.cycle = false
		}
	}
	// If start not explicitly set, default is minvalue.
	if !startExplicit {
		p.start = p.minvalue
	}
	return p, true
}

// seqParamsEqual compares two CREATE SEQUENCE SQL strings semantically by normalising
// both to their resolved parameter sets (filling in PostgreSQL defaults for omitted options).
// This avoids false diffs between source SQL (omits defaults) and the catalog SQL (explicit).
func seqParamsEqual(a, b string) bool {
	pa, ok1 := seqParamsFromSQL(a)
	pb, ok2 := seqParamsFromSQL(b)
	if !ok1 || !ok2 {
		// Fall back to raw string fingerprint comparison.
		return fpGenericSQL(a) == fpGenericSQL(b)
	}
	return pa == pb
}

// buildAlterSequenceSQL builds an ALTER SEQUENCE statement to update a sequence in-place
// from the desired CREATE SEQUENCE SQL. This avoids DROP+CREATE which would reset the
// current sequence value.  Returns "" if the desired SQL cannot be parsed.
func buildAlterSequenceSQL(desired *schema.Sequence) string {
	if desired == nil {
		return ""
	}
	p, ok := seqParamsFromSQL(desired.DefSQL)
	if !ok {
		return ""
	}
	cycleStr := "NO CYCLE"
	if p.cycle {
		cycleStr = "CYCLE"
	}
	return fmt.Sprintf(
		"ALTER SEQUENCE IF EXISTS %s.%s INCREMENT BY %d MINVALUE %d MAXVALUE %d CACHE %d %s",
		ident(desired.Schema), ident(desired.Name),
		p.increment, p.minvalue, p.maxvalue, p.cache, cycleStr,
	)
}

func (c policyCmd) normalized() string {
	s := strings.ToLower(strings.TrimSpace(string(c)))
	if s == "*" || s == "" {
		return "all"
	}
	return s
}

func (c policyCmd) equals(other policyCmd) bool {
	return c.normalized() == other.normalized()
}

func normExpr(s string) string {
	return strings.TrimSpace(strings.ToLower(reMultiSpace.ReplaceAllString(s, " ")))
}

func sortedCopy(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	o := append([]string(nil), s...)
	sort.Strings(o)
	return o
}

func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
