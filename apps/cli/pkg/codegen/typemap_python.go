package codegen

import "strings"

// PythonTypeMap implements TypeMap for Python (Pydantic v2).
// The Map method returns the base type expression; nullable wraps in Optional[T].
// imports returns the set of symbols that need to be imported from standard library
// modules (e.g. "UUID", "Decimal", "datetime", "Any").
type PythonTypeMap struct {
	// Overrides maps normalised PG type names → Python type expressions.
	Overrides map[string]string
	// CustomTypes maps PG user-defined type names (enums) to their generated
	// Python class name. Populated by the emitter before calling Map.
	// Lookups try both "schema.name" and bare "name".
	CustomTypes map[string]string
}

// Map returns (typeExpr, imports) for the given PG type.
// nullable=true wraps the base type in Optional[...].
// The returned imports slice contains Python symbol names that must appear in
// the `from <module> import ...` block (e.g. "Optional", "UUID").
func (m *PythonTypeMap) Map(pgType string, nullable bool) (string, []string) {
	base, imps := m.base(pgType)
	if nullable {
		imps = appendUniq(imps, "Optional")
		return "Optional[" + base + "]", imps
	}
	return base, imps
}

func (m *PythonTypeMap) base(pgType string) (string, []string) {
	if m.Overrides != nil {
		if v, ok := m.Overrides[normalizePGType(pgType)]; ok {
			return v, pythonImportsFor(v)
		}
	}
	t, isArray := stripPGType(pgType)

	// User-defined types (enums).
	if m.CustomTypes != nil {
		if name, ok := m.CustomTypes[t]; ok {
			if isArray {
				return "list[" + name + "]", nil
			}
			return name, nil
		}
		if dot := lastDot(t); dot >= 0 {
			if name, ok := m.CustomTypes[t[dot+1:]]; ok {
				if isArray {
					return "list[" + name + "]", nil
				}
				return name, nil
			}
		}
	}

	base, imps := pyBaseDefault(t)
	if isArray {
		base = "list[" + base + "]"
	}
	return base, imps
}

// pyBaseDefault returns the default Python type for a normalised PG base type.
func pyBaseDefault(pg string) (string, []string) {
	switch pg {
	case "text", "varchar", "character varying", "char", "character",
		"bpchar", "name", "citext", "tsvector", "interval",
		"inet", "cidr", "macaddr", "macaddr8":
		return "str", nil
	case "bigint", "int8", "int4", "int2", "smallint", "integer", "int",
		"smallserial", "serial2", "serial", "serial4", "bigserial", "serial8":
		return "int", nil
	case "real", "float4", "float8", "double precision", "double":
		return "float", nil
	case "numeric", "decimal":
		return "Decimal", []string{"Decimal"}
	case "boolean", "bool":
		return "bool", nil
	case "uuid":
		return "UUID", []string{"UUID"}
	case "timestamptz", "timestamp with time zone",
		"timestamp", "timestamp without time zone",
		"date",
		"time", "time without time zone", "timetz", "time with time zone":
		return "datetime", []string{"datetime"}
	case "json", "jsonb":
		return "dict[str, Any]", []string{"Any"}
	case "bytea":
		return "bytes", nil
	}
	return "Any", []string{"Any"}
}

// pythonImportsFor returns the Python imports needed for a literal override
// expression (used when the user supplies a custom type string via Overrides).
// This is best-effort: it checks for known symbols.
func pythonImportsFor(expr string) []string {
	var imps []string
	if strings.Contains(expr, "UUID") {
		imps = appendUniq(imps, "UUID")
	}
	if strings.Contains(expr, "Decimal") {
		imps = appendUniq(imps, "Decimal")
	}
	if strings.Contains(expr, "datetime") {
		imps = appendUniq(imps, "datetime")
	}
	if strings.Contains(expr, "Any") {
		imps = appendUniq(imps, "Any")
	}
	if strings.Contains(expr, "Optional") {
		imps = appendUniq(imps, "Optional")
	}
	return imps
}

// appendUniq appends v to slice only if it isn't already present.
func appendUniq(slice []string, v string) []string {
	for _, x := range slice {
		if x == v {
			return slice
		}
	}
	return append(slice, v)
}
