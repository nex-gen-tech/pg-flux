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
	// Strip public. prefix — format_type() returns unqualified names for public-schema types
	s = strings.TrimPrefix(s, "public.")

	// Normalize spaces around commas in type modifiers: "numeric(10, 2)" → "numeric(10,2)".
	// pg_query deparser emits spaces after commas in typmod lists, while pg_catalog.format_type does not.
	s = strings.ReplaceAll(s, ", ", ",")

	// Issue 7: Array types — normalize base type then re-attach [] suffix.
	// Handles int[], int4[], bigint[], bool[], float4[], varchar(n)[], etc.
	if strings.HasSuffix(s, "[]") {
		base := NormalizeTypeForCompare(strings.TrimSuffix(s, "[]"))
		return base + "[]"
	}

	// Type aliases: https://www.postgresql.org/docs/current/datatype.html#DATATYPE-TABLE
	switch s {
	case "int2", "smallint", "smallserial":
		return "smallint"
	case "int", "int4", "serial", "serial4", "integer":
		return "integer"
	case "int8", "serial8", "bigserial", "bigint":
		return "bigint"
	case "timestamptz", "timestamp with time zone":
		return "timestamptz"
	// Issue 3: bare "timestamp" → "timestamp without time zone" (what format_type returns)
	case "timestamp":
		return "timestamp without time zone"
	case "timetz", "time with time zone":
		return "timetz"
	// Issue 4: bare "time" → "time without time zone"
	case "time":
		return "time without time zone"
	case "float4":
		return "real"
	case "float8", "float":
		return "double precision"
	case "bool", "boolean":
		return "boolean" // no-op, consistent spacing
	// Issue 2: decimal is an alias for numeric
	case "decimal":
		return "numeric"
	// Issue 1/6: bpchar is pg_query's internal name for char/character; bare bpchar = character(1)
	case "bpchar":
		return "character(1)"
	// Issue 6: bare char / character (no length specifier) → character(1)
	case "char", "character":
		return "character(1)"
	}

	// Issue 2: decimal(p,s) → numeric(p,s)
	if strings.HasPrefix(s, "decimal(") {
		return "numeric" + strings.TrimPrefix(s, "decimal")
	}

	// Issue 1: bpchar(n) is pg_query's internal representation of char(n) / character(n)
	if strings.HasPrefix(s, "bpchar(") {
		return "character" + strings.TrimPrefix(s, "bpchar")
	}

	// Issue 1: char(n) → character(n) to match pg_catalog.format_type output
	if strings.HasPrefix(s, "char(") {
		return "character" + strings.TrimPrefix(s, "char")
	}

	// Issue 5: varbit(n) → bit varying(n); varbit → bit varying
	if s == "varbit" {
		return "bit varying"
	}
	if strings.HasPrefix(s, "varbit(") {
		return "bit varying" + strings.TrimPrefix(s, "varbit")
	}

	// "character varying" vs "varchar" — canonicalize to "character varying" to match pg_catalog.format_type output
	if strings.HasPrefix(s, "varchar") {
		s = "character varying" + strings.TrimPrefix(s, "varchar")
		return s
	}
	if strings.HasPrefix(s, "character varying") {
		return s
	}
	// character(n) — already correct; character without parens handled in switch above
	if strings.HasPrefix(s, "character(") {
		return s
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
