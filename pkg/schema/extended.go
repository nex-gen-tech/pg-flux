package schema

import "strings"

// Index models a B-tree (or other) user index in desired or live state.
type Index struct {
	Schema, Name       string
	TableSchema, Table string
	CreateSQL          string // deparse of CREATE INDEX, or pg_get_indexdef
	Fingerprint        string // pg_query Fingerprint of normalized def (optional, filled by differ)
	Concurrent         bool
}

// Function models a simple SQL/PLpgSQL function (one identity per name+arg types in v1).
type Function struct {
	Schema   string
	Name     string
	Language string
	// Kind matches pg_proc.prokind: f=function, a=aggregate, w=window.
	Kind string
	// DefSQL is CREATE OR REPLACE from source or pg_get_functiondef; compared via fingerprint in differ.
	DefSQL      string
	Fingerprint string
	Identity    string // schema.name(args) for map key
}

// Policy is a row-level security policy.
type Policy struct {
	Schema, Table, Name string
	Cmd                 string
	Roles               []string
	UsingSQL, WithCheck string
	Permissive          bool
	DefSQL              string // for fingerprint / display
}

// IndexKey is schema-qualified index name in lower case.
func IndexKey(sch, name string) string {
	if sch == "" {
		sch = "public"
	}
	return TableKey(sch, strings.ToLower(name))
}

// FunctionKey is identity string for map lookup.
func FunctionKey(identity string) string {
	return strings.ToLower(identity)
}

// FunctionDependencyKey returns a coarse "schema.name" string (lowercased) taken from
// a full function identity (schema.name(args)...) for dependency edges where triggers
// reference EXECUTE FUNCTION name without a full arg list. If identity has no "(", the
// whole string is lowercased and returned.
func FunctionDependencyKey(identity string) string {
	identity = strings.TrimSpace(identity)
	if identity == "" {
		return ""
	}
	i := strings.IndexByte(identity, '(')
	if i <= 0 {
		return strings.ToLower(identity)
	}
	return strings.ToLower(identity[:i])
}

// PolicyKey is schemaname + tablename + policyname.
func PolicyKey(sch, rel, pol string) string {
	if sch == "" {
		sch = "public"
	}
	return sch + "." + strings.ToLower(rel) + "/" + pol
}
