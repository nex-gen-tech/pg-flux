package schema

import "strings"

// Extension is an installed extension (CREATE EXTENSION) or live row from pg_extension.
type Extension struct {
	Name   string
	DefSQL string
	// Version is the pinned / desired version from WITH VERSION, or the installed extversion from the catalog.
	Version string
}

// ExtensionKey lowercases the extension name for map lookup.
func ExtensionKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// MiscObject records catalog objects that are loaded for awareness/drift messaging but not fully migrated.
type MiscObject struct {
	Kind string // e.g. FDW, EVENT_TRIGGER, PUBLICATION, SUBSCRIPTION, OPERATOR_CLASS, STATISTICS
	Name string
	// DefSQL is optional deparsed or original DDL.
	DefSQL string
}
