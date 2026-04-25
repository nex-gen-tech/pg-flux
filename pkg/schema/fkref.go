package schema

import (
	"regexp"
	"strings"
)

var reFKReference = regexp.MustCompile(`(?i)REFERENCES\s+` +
	`(?P<ref>(?:"?[\w.]+"?\s*\.\s*)?"?[\w.]+"?)`)

// ReferenceTableKeyFromDefSQL returns "schema.table" of the referenced relation in a
// FOREIGN KEY … REFERENCES … fragment, or "" if not found.
func ReferenceTableKeyFromDefSQL(def string) string {
	if def == "" {
		return ""
	}
	m := reFKReference.FindStringSubmatch(def)
	if m == nil || len(m) < 2 {
		return ""
	}
	raw := strings.TrimSpace(strings.ToLower(m[1]))
	raw = strings.ReplaceAll(raw, `"`, "")
	parts := strings.Split(raw, ".")
	if len(parts) == 1 {
		return TableKey("public", parts[0])
	}
	if len(parts) == 2 {
		return TableKey(parts[0], parts[1])
	}
	return ""
}
