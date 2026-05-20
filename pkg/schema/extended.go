package schema

import "strings"

// Index models a B-tree (or other) user index in desired or live state.
type Index struct {
	Schema, Name       string
	TableSchema, Table string
	CreateSQL          string // deparse of CREATE INDEX, or pg_get_indexdef
	Fingerprint        string // pg_query Fingerprint of normalized def (optional, filled by differ)
	Concurrent         bool
	Comment            string
}

// Function models a simple SQL/PLpgSQL function (one identity per name+arg types in v1).
type Function struct {
	Schema   string
	Name     string
	Language string
	// Kind matches pg_proc.prokind: f=function, a=aggregate, w=window, p=procedure.
	Kind string
	// DefSQL is CREATE OR REPLACE from source or pg_get_functiondef; compared via fingerprint in differ.
	DefSQL      string
	Fingerprint string
	Identity    string // schema.name(args) for map key
	Comment     string
	Owner       string
	// Metadata fields (ALTER FUNCTION ... attribute):
	// Volatility: "IMMUTABLE" | "STABLE" | "VOLATILE" — from pg_proc.provolatile (i/s/v).
	Volatility string
	// Security: "DEFINER" | "INVOKER" — from pg_proc.prosecdef.
	Security string
	// Parallel: "SAFE" | "RESTRICTED" | "UNSAFE" — from pg_proc.proparallel (s/r/u).
	Parallel string
	// LeakProof: from pg_proc.proleakproof.
	LeakProof bool
	// Cost is the planner cost estimate (pg_proc.procost). Zero means "use default".
	Cost float64
	// Rows is the planner rows estimate for SETOF-returning functions (pg_proc.prorows).
	// Zero means "use default" (1000 for SETOF, 1 otherwise).
	Rows float64
	// Config holds SET clause entries (pg_proc.proconfig) like "search_path=public, pg_temp".
	Config []string
	// Privileges captures GRANT EXECUTE on this function (pg_proc.proacl).
	Privileges []Privilege

	// --- Structured signature (populated by inspector; empty when loaded from source) ---

	// Args lists IN / INOUT / VARIADIC parameters in source order. OUT-mode
	// args are split into ReturnsTable when the function uses RETURNS TABLE
	// or has bare OUT parameters.
	Args []FunctionArg
	// ReturnType is the formatted PG type returned by the function (e.g.
	// "integer", "public.user_role", "void"). Empty for procedures.
	ReturnType string
	// ReturnsSet is true for SETOF-returning functions.
	ReturnsSet bool
	// ReturnsTable, when non-empty, lists the OUT columns of a RETURNS TABLE
	// (...) function or the set of OUT args that synthesise a row type.
	ReturnsTable []FunctionArg
}

// FunctionArg is one parameter or return-table column of a function.
// Mode maps to pg_proc.proargmodes: "i" (IN, default), "o" (OUT), "b" (INOUT),
// "v" (VARIADIC), "t" (TABLE column — used in ReturnsTable).
type FunctionArg struct {
	Name string
	Type string
	Mode string
	// HasDefault is true when the parameter has a DEFAULT clause (codegen
	// emitters can mark these as optional).
	HasDefault bool
}

// Policy is a row-level security policy.
type Policy struct {
	Schema, Table, Name string
	Cmd                 string
	Roles               []string
	UsingSQL, WithCheck string
	Permissive          bool
	DefSQL              string // for fingerprint / display
	Comment             string
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
