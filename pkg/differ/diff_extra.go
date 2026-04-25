package differ

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	pgq "github.com/pganalyze/pg_query_go/v6"

	"github.com/nexg/pg-flux/pkg/plan"
	"github.com/nexg/pg-flux/pkg/schema"
)

var reMultiSpace = regexp.MustCompile(`\s+`)
var reDefaultPublicSchemaDot = regexp.MustCompile(`(?i)\bpublic\.`)

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

func diffIndexes(d, l *schema.SchemaState) []change {
	var out []change
	ensureMaps(d)
	ensureMaps(l)
	for k, di := range d.Indexes {
		if di == nil {
			continue
		}
		li := l.Indexes[k]
		if li == nil {
			out = append(out, change{kind: plan.ChangeCreateIndex, idx: di})
			continue
		}
		if !indexDefsEqual(di, li) {
			out = append(out, change{kind: plan.ChangeDropIndex, dropIdx: k, sch: li.Schema, ixName: li.Name})
			out = append(out, change{kind: plan.ChangeCreateIndex, idx: di})
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

// quoteSQLIdent quotes a PostgreSQL identifier when it contains non-standard characters.
func quoteSQLIdent(s string) string {
	if s == "" {
		return `""`
	}
	needQuote := false
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_') {
			needQuote = true
			break
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
	return fpSQL(dep)
}

// createStmtDefFingerprint normalizes a single top-level CREATE (view, trigger, sequence, etc.)
// by parse+deparse, matching catalog- or file-built DDL that differs only cosmetically.
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
	return fpSQL(dep)
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
	if di == nil || li == nil {
		return di == li
	}
	a := indexFingerprintNormalizers(canonIndexSQL(di.CreateSQL), di.TableSchema)
	b := indexFingerprintNormalizers(canonIndexSQL(li.CreateSQL), li.TableSchema)
	return fpSQL(a) == fpSQL(b)
}

// indexFingerprintNormalizers collapse harmless differences in CREATE INDEX.
func indexFingerprintNormalizers(s string, tableSchema string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "concurrently", "")
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

// fpFunctionSQL normalizes delimiters/whitespace before pg_query Fingerprint, so deparse and catalog can match.
func fpFunctionSQL(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = reDollarDelim.ReplaceAllLiteralString(s, "$$")
	s = reMultiSpace.ReplaceAllString(s, " ")
	return fpSQL(s)
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
	if !strings.EqualFold(a.Cmd, b.Cmd) || a.Permissive != b.Permissive {
		return false
	}
	if normExpr(a.UsingSQL) != normExpr(b.UsingSQL) {
		return false
	}
	if normExpr(a.WithCheck) != normExpr(b.WithCheck) {
		return false
	}
	return stringSliceEqual(sortedCopy(a.Roles), sortedCopy(b.Roles))
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
