package dag

import (
	"regexp"
	"strings"

	"github.com/nexg/pg-flux/pkg/plan"
)

// Extended DDL heuristics for types/domains and type-like references.
// See package depgraph: not every PostgreSQL form is modeled.

var (
	// identOrQuoted matches a schema-qualified or bare identifier, with optional double-quoting.
	// Matches: my_type, public.my_type, "My Type", "pub"."My Type"
	identOrQuoted = `(?:(?:(?:"[^"]*"|[a-z_][a-z0-9_]*)\.)?(?:"[^"]*"|[a-z_][a-z0-9_]*))`

	reDDLCreateType   = regexp.MustCompile(`(?is)CREATE\s+TYPE\s+(` + identOrQuoted + `)`)
	reDDLCreateDomain = regexp.MustCompile(`(?is)CREATE\s+DOMAIN\s+(` + identOrQuoted + `)`)
	reAllQualified    = regexp.MustCompile(`\b([a-z_][a-z0-9_]*\.[a-z_][a-z0-9_]*)\b`)
	reDomainAS        = regexp.MustCompile(`(?is)CREATE\s+DOMAIN(?:\s+IF\s+NOT\s+EXISTS)?\s+(?:[a-z_][a-z0-9_]*\.)?[a-z_][a-z0-9_]*\s+AS\s+((?:[a-z_][a-z0-9_]*\.)?[a-z_][a-z0-9_]*|[_a-z][a-z0-9_]*)\b`)
	reTypeRangeSub    = regexp.MustCompile(`(?is)SUBTYPE\s*=\s*((?:[a-z_][a-z0-9_]*\.)?[a-z_][a-z0-9_]*|[_a-z][a-z0-9_]*)\b`)
	// Go regexp has no lookahead; we only match schema-qualified types (a.b).
	reColQualType    = regexp.MustCompile(`(?i)(?:,|\()\s*_*[a-z_][a-z0-9_]*\s+([a-z_][a-z0-9_]*\.[a-z_][a-z0-9_]*)`)
	reReturnsQual    = regexp.MustCompile(`(?i)\bRETURNS\s+((?:[a-z_][a-z0-9_]*\.)[a-z_][a-z0-9_]*)`)
	reDDLCreateTable  = regexp.MustCompile(`(?is)^\s*CREATE\s+TABLE(?:\s+IF\s+NOT\s+EXISTS)?\s+`)
)

func ddlStripped(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	var b strings.Builder
	for _, line := range strings.Split(s, "\n") {
		if i := strings.Index(line, "--"); i >= 0 {
			b.WriteString(line[:i])
		} else {
			b.WriteString(line)
		}
		b.WriteByte('\n')
	}
	return strings.TrimSpace(b.String())
}

func firstStatementPrefix(s string) string {
	s = strings.TrimSpace(s)
	depth := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case ';':
			if depth == 0 {
				return strings.TrimSpace(s[:i])
			}
		}
	}
	return s
}

func ddlProducers(s plan.Statement) []string {
	ddl := ddlStripped(s.DDL)
	if ddl == "" {
		return nil
	}
	first := firstStatementPrefix(ddl)
	switch s.OpType {
	case "CREATE_TYPE":
		if o := strings.TrimSpace(s.Object); o != "" {
			return []string{normalizeObjKey(o)}
		}
	case "CREATE_DOMAIN":
		if o := strings.TrimSpace(s.Object); o != "" {
			return []string{normalizeObjKey(o)}
		}
	case "RAW_DDL":
		if m := reDDLCreateType.FindStringSubmatch(first); len(m) > 1 {
			return []string{normalizeObjKey(m[1])}
		}
		if m := reDDLCreateDomain.FindStringSubmatch(first); len(m) > 1 {
			return []string{normalizeObjKey(m[1])}
		}
	}
	return nil
}

func registerDDLProducers(producer map[string]int, i int, s plan.Statement) {
	for _, k := range ddlProducers(s) {
		if k == "" {
			continue
		}
		producer[normalizeObjKey(k)] = i
	}
}

// ddlTypeRequires extra dependency keys for ordering (see depgraph).
func ddlTypeRequires(s plan.Statement) []string {
	ddl := ddlStripped(s.DDL)
	if ddl == "" {
		return nil
	}
	seen := map[string]struct{}{}
	var out []string
	add := func(k string) {
		k = strings.TrimSpace(k)
		if k == "" || !strings.Contains(k, ".") {
			return
		}
		k = normalizeObjKey(k)
		if k == "" {
			return
		}
		sch, _ := splitKeySchema(k)
		if sch == "pg_catalog" {
			return
		}
		if _, ok := seen[k]; ok {
			return
		}
		seen[k] = struct{}{}
		out = append(out, k)
	}
	op := s.OpType
	if op == "RAW_DDL" || op == "CREATE_DOMAIN" {
		if m := reDomainAS.FindStringSubmatch(ddl); len(m) > 1 && strings.Contains(m[1], ".") {
			add(m[1])
		}
	}
	// RANGE ( … SUBTYPE = t … )
	if strings.Contains(strings.ToLower(ddl), " as range") {
		if m := reTypeRangeSub.FindStringSubmatch(ddl); len(m) > 1 && strings.Contains(m[1], ".") {
			add(m[1])
		}
	}
	// COMPOSITE: all qualified a.b in attribute list, excluding the created type
	if m := reDDLCreateType.FindStringSubmatch(firstStatementPrefix(ddl)); len(m) > 1 {
		created := normalizeObjKey(m[1])
		lo := strings.ToLower(ddl)
		asI := strings.Index(lo, " as (")
		if asI < 0 {
			asI = strings.Index(lo, "as (")
		}
		if asI < 0 {
			asI = strings.Index(lo, "as(")
		}
		if asI >= 0 {
			rest := ddl[asI:]
			open := strings.IndexByte(rest, '(')
			if open >= 0 {
				seg := extractParenGroup(rest, open)
				for _, sm := range reAllQualified.FindAllStringSubmatch(seg, -1) {
					if len(sm) < 2 {
						continue
					}
					if normalizeObjKey(sm[1]) == created {
						continue
					}
					add(sm[1])
				}
			}
		}
	}
	var firstP string
	if op == "RAW_DDL" {
		firstP = firstStatementPrefix(ddl)
	}
	tableish := op == "CREATE_TABLE" || (op == "RAW_DDL" && reDDLCreateTable.MatchString(firstP))
	if tableish {
		for _, m := range reColQualType.FindAllStringSubmatch(ddl, -1) {
			if len(m) < 2 {
				continue
			}
			add(m[1])
		}
	}
	if op == "CREATE_FUNCTION" || op == "CREATE_AGGREGATE" || op == "CREATE_WINDOW_FUNCTION" {
		if m := reReturnsQual.FindStringSubmatch(ddl); len(m) > 1 {
			if strings.Contains(m[1], ".") {
				add(m[1])
			}
		}
	}
	return out
}

// extractParenGroup returns inner string for the matching paren at startIdx.
func extractParenGroup(s string, startIdx int) string {
	if startIdx < 0 || startIdx >= len(s) || s[startIdx] != '(' {
		return ""
	}
	d := 0
	for i := startIdx; i < len(s); i++ {
		switch s[i] {
		case '(':
			d++
		case ')':
			d--
			if d == 0 {
				return s[startIdx+1 : i]
			}
		}
	}
	return ""
}

func splitKeySchema(k string) (string, string) {
	p := strings.SplitN(k, ".", 2)
	if len(p) < 2 {
		return "public", p[0]
	}
	return p[0], p[1]
}
