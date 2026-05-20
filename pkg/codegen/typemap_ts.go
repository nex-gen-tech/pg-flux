package codegen

// TSTypeMap implements TypeMap for TypeScript. TS has no package-level imports
// for primitive types, so the imports slot is always nil — kept for interface
// compatibility with the Go side.
type TSTypeMap struct {
	Overrides   map[string]string // pg type → TS type expression
	CustomTypes map[string]string // PG user-defined types → generated TS identifier (see GoTypeMap.CustomTypes)
	// BigintAs overrides the default mapping for bigint / int8 / bigserial.
	// "bigint" (default), "number", "string".
	BigintAs string
	// DateAs overrides the default mapping for timestamp* / date / time.
	// "Date" (default), "string", "temporal".
	DateAs string
}

func (m *TSTypeMap) Map(pgType string, nullable bool) (string, []string) {
	base := m.base(pgType)
	if nullable {
		// NullStyle is applied at the emitter level (because "optional" needs
		// the `?:` to land on the property, not the type). Here we always
		// return the union form; the TS emitter rewrites if needed.
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
	base := m.tsBaseDefault(t)
	if isArray {
		base = base + "[]"
	}
	return base
}

// tsBaseDefault is the same idea as goBaseDefault but for TS native types.
// bigintAs and dateAs honour the user's TS preference; default fall-through is
// "string" for unknown PG types — safe and ESLint-friendly.
func (m *TSTypeMap) tsBaseDefault(pg string) string {
	bigintAs := m.BigintAs
	if bigintAs == "" {
		bigintAs = "bigint"
	}
	dateAs := m.DateAs
	if dateAs == "" {
		dateAs = "Date"
	}
	switch pg {
	case "smallint", "int2", "integer", "int", "int4", "real", "float4",
		"double precision", "float8", "double", "smallserial", "serial2",
		"serial", "serial4":
		return "number"
	case "bigint", "int8", "bigserial", "serial8":
		return bigintAs
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
		return tsDateExpr(dateAs)
	case "interval":
		return "string"
	case "numeric", "decimal":
		return "string"
	case "inet", "cidr", "macaddr", "macaddr8":
		return "string"
	}
	return "string"
}

// tsDateExpr returns the TS expression for the configured date_as mode.
func tsDateExpr(dateAs string) string {
	switch dateAs {
	case "string":
		return "string"
	case "temporal":
		return "Temporal.Instant"
	}
	return "Date"
}
