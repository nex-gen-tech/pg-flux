package codegen

import (
	"fmt"
	"strings"

	"github.com/nex-gen-tech/pg-flux/pkg/schema"
)

// emitZodValidators renders a parallel validators.ts module with z.object
// schemas matching every table, enum, and composite type. The schemas honour
// bigint_as / date_as so the runtime shapes match the static types. Optional
// — users opt in via validators: zod.
func emitZodValidators(s *schema.SchemaState, opts Options, g *TSGenerator) string {
	var b strings.Builder
	b.WriteString("import { z } from \"zod\";\n\n")

	// Enums: z.enum([...]).
	for _, k := range SortedKeys(s.EnumValues) {
		values := s.EnumValues[k]
		parts := strings.SplitN(k, ".", 2)
		var sch, name string
		if len(parts) == 2 {
			sch, name = parts[0], parts[1]
		} else {
			sch, name = "public", parts[0]
		}
		typeName := g.typeName(sch, name)
		quoted := make([]string, len(values))
		for i, v := range values {
			quoted[i] = fmt.Sprintf("%q", v)
		}
		fmt.Fprintf(&b, "export const %sSchema = z.enum([%s]);\n",
			typeName, strings.Join(quoted, ", "))
	}
	if len(s.EnumValues) > 0 {
		b.WriteString("\n")
	}

	// Composite types: z.object({...}).
	for _, k := range SortedKeys(s.CompositeTypes) {
		ct := s.CompositeTypes[k]
		if ct == nil {
			continue
		}
		typeName := g.typeName(ct.Schema, ct.Name)
		fmt.Fprintf(&b, "export const %sSchema = z.object({\n", typeName)
		for _, a := range ct.Attributes {
			key := opts.Emit.ApplyColumnCase(a.Name)
			zodType := zodPrimitive(a.Type, opts.Emit.BigintAs, opts.Emit.DateAs)
			fmt.Fprintf(&b, "  %s: %s,\n", key, zodType)
		}
		b.WriteString("});\n\n")
	}

	// Tables: z.object({...}).
	for _, k := range SortedKeys(s.Tables) {
		t := s.Tables[k]
		if t == nil {
			continue
		}
		typeName := g.typeName(t.Schema, t.Name)
		fmt.Fprintf(&b, "export const %sSchema = z.object({\n", typeName)
		for _, c := range t.Columns {
			if c == nil {
				continue
			}
			key := opts.Emit.ApplyColumnCase(c.Name)
			zodType := zodForColumn(c, opts, g)
			fmt.Fprintf(&b, "  %s: %s,\n", key, zodType)
		}
		b.WriteString("});\n\n")
	}
	return b.String()
}

// zodForColumn computes the zod expression for a single column. Prefers a
// known generated schema (e.g. UserRoleSchema for an enum-typed column) even
// when the column has a `tstype=` hint, since the runtime shape is still the
// enum. Falls back to z.any() only when the underlying type is truly opaque.
func zodForColumn(c *schema.Column, opts Options, g *TSGenerator) string {
	t, isArray := stripPGType(c.TypeSQL)
	// Custom-type reference: enums/composites have a generated <Name>Schema.
	if ttm, ok := opts.TypeMap.(*TSTypeMap); ok && ttm.CustomTypes != nil {
		if customName, hit := ttm.CustomTypes[t]; hit {
			expr := customName + "Schema"
			if isArray {
				expr = "z.array(" + expr + ")"
			}
			if !c.NotNull {
				expr += ".nullable()"
			}
			return expr
		}
	}
	// tstype hint without a known PG type: opaque schema with a TODO so the
	// user can fill it in. Domains over standard PG types are handled below.
	if hints := ParseCommentHints(c.Comment); hints.Overrides["tstype"] != "" {
		expr := "z.any() /* TODO: tstype hint, no schema generated */"
		if !c.NotNull {
			expr += ".nullable()"
		}
		return expr
	}
	base := zodPrimitive(t, opts.Emit.BigintAs, opts.Emit.DateAs)
	if isArray {
		base = "z.array(" + base + ")"
	}
	if !c.NotNull {
		base += ".nullable()"
	}
	return base
}

// zodPrimitive maps a PG type to its zod primitive expression, respecting the
// user's bigint and date conventions.
func zodPrimitive(pgType, bigintAs, dateAs string) string {
	t, _ := stripPGType(pgType)
	switch t {
	case "smallint", "int2", "integer", "int", "int4", "real", "float4",
		"double precision", "float8", "double", "smallserial", "serial2",
		"serial", "serial4":
		return "z.number()"
	case "bigint", "int8", "bigserial", "serial8":
		switch bigintAs {
		case "number":
			return "z.number()"
		case "string":
			return "z.string()"
		}
		return "z.bigint()"
	case "boolean", "bool":
		return "z.boolean()"
	case "text", "varchar", "character varying", "char", "character", "name", "citext", "uuid":
		if t == "uuid" {
			return "z.string().uuid()"
		}
		return "z.string()"
	case "bytea":
		return "z.instanceof(Uint8Array)"
	case "json", "jsonb":
		return "z.unknown()"
	case "timestamp", "timestamp without time zone", "timestamptz",
		"timestamp with time zone", "date", "time", "time without time zone",
		"timetz", "time with time zone":
		switch dateAs {
		case "string":
			return "z.string()"
		case "temporal":
			return "z.any()" // Temporal isn't zod-native; user can override
		}
		return "z.date()"
	case "interval":
		return "z.string()"
	case "numeric", "decimal":
		return "z.string()"
	case "inet", "cidr", "macaddr", "macaddr8":
		return "z.string()"
	}
	return "z.string()"
}
