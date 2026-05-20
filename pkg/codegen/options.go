package codegen

import "strings"

// EmitOptions controls every per-emitter behaviour that doesn't belong on
// the global Options struct. Most options apply to one language but a few are
// language-neutral (Layout, ColumnCase, Filter). Unset zero values fall back
// to sensible defaults — code does NOT need to set every field.
type EmitOptions struct {
	// --- Language-neutral ---

	// Layout selects how generated files are organised on disk:
	//   per-kind  (default): tables.go / enums.go / types.go / views.go
	//   per-object         : one file per object (users.go, orders.go, ...)
	//   single             : all output in one file (db.go / db.ts)
	Layout Layout
	// ColumnCase controls how source column names map to field/property names:
	//   "snake" → email_address (verbatim; matches PG conventions)
	//   "camel" → emailAddress  (JS convention)
	//   "pascal"→ EmailAddress  (Go convention; ignored by emitters that
	//                            already PascalCase fields like Go)
	// For Go, this affects the `db` and `json` struct tag value, not the
	// Go field name (which is always PascalCase).
	ColumnCase ColumnCase
	// Readonly marks columns whose underlying values are written only by the
	// database (identity, generated, defaulted). The marker is:
	//   - TS:  prefix the field with `readonly`
	//   - Go:  emit a "// readonly: ..." comment (no language keyword)
	// Accepted values: identity, generated, defaults, all, none (default).
	Readonly ReadonlyPolicy
	// InsertUpdateHelpers adds Insert<T> and Update<T> partial helper types
	// (TS only) so callers have ergonomic types for write paths.
	InsertUpdateHelpers bool
	// BrandedIDs emits each PRIMARY KEY column type as a branded type:
	//   type UserId = bigint & { readonly __brand: 'UserId' }
	// (TS only.) Prevents accidentally passing a PostId where a UserId is
	// expected. No-op when the table has a composite PK.
	BrandedIDs bool
	// FilePerKind / FilePerObject layout selectors (Layout above is the
	// source of truth; these convenience fields are derived).

	// --- TypeScript-specific ---

	// BigintAs controls how bigint / int8 / bigserial map in TS:
	//   "bigint" (default) — native bigint, must be serialised carefully
	//   "number"           — JS number; loses precision above 2^53
	//   "string"           — string preserves precision through JSON
	BigintAs string
	// DateAs controls how timestamp* / date / time map in TS:
	//   "Date" (default)  — JS Date
	//   "string"          — ISO 8601 string (lossless for JSON APIs)
	//   "temporal"        — Temporal.Instant / PlainDate (stage-3 proposal)
	DateAs string
	// NullStyle controls how nullable columns are spelled:
	//   "union" (default) — field: T | null
	//   "undefined"       — field: T | undefined
	//   "optional"        — field?: T
	NullStyle string
	// EnumStyle controls how PG enums are emitted in TS:
	//   "union"        (default) — string literal union: type Role = "a" | "b"
	//   "const-object"           — `as const` object + derived type
	//   "ts-enum"                — TypeScript enum keyword (runtime values)
	EnumStyle string
	// Validators selects a runtime validation library to emit schemas for:
	//   ""    (default; no schemas)
	//   "zod" — z.object(...) schemas in validators.ts
	Validators string

	// --- Go-specific ---

	// ORMTags adds tag flavours for an ORM beyond the default `db` and `json`.
	//   ""    (default) — only db + json
	//   "sqlx"          — only db (no json); sqlx is the most stdlib-y
	//   "gorm"          — adds gorm tags: primaryKey, autoCreateTime, default:...
	//   "bun"           — adds bun tags
	//   "ent"           — minimal ent compatibility
	ORMTags string
	// OmitEmpty controls which json struct tags get `,omitempty`:
	//   ""        (default) — no omitempty
	//   "nullable"          — only nullable columns
	//   "defaults"          — nullable + defaulted columns
	//   "all"               — every column
	OmitEmpty string

	// --- Filtering ---

	// Filter restricts which objects make it into the output (per emitter).
	Filter Filter

	// --- Per-object overrides ---

	// JSONShapes maps "schema.table.column" → TS type expression for jsonb
	// columns whose runtime shape is known. Replaces `unknown` with the
	// declared type. Go side ignores this since json.RawMessage is generic.
	JSONShapes map[string]string
}

// Layout is the file-organisation strategy for generator output.
type Layout string

const (
	LayoutPerKind   Layout = "per-kind"   // default
	LayoutPerObject Layout = "per-object" // one file per object
	LayoutSingle    Layout = "single"     // single file
)

// ColumnCase selects the source-column → field-key naming convention.
type ColumnCase string

const (
	ColumnCaseSnake  ColumnCase = "snake"  // verbatim (default)
	ColumnCaseCamel  ColumnCase = "camel"  // emailAddress
	ColumnCasePascal ColumnCase = "pascal" // EmailAddress
)

// ReadonlyPolicy selects which columns get marked readonly.
type ReadonlyPolicy string

const (
	ReadonlyNone      ReadonlyPolicy = "none" // default
	ReadonlyIdentity  ReadonlyPolicy = "identity"
	ReadonlyGenerated ReadonlyPolicy = "generated"
	ReadonlyDefaults  ReadonlyPolicy = "defaults"
	ReadonlyAll       ReadonlyPolicy = "all"
)

// normalize fills in defaults for unset fields.
func (o *EmitOptions) normalize() {
	if o.Layout == "" {
		o.Layout = LayoutPerKind
	}
	if o.ColumnCase == "" {
		o.ColumnCase = ColumnCaseSnake
	}
	if o.Readonly == "" {
		o.Readonly = ReadonlyNone
	}
	if o.BigintAs == "" {
		o.BigintAs = "bigint"
	}
	if o.DateAs == "" {
		o.DateAs = "Date"
	}
	if o.NullStyle == "" {
		o.NullStyle = "union"
	}
	if o.EnumStyle == "" {
		o.EnumStyle = "union"
	}
}

// ApplyColumnCase rewrites a snake_case column name per the selected convention.
// Always works on the original input; emitters call this once per column to get
// the field-key value.
func (o EmitOptions) ApplyColumnCase(s string) string {
	switch o.ColumnCase {
	case ColumnCaseCamel:
		return CamelCase(s)
	case ColumnCasePascal:
		return PascalCase(s)
	}
	return s
}

// jsonShapeFor returns the user-declared TS type for a specific column, or
// empty when the column has no shape override.
func (o EmitOptions) jsonShapeFor(schema, table, column string) string {
	if o.JSONShapes == nil {
		return ""
	}
	keys := []string{
		schema + "." + table + "." + column,
		table + "." + column,
	}
	for _, k := range keys {
		if v, ok := o.JSONShapes[strings.ToLower(k)]; ok {
			return v
		}
		if v, ok := o.JSONShapes[k]; ok {
			return v
		}
	}
	return ""
}
