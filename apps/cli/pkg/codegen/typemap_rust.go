package codegen

import "strings"

// RustTypeMap implements TypeMap for Rust (sqlx + serde).
// Map returns fully-qualified type expressions (e.g. chrono::DateTime<chrono::Utc>)
// so generated files do not require extra `use` declarations for field types.
// The `imports` slice returns crate names that the caller must list in Cargo.toml
// (e.g. "uuid", "chrono", "serde_json").
type RustTypeMap struct {
	// Overrides maps normalised PG type names → Rust type expressions.
	Overrides map[string]string
	// CustomTypes maps PG user-defined type names (enums, composites, domains)
	// to their generated Rust identifier. Populated by the emitter.
	CustomTypes map[string]string
}

// Map returns (typeExpr, cargoFeatures) for the given PG type.
// nullable=true wraps the result in Option<T>.
func (m *RustTypeMap) Map(pgType string, nullable bool) (string, []string) {
	base, imps := m.base(pgType)
	if nullable {
		return "Option<" + base + ">", imps
	}
	return base, imps
}

func (m *RustTypeMap) base(pgType string) (string, []string) {
	if m.Overrides != nil {
		if v, ok := m.Overrides[normalizePGType(pgType)]; ok {
			return v, rustImportsFor(v)
		}
	}
	t, isArray := stripPGType(pgType)

	// User-defined types (enums, composites, domains).
	if m.CustomTypes != nil {
		if name, ok := m.CustomTypes[t]; ok {
			if isArray {
				return "Vec<" + name + ">", nil
			}
			return name, nil
		}
		if dot := lastDot(t); dot >= 0 {
			if name, ok := m.CustomTypes[t[dot+1:]]; ok {
				if isArray {
					return "Vec<" + name + ">", nil
				}
				return name, nil
			}
		}
	}

	base, imps := rustBaseDefault(t)
	if isArray {
		base = "Vec<" + base + ">"
	}
	return base, imps
}

// rustBaseDefault returns the default Rust type for a normalised PG base type.
// Fully-qualified paths are used so no `use` imports are needed in generated files.
func rustBaseDefault(pg string) (string, []string) {
	switch pg {
	case "smallint", "int2", "smallserial", "serial2":
		return "i16", nil
	case "integer", "int", "int4", "serial", "serial4":
		return "i32", nil
	case "bigint", "int8", "bigserial", "serial8":
		return "i64", nil
	case "real", "float4":
		return "f32", nil
	case "double precision", "float8", "double":
		return "f64", nil
	case "boolean", "bool":
		return "bool", nil
	case "text", "varchar", "character varying", "char", "character",
		"name", "citext", "bpchar",
		"inet", "cidr", "macaddr", "macaddr8",
		"tsvector", "interval":
		return "String", nil
	case "numeric", "decimal":
		// String is the safe default; users override to rust_decimal::Decimal
		// via type_overrides in config if they have that crate.
		return "String", nil
	case "bytea":
		return "Vec<u8>", nil
	case "uuid":
		return "uuid::Uuid", []string{"uuid"}
	case "json", "jsonb":
		return "serde_json::Value", []string{"serde_json"}
	case "timestamptz", "timestamp with time zone":
		return "chrono::DateTime<chrono::Utc>", []string{"chrono"}
	case "timestamp", "timestamp without time zone":
		return "chrono::NaiveDateTime", []string{"chrono"}
	case "date":
		return "chrono::NaiveDate", []string{"chrono"}
	case "time", "time without time zone":
		return "chrono::NaiveTime", []string{"chrono"}
	case "timetz", "time with time zone":
		return "chrono::DateTime<chrono::Utc>", []string{"chrono"}
	}
	return "String", nil
}

// rustImportsFor inspects an override expression for known crate prefixes.
func rustImportsFor(expr string) []string {
	var imps []string
	if strings.Contains(expr, "uuid::") {
		imps = appendUniq(imps, "uuid")
	}
	if strings.Contains(expr, "chrono::") {
		imps = appendUniq(imps, "chrono")
	}
	if strings.Contains(expr, "serde_json::") {
		imps = appendUniq(imps, "serde_json")
	}
	return imps
}
