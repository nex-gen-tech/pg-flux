package differ

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	pgq "github.com/pganalyze/pg_query_go/v6"

	"github.com/nex-gen-tech/pg-flux/pkg/plan"
	"github.com/nex-gen-tech/pg-flux/pkg/schema"
)

var reMultiSpace = regexp.MustCompile(`\s+`)
var reDefaultPublicSchemaDot = regexp.MustCompile(`(?i)\bpublic\.`)

// reAnyArray matches a PostgreSQL catalog "= ANY(ARRAY[...::type, ...])" pattern produced
// by pg_get_constraintdef for IN-list CHECK constraints.
//
// NOTE: this regex has a known limitation — it stops at the first `]` even when
// it's inside a single-quoted string literal. Callers that need to handle
// brackets inside literals (e.g. user values like 'a]b') should use
// normalizeAnyArrayForFingerprint instead.
var reAnyArray = regexp.MustCompile(`= any\(array\[([^\]]+)\]\)`)

// normalizeAnyArrayForFingerprint rewrites every "= ANY (ARRAY[item, ...])"
// construct in s to the equivalent "IN (item, ...)" form, walking the string
// with quote-state awareness so that brackets inside single-quoted literals
// don't terminate the match early. Type casts (::text, ::int4, etc.) on each
// element are stripped — pg_get_constraintdef emits them but source SQL
// typically doesn't. This is the quote-safe replacement for the legacy
// reAnyArray/reAnyArrayInList regexes.
func normalizeAnyArrayForFingerprint(s string) string {
	lower := strings.ToLower(s)
	var out strings.Builder
	out.Grow(len(s))
	i := 0
	for i < len(s) {
		// Look for "= any (array[" allowing flexible whitespace.
		idx := indexAnyArrayStart(lower, i)
		if idx < 0 {
			out.WriteString(s[i:])
			break
		}
		out.WriteString(s[i:idx])
		// Find the start of the `[` after array.
		openBracket := strings.Index(lower[idx:], "[")
		if openBracket < 0 {
			out.WriteString(s[idx:])
			break
		}
		openBracket += idx
		// Walk forward respecting single-quoted strings until matching `]`.
		closeBracket := -1
		inQuote := false
		for j := openBracket + 1; j < len(s); j++ {
			c := s[j]
			if c == '\'' {
				if inQuote && j+1 < len(s) && s[j+1] == '\'' {
					j++ // skip '' escape
					continue
				}
				inQuote = !inQuote
				continue
			}
			if inQuote {
				continue
			}
			if c == ']' {
				closeBracket = j
				break
			}
		}
		if closeBracket < 0 {
			out.WriteString(s[idx:])
			break
		}
		inner := s[openBracket+1 : closeBracket]
		// Strip type casts on each element: 'foo'::text → 'foo'.
		inner = reCastSuffix.ReplaceAllString(inner, "$1")
		// Collapse whitespace.
		inner = reMultiSpace.ReplaceAllString(inner, " ")
		inner = strings.TrimSpace(inner)
		out.WriteString(" IN (")
		out.WriteString(inner)
		out.WriteString(")")
		// Skip past the closing `)` after `])`.
		k := closeBracket + 1
		for k < len(s) && (s[k] == ' ' || s[k] == '\t') {
			k++
		}
		if k < len(s) && s[k] == ')' {
			i = k + 1
		} else {
			i = closeBracket + 1
		}
	}
	return out.String()
}

// indexAnyArrayStart finds the next "= any (array" occurrence in lower starting
// at index from, allowing flexible whitespace between tokens. Returns the
// position of the `=` character, or -1.
func indexAnyArrayStart(lower string, from int) int {
	for {
		eq := strings.Index(lower[from:], "= any")
		if eq < 0 {
			return -1
		}
		eq += from
		// Find "array" after "any", skipping whitespace and optional "(".
		j := eq + len("= any")
		for j < len(lower) && (lower[j] == ' ' || lower[j] == '\t') {
			j++
		}
		if j >= len(lower) || lower[j] != '(' {
			from = eq + 1
			continue
		}
		j++
		for j < len(lower) && (lower[j] == ' ' || lower[j] == '\t') {
			j++
		}
		if j+5 <= len(lower) && lower[j:j+5] == "array" {
			return eq
		}
		from = eq + 1
	}
}

// reCastSuffix strips a trailing "::typename" cast from a quoted literal, e.g.
//   'pending'::text          → 'pending'
//   'high'::todo_priority    → 'high'
//   'x'::public.my_enum      → 'x'
// The type-name character class includes _ (for user-defined types like
// todo_priority) and . (for schema-qualified types like public.my_enum), as
// well as spaces (for multi-word built-ins like "character varying").
var reCastSuffix = regexp.MustCompile(`('+[^']*'+)::[a-z][\w. ]*`)

// reTextCast strips explicit "::text" casts added by PostgreSQL when a varchar/char column is used
// in a constraint expression (e.g. email::text like '%@%' → email like '%@%').
var reTextCast = regexp.MustCompile(`::\s*text\b`)

// reTypeCastBroad matches a PostgreSQL "::type" suffix, including multi-word types like
// `::character varying`, `::double precision`, `::timestamp with time zone`, and size
// modifiers like `::varchar(50)` or array suffixes `::int4[]`. Used by expression
// normalizers so type-coercion noise added by pg_get_expr / pg_get_constraintdef does
// not cause false-positive diffs against the source SQL.
var reTypeCastBroad = regexp.MustCompile(
	`::"?[\w]+"?(?:\."?[\w]+"?)?` + // ::name or ::schema.name (optionally quoted)
		`(?:\s+(?:varying|precision|with(?:out)?\s+time\s+zone|to\s+\w+))*` + // multi-word trailers
		`(?:\s*\(\s*\d+(?:\s*,\s*\d+)?\s*\))?` + // optional (n) or (n, m)
		`(?:\s*\[\s*\])?`, // optional []
)

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
	// Issue #8: pg_get_viewdef emits "CREATE VIEW ..." (never "CREATE OR REPLACE VIEW"),
	// but users frequently write "CREATE OR REPLACE VIEW" in source SQL. The "OR REPLACE"
	// clause is a metadata-only directive — it affects how the statement is *applied*, not
	// the resulting view body. Strip it so the two forms fingerprint identically.
	dep = reCreateOrReplace.ReplaceAllString(dep, "create ")
	// Strip type casts added by pg_get_viewdef / pg_get_triggerdef to match the source
	// SQL which typically omits them (e.g. 'archived'::post_status → 'archived'; or
	// handle::character varying → handle for generated-column-style coercions).
	dep = reTypeCastBroad.ReplaceAllString(dep, "")
	// Strip a CREATE VIEW … WITH (…) options clause. View options are tracked
	// separately on the View struct (CheckOption / SecurityBarrier / SecurityInvoker)
	// and diff'd via diffViewAttrs. Removing them from the body fingerprint keeps
	// pure body comparison clean.
	dep = reViewWithOptions.ReplaceAllString(dep, " ")
	// PG15 emits column references as "table.column" inside pg_get_viewdef while
	// PG16+ omits the redundant qualifier. Strip qualifier prefixes so both forms
	// fingerprint identically. (Multi-table joins with same column name on both
	// sides would lose precision here, but views of that shape are uncommon and
	// users can disambiguate via AS aliases.)
	dep = reViewColQualifier.ReplaceAllString(dep, "$1")
	// pg_get_viewdef rewrites WHERE x IN (a, b) → x = ANY (ARRAY[a, b]). Source-side
	// deparse preserves "IN (...)". Collapse the live form back to IN-list for the
	// fingerprint. Uses the quote-aware walker so brackets inside literal strings
	// don't terminate the match early.
	dep = strings.ToLower(normalizeAnyArrayForFingerprint(dep))
	// Trailing semicolon, redundant whitespace.
	dep = strings.TrimSuffix(strings.TrimSpace(dep), ";")
	return strings.TrimSpace(reMultiSpace.ReplaceAllString(dep, " "))
}

// reAnyArrayInList canonicalises PostgreSQL's "= ANY (ARRAY[...])" into "IN (...)".
// pg_get_viewdef rewrites IN-lists this way; pg_query.Deparse preserves the original
// form, so without this normalization source ("IN") and live ("= ANY (ARRAY[...])")
// don't fingerprint identically.
var reAnyArrayInList = regexp.MustCompile(`\s*=\s*any\s*\(\s*array\s*\[\s*([^\]]+?)\s*\]\s*\)`)

// reViewColQualifier strips "tablename." prefixes off column refs in view bodies.
// Anchored to a leading SELECT-area token boundary so FROM table.column doesn't
// match.  We use word-then-dot-then-word, capture the trailing column name.
var reViewColQualifier = regexp.MustCompile(`\b[a-z_][a-z0-9_]*\.([a-z_][a-z0-9_]*)`)

// reViewWithOptions matches a "with (…)" options clause that immediately follows
// the view name in a CREATE VIEW statement (case-insensitive, single-line).
var reViewWithOptions = regexp.MustCompile(`(?i)\bwith\s*\([^)]*\)\s*`)

// reCreateOrReplace matches the literal "create or replace " (case-insensitive,
// whitespace-flexible) at the start of a CREATE statement so the body fingerprint
// doesn't depend on whether the user wrote `CREATE` or `CREATE OR REPLACE`.
// pg_get_viewdef / pg_get_triggerdef / pg_get_functiondef never emit `OR REPLACE`,
// so collapsing both forms to bare `CREATE ` keeps source and catalog aligned.
var reCreateOrReplace = regexp.MustCompile(`(?i)\bcreate\s+or\s+replace\s+`)

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
	// Compare the normalised strings directly. We deliberately do NOT use
	// pgq.Fingerprint here: it replaces every literal constant with a $n
	// placeholder, which collapses semantically-distinct predicates like
	// `WHERE priority IN ('high', 'urgent')` vs `WHERE priority IN ('high', 'critical')`
	// to the same fingerprint — a silent false-negative that Issue #7's
	// negative tests are designed to catch.
	return a == b
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

// reIndexAscNullsLast / reIndexDescNullsFirst strip redundant NULLS clauses where
// they match PG's default ordering — pg_get_indexdef omits them, but source SQL
// often spells them out:
//   ASC NULLS LAST    is the ASC default  → bare ASC (or nothing)
//   DESC NULLS FIRST  is the DESC default → bare DESC
//   ASC               is the column default → strip entirely
var (
	reIndexAscNullsLast   = regexp.MustCompile(`(?i)\basc\s+nulls\s+last\b`)
	reIndexDescNullsFirst = regexp.MustCompile(`(?i)\bdesc\s+nulls\s+first\b`)
	reIndexBareAsc        = regexp.MustCompile(`(?i)\basc\b\s*(,|\))`)
)

// indexFingerprintNormalizers collapse harmless differences in CREATE INDEX.
func indexFingerprintNormalizers(s string, tableSchema string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "concurrently", "")
	// pg_get_indexdef never includes IF NOT EXISTS; strip it for comparison.
	s = strings.ReplaceAll(s, "if not exists", "")
	// pg_get_indexdef uses "ON ONLY" for indexes on partitioned table parents;
	// source SQL omits "ONLY". Normalize both to "ON".
	s = strings.ReplaceAll(s, " on only ", " on ")
	// Strip redundant default NULLS clauses (see regex docs above).
	s = reIndexAscNullsLast.ReplaceAllString(s, "")
	s = reIndexDescNullsFirst.ReplaceAllString(s, "desc")
	// Strip bare "ASC" before commas / closing paren (column default ordering).
	s = reIndexBareAsc.ReplaceAllString(s, "$1")
	ts := strings.TrimSpace(strings.ToLower(tableSchema))
	if ts != "" {
		s = strings.ReplaceAll(s, "on "+ts+".", "on ")
	}
	// Issue #7: pg_get_indexdef rewrites a WHERE predicate of the form
	//   "col IN (lit, lit, ...)"
	// to
	//   "col = ANY (ARRAY[lit::sometype, lit::sometype, ...])"
	// (and may also strip the ::type casts the user wrote). Source SQL keeps
	// the IN(...) form. Collapse the catalog form back to IN-list so the two
	// fingerprint identically. The quote-aware walker handles brackets inside
	// string literals; type casts on individual elements are stripped.
	//
	// CRITICAL: this is a syntactic equivalence — `IN (a, b)` and
	// `IN (a, b, c)` still produce different normalized forms, so adding or
	// removing list elements is still detected as drift.
	s = strings.ToLower(normalizeAnyArrayForFingerprint(s))
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
	// First pass: round-trip through pg_query Parse+Deparse so both source-side
	// and pg_get_functiondef strings end up in the SAME canonical form. This
	// eliminates whitespace, keyword-case, and comment differences.
	if dep, ok := deparseOneStmt(strings.TrimSpace(s)); ok {
		s = dep
	}
	s = strings.ToLower(strings.TrimSpace(s))
	s = reDollarDelim.ReplaceAllLiteralString(s, "$$")
	s = reMultiSpace.ReplaceAllString(s, " ")
	// Normalize type aliases in the function header (before the function body).
	// Split on $$ to isolate the header from the body — avoid modifying body literals.
	parts := strings.SplitN(s, "$$", 2)
	header := parts[0]
	// Strip "security invoker" — it's the default; pg_get_functiondef omits it,
	// so explicit source mentions cause spurious diffs.
	header = reSecurityInvoker.ReplaceAllLiteralString(header, " ")
	// Normalize SET clause: "set k = v" / "set k to v" / "set k = 'v'" → "set k to v"
	header = reSetClauseOp.ReplaceAllString(header, "set $1 to ")
	header = reSetClauseQuoted.ReplaceAllString(header, "set $1 to $2")
	// Normalize type aliases.
	header = reFnTypeTokens.ReplaceAllStringFunc(header, fnNormType)
	// Sort the option clauses between the RETURNS clause and the AS $$.
	header = sortFunctionOptions(header)
	parts[0] = header
	s = strings.Join(parts, "$$")
	s = reMultiSpace.ReplaceAllString(s, " ")
	return s
}

var (
	// reSetClauseOp normalises a SET clause without quoted value:
	//   "set name = value" / "set name to value" → "set name to value"
	reSetClauseOp = regexp.MustCompile(`\bset\s+(\w+)\s*(?:=|to)\s*`)
	// reSetClauseQuoted strips outer single quotes from the SET value:
	//   "set name to 'value'" → "set name to value"
	reSetClauseQuoted = regexp.MustCompile(`\bset\s+(\w+)\s+to\s+'([^']*)'`)
	// reSecurityInvoker matches the explicit SECURITY INVOKER clause (the PG default).
	reSecurityInvoker = regexp.MustCompile(`\bsecurity\s+invoker\b`)
	// reFunctionOptionClauses matches the cluster of function-option tokens between
	// the LANGUAGE clause and the AS $$ delimiter so they can be reordered into
	// a stable canonical sequence.
	reFunctionOptionClauses = regexp.MustCompile(`(language\s+\w+)\s+(.*?)\s+as\s*$`)
)

// sortFunctionOptions canonicalises the cluster of optional clauses appearing
// between LANGUAGE <lang> and AS $$. pg_get_functiondef orders them in a fixed
// way; source can declare them in any order. We split on whitespace, group
// known multi-token clauses (PARALLEL SAFE, etc.), sort, and re-emit.
func sortFunctionOptions(header string) string {
	// Find the LANGUAGE clause and the trailing " as " sentinel; rewrite middle.
	// Use a non-regex anchor to avoid greedy issues with literals in defaults.
	lower := strings.ToLower(header)
	lang := strings.Index(lower, "language ")
	asKw := strings.LastIndex(lower, " as ")
	if lang < 0 || asKw < 0 || asKw < lang {
		return header
	}
	// Advance past "language <word>"
	rest := header[lang+len("language "):]
	space := strings.IndexAny(rest, " \t\n")
	if space < 0 {
		return header
	}
	langTokenEnd := lang + len("language ") + space
	mid := header[langTokenEnd:asKw]
	mid = strings.TrimSpace(mid)
	if mid == "" {
		return header
	}
	// Tokenize. Group well-known multi-word clauses.
	tokens := splitFunctionOptions(mid)
	sort.Strings(tokens)
	rebuilt := header[:langTokenEnd] + " " + strings.Join(tokens, " ") + header[asKw:]
	return rebuilt
}

// splitFunctionOptions splits the option cluster into atomic clauses, grouping
// known multi-token clauses (PARALLEL SAFE, SECURITY DEFINER, COST n, ROWS n,
// SET name TO value, NOT LEAKPROOF). Single-word options pass through.
//
// Tokenization is quote-aware: single-quoted string literals stay as one token
// so "SET application_name TO 'my app name'" remains a single 4-token clause
// instead of being split inside the quoted value (would have caused spurious
// function-fingerprint drift on any pg_get_functiondef output with quoted SET
// values containing spaces).
//
// SET clause value is greedy: it absorbs the rest of the input up to the next
// known top-level keyword (parallel/security/cost/rows/not/stable/immutable/
// volatile/leakproof/strict/window/external/transform/support). This makes
// `set search_path to '$user', public, app_data` survive as one clause even
// though it spans multiple tokens after the comma.
func splitFunctionOptions(s string) []string {
	tokens := quoteAwareFields(s)
	var out []string
	i := 0
	for i < len(tokens) {
		w := tokens[i]
		switch strings.ToLower(w) {
		case "parallel", "security":
			if i+1 < len(tokens) {
				out = append(out, w+" "+tokens[i+1])
				i += 2
				continue
			}
		case "cost", "rows":
			if i+1 < len(tokens) {
				out = append(out, w+" "+tokens[i+1])
				i += 2
				continue
			}
		case "not":
			if i+1 < len(tokens) {
				out = append(out, w+" "+tokens[i+1])
				i += 2
				continue
			}
		case "set":
			// "set <k> to <v>" — value is greedy up to next top-level keyword.
			if i+3 < len(tokens) && strings.EqualFold(tokens[i+2], "to") {
				end := i + 4
				for end < len(tokens) && !isFunctionTopLevelKeyword(tokens[end]) {
					end++
				}
				out = append(out, strings.Join(tokens[i:end], " "))
				i = end
				continue
			}
		}
		out = append(out, w)
		i++
	}
	return out
}

// isFunctionTopLevelKeyword reports whether a token starts a new top-level
// function-options clause (i.e., ends the greedy value capture of a SET clause).
func isFunctionTopLevelKeyword(tok string) bool {
	switch strings.ToLower(tok) {
	case "parallel", "security", "cost", "rows", "not",
		"stable", "immutable", "volatile", "leakproof", "strict",
		"window", "external", "transform", "support", "set":
		return true
	}
	return false
}

// quoteAwareFields splits s on whitespace, treating single-quoted runs as one
// token. PG's pg_get_functiondef quotes string values with single quotes and
// escapes embedded quotes with `''`. Whitespace inside the quoted range is
// preserved as part of the token.
func quoteAwareFields(s string) []string {
	var out []string
	var cur strings.Builder
	inQuote := false
	flush := func() {
		if cur.Len() > 0 {
			out = append(out, cur.String())
			cur.Reset()
		}
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '\'':
			cur.WriteByte(c)
			if inQuote {
				// '' = escaped quote inside the literal — keep inQuote.
				if i+1 < len(s) && s[i+1] == '\'' {
					cur.WriteByte('\'')
					i++
					continue
				}
				inQuote = false
			} else {
				inQuote = true
			}
		case !inQuote && (c == ' ' || c == '\t' || c == '\n' || c == '\r'):
			flush()
		default:
			cur.WriteByte(c)
		}
	}
	flush()
	return out
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
			// Prefer ALTER POLICY when only USING/WITH CHECK/role list differ — Cmd
			// (FOR ...) and Permissive (AS PERMISSIVE/RESTRICTIVE) cannot be altered
			// in PostgreSQL, so those force DROP+CREATE.
			if policyCmd(dp.Cmd).equals(policyCmd(lp.Cmd)) && dp.Permissive == lp.Permissive {
				out = append(out, change{kind: plan.ChangeAlterPolicy, pol: dp, polKey: k})
			} else {
				out = append(out, change{kind: plan.ChangeDropPolicy, polKey: k, pol: lp})
				out = append(out, change{kind: plan.ChangeCreatePolicy, pol: dp, polKey: k})
			}
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
// Note: prefer reTypeCastBroad for fingerprint contexts where multi-word types
// (character varying, timestamp with time zone, etc.) appear.
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
	// Strip type casts: ::typename, ::schema.typename, and multi-word types like
	// `::character varying(50)` or `::timestamp with time zone` that PG injects in
	// pg_get_expr output but the desired-state SQL omits.
	s = reTypeCastBroad.ReplaceAllString(s, "")
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

// buildAlterPolicySQL renders an ALTER POLICY statement for in-place modification of an
// existing RLS policy. PostgreSQL's ALTER POLICY supports changing the TO role list,
// the USING expression, and the WITH CHECK expression — but NOT the command kind
// (FOR SELECT/INSERT/...) or AS PERMISSIVE/RESTRICTIVE. The caller is responsible for
// ensuring those compatibility conditions before emitting this change.
//
// Clauses are emitted unconditionally when present on the desired policy so the result
// is idempotent: re-applying the same ALTER POLICY against an already-aligned policy
// leaves the catalog unchanged. This avoids the brief DROP+CREATE window that would
// otherwise leave an RLS-enforcing table without its policy.
func buildAlterPolicySQL(p *schema.Policy) string {
	if p == nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "ALTER POLICY %s ON %s.%s",
		ident(p.Name), ident(p.Schema), ident(p.Table))
	if len(p.Roles) > 0 {
		roles := make([]string, len(p.Roles))
		for i, r := range p.Roles {
			roles[i] = ident(r)
		}
		fmt.Fprintf(&b, " TO %s", strings.Join(roles, ", "))
	}
	if strings.TrimSpace(p.UsingSQL) != "" {
		fmt.Fprintf(&b, " USING (%s)", p.UsingSQL)
	}
	if strings.TrimSpace(p.WithCheck) != "" {
		fmt.Fprintf(&b, " WITH CHECK (%s)", p.WithCheck)
	}
	return b.String()
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
		"ALTER SEQUENCE IF EXISTS %s.%s INCREMENT BY %d MINVALUE %d MAXVALUE %d START WITH %d CACHE %d %s",
		ident(desired.Schema), ident(desired.Name),
		p.increment, p.minvalue, p.maxvalue, p.start, p.cache, cycleStr,
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
