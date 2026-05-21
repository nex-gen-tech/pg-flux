package dump

import (
	"fmt"
	"sort"
	"strings"

	"github.com/nex-gen-tech/pg-flux/pkg/differ"
	"github.com/nex-gen-tech/pg-flux/pkg/schema"
)

// renderEnums emits CREATE TYPE ... AS ENUM (...). Values come from
// SchemaState.EnumValues (also populated by the inspector for live state).
func renderEnums(s *schema.SchemaState) []object {
	if s == nil || len(s.EnumValues) == 0 {
		return nil
	}
	keys := make([]string, 0, len(s.EnumValues))
	for k := range s.EnumValues {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out []object
	for _, k := range keys {
		values := s.EnumValues[k]
		// Key format: "schema.name"
		parts := strings.SplitN(k, ".", 2)
		var sch, name string
		if len(parts) == 2 {
			sch, name = parts[0], parts[1]
		} else {
			sch, name = "public", parts[0]
		}
		var b strings.Builder
		fmt.Fprintf(&b, "CREATE TYPE %s.%s AS ENUM (\n", differ.Ident(sch), differ.Ident(name))
		for i, v := range values {
			if i > 0 {
				b.WriteString(",\n")
			}
			fmt.Fprintf(&b, "  %s", quote(v))
		}
		b.WriteString("\n);\n")
		// UserTypes is a set; comments/owners for enums are not currently modeled
		// on a dedicated struct. If they appear in pg_description / pg_type, they'd
		// be picked up by a future enhancement.
		out = append(out, object{
			Kind: "types", Schema: sch, Name: name,
			SortKey: sortEnums, SQL: b.String(),
		})
	}
	return out
}

func renderDomains(s *schema.SchemaState) []object {
	if s == nil || len(s.Domains) == 0 {
		return nil
	}
	keys := make([]string, 0, len(s.Domains))
	for k := range s.Domains {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out []object
	for _, k := range keys {
		d := s.Domains[k]
		if d == nil {
			continue
		}
		var b strings.Builder
		fmt.Fprintf(&b, "CREATE DOMAIN %s.%s AS %s",
			differ.Ident(d.Schema), differ.Ident(d.Name), d.BaseType)
		for _, c := range d.Constraints {
			if c.Name != "" {
				fmt.Fprintf(&b, "\n  CONSTRAINT %s CHECK (%s)", differ.Ident(c.Name), c.Expr)
			} else {
				fmt.Fprintf(&b, "\n  CHECK (%s)", c.Expr)
			}
		}
		b.WriteString(";\n")
		if d.Owner != "" {
			fmt.Fprintf(&b, "ALTER DOMAIN %s.%s OWNER TO %s;\n",
				differ.Ident(d.Schema), differ.Ident(d.Name), differ.Ident(d.Owner))
		}
		out = append(out, object{
			Kind: "types", Schema: d.Schema, Name: d.Name,
			SortKey: sortDomains, SQL: b.String(),
		})
	}
	return out
}

func renderCompositeTypes(s *schema.SchemaState) []object {
	if s == nil || len(s.CompositeTypes) == 0 {
		return nil
	}
	keys := make([]string, 0, len(s.CompositeTypes))
	for k := range s.CompositeTypes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out []object
	for _, k := range keys {
		ct := s.CompositeTypes[k]
		if ct == nil {
			continue
		}
		var b strings.Builder
		fmt.Fprintf(&b, "CREATE TYPE %s.%s AS (\n",
			differ.Ident(ct.Schema), differ.Ident(ct.Name))
		for i, a := range ct.Attributes {
			if i > 0 {
				b.WriteString(",\n")
			}
			fmt.Fprintf(&b, "  %s %s", differ.Ident(a.Name), a.Type)
		}
		b.WriteString("\n);\n")
		if ct.Comment != "" {
			fmt.Fprintf(&b, "COMMENT ON TYPE %s.%s IS %s;\n",
				differ.Ident(ct.Schema), differ.Ident(ct.Name), quote(ct.Comment))
		}
		if ct.Owner != "" {
			fmt.Fprintf(&b, "ALTER TYPE %s.%s OWNER TO %s;\n",
				differ.Ident(ct.Schema), differ.Ident(ct.Name), differ.Ident(ct.Owner))
		}
		out = append(out, object{
			Kind: "types", Schema: ct.Schema, Name: ct.Name,
			SortKey: sortCompositeTypes, SQL: b.String(),
		})
	}
	return out
}

func renderRangeTypes(s *schema.SchemaState) []object {
	if s == nil || len(s.RangeTypes) == 0 {
		return nil
	}
	keys := make([]string, 0, len(s.RangeTypes))
	for k := range s.RangeTypes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out []object
	for _, k := range keys {
		rt := s.RangeTypes[k]
		if rt == nil {
			continue
		}
		var b strings.Builder
		fmt.Fprintf(&b, "CREATE TYPE %s.%s AS RANGE (SUBTYPE = %s",
			differ.Ident(rt.Schema), differ.Ident(rt.Name), rt.Subtype)
		for _, opt := range rt.Options {
			fmt.Fprintf(&b, ", %s", opt)
		}
		b.WriteString(");\n")
		if rt.Owner != "" {
			fmt.Fprintf(&b, "ALTER TYPE %s.%s OWNER TO %s;\n",
				differ.Ident(rt.Schema), differ.Ident(rt.Name), differ.Ident(rt.Owner))
		}
		out = append(out, object{
			Kind: "types", Schema: rt.Schema, Name: rt.Name,
			SortKey: sortRangeTypes, SQL: b.String(),
		})
	}
	return out
}
