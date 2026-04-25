package schema

import (
	"regexp"
	"strings"
)

var spaceRE = regexp.MustCompile(`\s+`)

// NormalizeTypeForCompare maps common PostgreSQL type spellings to a form comparable
// to what pg_catalog.format_type typically returns, so "int" and "int4" match "integer".
func NormalizeTypeForCompare(s string) string {
	s = spaceRE.ReplaceAllString(strings.ToLower(strings.TrimSpace(s)), " ")
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	// Strip pg_catalog. prefix often seen in deparse/DDL
	s = strings.TrimPrefix(s, "pg_catalog.")
	// Type aliases: https://www.postgresql.org/docs/current/datatype.html#DATATYPE-TABLE
	switch s {
	case "int2", "smallint", "smallserial":
		return "smallint"
	case "int", "int4", "serial", "serial4", "integer":
		return "integer"
	case "int8", "serial8", "bigserial", "bigint":
		return "bigint"
	case "float4":
		return "real"
	case "float8", "float":
		return "double precision"
	case "bool", "boolean":
		return "boolean" // no-op, consistent spacing
	}
	// "character varying" vs "varchar" — keep as lowercased string for substring checks below
	if strings.HasPrefix(s, "varchar") || strings.HasPrefix(s, "character varying") {
		return s // keep distinct mod forms (varchar(10) vs char varying(10) still may differ; E2E uses simple types
	}
	return s
}

// splitTopLevelCommaTypes splits a comma-separated type list, respecting nested parentheses.
func splitTopLevelCommaTypes(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	var out []string
	var b strings.Builder
	depth := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				out = append(out, strings.TrimSpace(b.String()))
				b.Reset()
				continue
			}
		}
		b.WriteByte(c)
	}
	if b.Len() > 0 {
		out = append(out, strings.TrimSpace(b.String()))
	}
	return out
}

// BuildFunctionIdentity returns a map key: schema.name(typ1, typ2, ...) with normalized type names.
// commaSepArgTypes is a comma-separated type list (from the parser or from catalog format_type()).
func BuildFunctionIdentity(nsp, fn, commaSepArgTypes string) string {
	nsp = strings.ToLower(strings.TrimSpace(nsp))
	if nsp == "" {
		nsp = "public"
	}
	fn = strings.ToLower(strings.TrimSpace(fn))
	parts := splitTopLevelCommaTypes(commaSepArgTypes)
	if len(parts) == 0 && strings.TrimSpace(commaSepArgTypes) != "" {
		parts = []string{strings.TrimSpace(commaSepArgTypes)}
	}
	for i, p := range parts {
		p = strings.TrimSpace(p)
		p = strings.TrimPrefix(p, "pg_catalog.")
		p = stripOneOuterParensType(p)
		parts[i] = NormalizeTypeForCompare(p)
	}
	return nsp + "." + fn + "(" + strings.Join(parts, ", ") + ")"
}

func stripOneOuterParensType(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '(' && s[len(s)-1] == ')' {
		inner := strings.TrimSpace(s[1 : len(s)-1])
		if !strings.ContainsRune(inner, '(') {
			return inner
		}
	}
	return s
}
