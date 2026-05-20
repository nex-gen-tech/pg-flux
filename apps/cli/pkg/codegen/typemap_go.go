package codegen

import "strings"

// GoTypeMap implements TypeMap for Go, using stdlib types only by default.
// Custom mappings (e.g. github.com/shopspring/decimal.Decimal for numeric) are
// layered on via OverrideConfig.TypeOverrides — the user's choice wins.
type GoTypeMap struct {
	Overrides map[string]string // pg type → fully-qualified Go type (e.g. "time.Time" or "github.com/foo/bar.Baz")
	// CustomTypes maps PG user-defined type names (enums, composites, domains)
	// to their generated Go identifier — e.g. "public.user_role" → "UserRole".
	// Populated by the emitter from the SchemaState before Generate runs.
	// Lookups try both "schema.name" and bare "name" so columns that reference
	// an enum by its unqualified name resolve correctly.
	CustomTypes map[string]string
}

// Map returns (typeExpr, imports) for the PG type. The typeExpr is suitable
// for direct insertion into a struct field declaration. Nullable=true wraps
// the base type in a pointer.
func (m *GoTypeMap) Map(pgType string, nullable bool) (string, []string) {
	base, imps := m.base(pgType)
	if nullable {
		return "*" + base, imps
	}
	return base, imps
}

// base returns the non-nullable Go type for a PG type. Falls back to "string"
// for unknown types so the generator always produces compilable code even on
// exotic catalog content; user can fix via override config.
func (m *GoTypeMap) base(pgType string) (string, []string) {
	// User overrides first. The override value is a fully-qualified type;
	// the emitter splits package and base name.
	if m.Overrides != nil {
		if v, ok := m.Overrides[normalizePGType(pgType)]; ok {
			return resolveQualifiedType(v)
		}
	}
	// Strip size modifiers and array suffixes for matching: varchar(50) → varchar.
	// Arrays are tracked separately so we can build "[]T" at the end.
	t, isArray := stripPGType(pgType)
	// User-defined types (enums, composites, domains) resolve to the generated
	// Go type name. The schema-qualified form takes precedence so a column
	// like "public.user_role" matches; bare "user_role" works for unqualified
	// references.
	if m.CustomTypes != nil {
		if name, ok := m.CustomTypes[t]; ok {
			if isArray {
				return "[]" + name, nil
			}
			return name, nil
		}
		// Try bare-name lookup when t isn't already qualified.
		if dot := lastDot(t); dot >= 0 {
			if name, ok := m.CustomTypes[t[dot+1:]]; ok {
				if isArray {
					return "[]" + name, nil
				}
				return name, nil
			}
		}
	}
	base, imps := goBaseDefault(t)
	if isArray {
		base = "[]" + base
	}
	return base, imps
}

func lastDot(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '.' {
			return i
		}
	}
	return -1
}

// goBaseDefault returns the default stdlib Go type for a normalized PG type.
// Returns "string" as the safe-but-ugly fallback for unknown types.
func goBaseDefault(pg string) (string, []string) {
	switch pg {
	case "smallint", "int2":
		return "int16", nil
	case "integer", "int", "int4":
		return "int32", nil
	case "bigint", "int8":
		return "int64", nil
	case "smallserial", "serial2":
		return "int16", nil
	case "serial", "serial4":
		return "int32", nil
	case "bigserial", "serial8":
		return "int64", nil
	case "real", "float4":
		return "float32", nil
	case "double precision", "float8", "double":
		return "float64", nil
	case "boolean", "bool":
		return "bool", nil
	case "text", "varchar", "character varying", "char", "character", "name", "citext":
		return "string", nil
	case "bytea":
		return "[]byte", nil
	case "uuid":
		// Stdlib has no UUID type; surface as string by default. Users with
		// the google/uuid library override to "github.com/google/uuid.UUID".
		return "string", nil
	case "json", "jsonb":
		return "json.RawMessage", []string{"encoding/json"}
	case "timestamp", "timestamp without time zone", "timestamptz", "timestamp with time zone", "date", "time", "time without time zone", "timetz", "time with time zone":
		return "time.Time", []string{"time"}
	case "interval":
		return "time.Duration", []string{"time"}
	case "numeric", "decimal":
		// Conservative default: string preserves precision without a third-party dep.
		return "string", nil
	case "inet", "cidr", "macaddr", "macaddr8":
		return "string", nil
	}
	return "string", nil
}

// normalizePGType lowercases and strips precision modifiers / array suffix so
// "VARCHAR(50)" and "varchar" match the same override entry.
func normalizePGType(s string) string {
	t, _ := stripPGType(s)
	return t
}

// stripPGType lowercases, removes precision modifiers (foo(10), foo(10,2)), the
// leading "pg_catalog." schema prefix, and the trailing "[]" array marker.
// Returns (base type, isArray).
func stripPGType(s string) (string, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.TrimPrefix(s, "pg_catalog.")
	isArray := strings.HasSuffix(s, "[]")
	s = strings.TrimSuffix(s, "[]")
	if i := strings.IndexByte(s, '('); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	return s, isArray
}

// resolveQualifiedType splits "github.com/foo/bar.Baz" into ("bar.Baz", ["github.com/foo/bar"]).
// A bare name like "string" returns ("string", nil).
func resolveQualifiedType(s string) (string, []string) {
	dot := strings.LastIndexByte(s, '.')
	if dot < 0 {
		return s, nil
	}
	slash := strings.LastIndexByte(s[:dot], '/')
	if slash < 0 {
		// "time.Time" → ("time.Time", ["time"])
		return s, []string{s[:dot]}
	}
	// "github.com/foo/bar.Baz" → import "github.com/foo/bar"; type "bar.Baz".
	imp := s[:dot]
	pkg := s[slash+1 : dot]
	return pkg + "." + s[dot+1:], []string{imp}
}
