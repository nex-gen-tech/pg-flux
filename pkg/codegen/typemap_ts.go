package codegen

// TSTypeMap implements TypeMap for TypeScript. TS has no package-level imports
// for primitive types, so the imports slot is always nil — kept for interface
// compatibility with the Go side.
type TSTypeMap struct {
	Overrides   map[string]string // pg type → TS type expression
	CustomTypes map[string]string // PG user-defined types → generated TS identifier (see GoTypeMap.CustomTypes)
}

func (m *TSTypeMap) Map(pgType string, nullable bool) (string, []string) {
	base := m.base(pgType)
	if nullable {
		return base + " | null", nil
	}
	return base, nil
}

func (m *TSTypeMap) base(pgType string) string {
	if m.Overrides != nil {
		if v, ok := m.Overrides[normalizePGType(pgType)]; ok {
			return v
		}
	}
	t, isArray := stripPGType(pgType)
	if m.CustomTypes != nil {
		if name, ok := m.CustomTypes[t]; ok {
			if isArray {
				return name + "[]"
			}
			return name
		}
		if dot := lastDot(t); dot >= 0 {
			if name, ok := m.CustomTypes[t[dot+1:]]; ok {
				if isArray {
					return name + "[]"
				}
				return name
			}
		}
	}
	base := tsBaseDefault(t)
	if isArray {
		base = base + "[]"
	}
	return base
}

// tsBaseDefault is the same idea as goBaseDefault but for TS native types.
// Default fall-through is "string" because TS treats unknown columns as
// strings until proven otherwise — safe and ESLint-friendly.
func tsBaseDefault(pg string) string {
	switch pg {
	case "smallint", "int2", "integer", "int", "int4", "real", "float4",
		"double precision", "float8", "double", "smallserial", "serial2",
		"serial", "serial4":
		return "number"
	case "bigint", "int8", "bigserial", "serial8":
		return "bigint"
	case "boolean", "bool":
		return "boolean"
	case "text", "varchar", "character varying", "char", "character", "name", "citext", "uuid":
		return "string"
	case "bytea":
		return "Uint8Array"
	case "json", "jsonb":
		return "unknown"
	case "timestamp", "timestamp without time zone", "timestamptz",
		"timestamp with time zone", "date", "time", "time without time zone",
		"timetz", "time with time zone":
		return "Date"
	case "interval":
		// No native interval type in JS; users override to a library type
		// or keep as string (PG-formatted "1 day 02:03:04").
		return "string"
	case "numeric", "decimal":
		return "string"
	case "inet", "cidr", "macaddr", "macaddr8":
		return "string"
	}
	return "string"
}
