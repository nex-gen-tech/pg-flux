package codegen

import (
	"strings"
	"unicode"
)

// PascalCase converts snake_case / kebab-case / dotted identifiers into
// PascalCase suitable for Go type names and TS interface names.
//
//	user_role        → UserRole
//	user-role        → UserRole
//	public.users     → PublicUsers (callers usually pass just the table name)
//	HTTPheader       → Httpheader   (collapses to one casing run)
//	id               → ID            (when ID is in initialisms; see PascalCaseInit)
func PascalCase(s string) string {
	if s == "" {
		return ""
	}
	var b strings.Builder
	upper := true
	for _, r := range s {
		switch {
		case r == '_' || r == '-' || r == '.' || r == ' ':
			upper = true
		default:
			if upper {
				b.WriteRune(unicode.ToUpper(r))
				upper = false
			} else {
				b.WriteRune(unicode.ToLower(r))
			}
		}
	}
	return b.String()
}

// PascalCaseInit is PascalCase with common initialisms upper-cased (matches the
// Go review style: ID, URL, HTTP, JSON, UUID, etc.). Only applied to whole-word
// tokens; "userid" stays "Userid".
func PascalCaseInit(s string) string {
	pc := PascalCase(s)
	for _, init := range commonInitialisms {
		// Only replace a full word boundary — uppercase the matching prefix
		// "Id" → "ID" at start, end, or before the next capital.
		pc = upperInit(pc, init)
	}
	return pc
}

// upperInit upper-cases an initialism at word boundaries inside an already-
// PascalCased string. The "word boundary" is start-of-string, end-of-string,
// or the next character being uppercase.
func upperInit(s, init string) string {
	if !strings.Contains(s, initPrefix(init)) && !strings.HasSuffix(s, initPrefix(init)) {
		return s
	}
	target := initPrefix(init) // "Id" form
	replace := strings.ToUpper(init)
	var b strings.Builder
	i := 0
	for i < len(s) {
		if i+len(target) <= len(s) && s[i:i+len(target)] == target {
			end := i + len(target)
			// Word boundary: end of string OR next char is uppercase
			if end == len(s) || (s[end] >= 'A' && s[end] <= 'Z') {
				b.WriteString(replace)
				i = end
				continue
			}
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

func initPrefix(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + strings.ToLower(s[1:])
}

// commonInitialisms is the conservative subset from golang/lint that almost
// every reviewer agrees on.
var commonInitialisms = []string{
	"ID", "URL", "URI", "HTTP", "HTTPS", "JSON", "XML", "SQL", "UUID",
	"API", "CSS", "DNS", "TCP", "UDP", "TLS", "IP", "DB",
}

// CamelCase is PascalCase with a lowercase first letter — used for Go local
// variables and TS field names when camelCase is preferred over snake_case.
func CamelCase(s string) string {
	pc := PascalCase(s)
	if pc == "" {
		return ""
	}
	return strings.ToLower(pc[:1]) + pc[1:]
}

// SnakeCase converts CamelCase or PascalCase back to snake_case. Used to round-
// trip column names when emitting `db:"snake_case"` struct tags from an
// already-PascalCased Go field.
func SnakeCase(s string) string {
	if s == "" {
		return ""
	}
	var b strings.Builder
	for i, r := range s {
		if i > 0 && unicode.IsUpper(r) {
			b.WriteByte('_')
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

// Singular returns a best-effort singularisation of a (presumed plural) table
// name. The rules are deliberately conservative: only the common English
// pluralisations are handled. Users who need irregular cases can override via
// the codegen config's name overrides map.
//
//	users        → user
//	addresses    → address
//	categories   → category
//	matrices     → matrice          (we DO NOT handle irregular forms)
//	settings     → setting           (drop trailing 's' is the safe default)
func Singular(s string) string {
	if s == "" {
		return s
	}
	low := strings.ToLower(s)
	switch {
	case strings.HasSuffix(low, "ies") && len(s) > 3:
		return s[:len(s)-3] + "y"
	case strings.HasSuffix(low, "sses"):
		return s[:len(s)-2]
	case strings.HasSuffix(low, "ches"), strings.HasSuffix(low, "shes"),
		strings.HasSuffix(low, "xes"), strings.HasSuffix(low, "zes"):
		return s[:len(s)-2]
	case strings.HasSuffix(low, "s") && !strings.HasSuffix(low, "ss"):
		return s[:len(s)-1]
	}
	return s
}

// EscapeStringLiteral escapes a string for embedding in Go double-quoted or TS
// double-quoted form. Quote character is single ('"') in both languages so the
// same helper works.
func EscapeStringLiteral(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// SortedKeys returns a stable-ordered list of keys for any map[string]T;
// generic so emitters don't reimplement it per kind.
func SortedKeys[T any](m map[string]T) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	// Plain string sort is fine; pg identifiers are ASCII-safe in practice.
	for i := 1; i < len(out); i++ {
		j := i
		for j > 0 && out[j-1] > out[j] {
			out[j-1], out[j] = out[j], out[j-1]
			j--
		}
	}
	return out
}
