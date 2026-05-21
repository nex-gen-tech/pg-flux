package codegen

import (
	"path"
	"strings"

	"github.com/nex-gen-tech/pg-flux/pkg/schema"
)

// Filter restricts which objects flow through the generator. Patterns use
// filepath.Match glob syntax: `*`, `?`, `[abc]`. Match is performed on the
// fully-qualified "schema.name" form. An empty filter matches everything.
type Filter struct {
	// IncludeTables — when non-empty, ONLY tables matching one of these
	// patterns are emitted (allowlist).
	IncludeTables []string
	// ExcludeTables — tables matching one of these are skipped (denylist).
	// Applied AFTER IncludeTables.
	ExcludeTables []string
	// ExcludeSchemas — every object in any matching schema is skipped.
	// Useful for hiding pg_flux's own tracking schema, partition children, etc.
	ExcludeSchemas []string
}

// Empty reports whether the filter would let everything through.
func (f Filter) Empty() bool {
	return len(f.IncludeTables) == 0 && len(f.ExcludeTables) == 0 && len(f.ExcludeSchemas) == 0
}

// ApplyToState returns a new SchemaState with non-matching objects removed.
// The input is not modified. Indexes / Triggers / Policies attached to a
// filtered-out table are also removed so the emitter doesn't dangle references.
func (f Filter) ApplyToState(s *schema.SchemaState) *schema.SchemaState {
	if f.Empty() || s == nil {
		return s
	}
	out := &schema.SchemaState{}
	// Tables.
	if s.Tables != nil {
		out.Tables = map[string]*schema.Table{}
		for k, t := range s.Tables {
			if t == nil {
				continue
			}
			if f.allows(t.Schema, t.Name) {
				out.Tables[k] = t
			}
		}
	}
	// Views — apply the same allow/deny rules; views often live in the same
	// schema as the tables they read.
	if s.Views != nil {
		out.Views = map[string]*schema.View{}
		for k, v := range s.Views {
			if v == nil {
				continue
			}
			if f.allowsSchema(v.Schema) {
				out.Views[k] = v
			}
		}
	}
	// Indexes — keep only those on a kept table.
	if s.Indexes != nil {
		out.Indexes = map[string]*schema.Index{}
		for k, ix := range s.Indexes {
			if ix == nil {
				continue
			}
			tk := schema.TableKey(ix.TableSchema, ix.Table)
			if _, ok := out.Tables[tk]; ok {
				out.Indexes[k] = ix
			}
		}
	}
	// Functions / Sequences / Triggers / Policies / etc. — schema-level filter only.
	if s.Functions != nil {
		out.Functions = map[string]*schema.Function{}
		for k, fn := range s.Functions {
			if fn == nil {
				continue
			}
			if f.allowsSchema(fn.Schema) {
				out.Functions[k] = fn
			}
		}
	}
	if s.Sequences != nil {
		out.Sequences = map[string]*schema.Sequence{}
		for k, sq := range s.Sequences {
			if sq == nil {
				continue
			}
			if f.allowsSchema(sq.Schema) {
				out.Sequences[k] = sq
			}
		}
	}
	if s.Triggers != nil {
		out.Triggers = map[string]*schema.Trigger{}
		for k, tr := range s.Triggers {
			if tr == nil {
				continue
			}
			tk := schema.TableKey(tr.Schema, tr.Table)
			if _, ok := out.Tables[tk]; ok {
				out.Triggers[k] = tr
			}
		}
	}
	if s.Policies != nil {
		out.Policies = map[string]*schema.Policy{}
		for k, p := range s.Policies {
			if p == nil {
				continue
			}
			tk := schema.TableKey(p.Schema, p.Table)
			if _, ok := out.Tables[tk]; ok {
				out.Policies[k] = p
			}
		}
	}
	// Types-family — schema-level filter.
	if s.EnumValues != nil {
		out.EnumValues = map[string][]string{}
		for k, v := range s.EnumValues {
			parts := strings.SplitN(k, ".", 2)
			schemaName := "public"
			if len(parts) == 2 {
				schemaName = parts[0]
			}
			if f.allowsSchema(schemaName) {
				out.EnumValues[k] = v
			}
		}
	}
	if s.Domains != nil {
		out.Domains = map[string]*schema.Domain{}
		for k, d := range s.Domains {
			if d == nil {
				continue
			}
			if f.allowsSchema(d.Schema) {
				out.Domains[k] = d
			}
		}
	}
	if s.CompositeTypes != nil {
		out.CompositeTypes = map[string]*schema.CompositeType{}
		for k, ct := range s.CompositeTypes {
			if ct == nil {
				continue
			}
			if f.allowsSchema(ct.Schema) {
				out.CompositeTypes[k] = ct
			}
		}
	}
	if s.RangeTypes != nil {
		out.RangeTypes = map[string]*schema.RangeType{}
		for k, rt := range s.RangeTypes {
			if rt == nil {
				continue
			}
			if f.allowsSchema(rt.Schema) {
				out.RangeTypes[k] = rt
			}
		}
	}
	// Pass-through for objects we never filter at this layer.
	out.Extensions = s.Extensions
	out.EventTriggers = s.EventTriggers
	out.Statistics = s.Statistics
	out.ForeignServers = s.ForeignServers
	out.ForeignTables = s.ForeignTables
	out.UserTypes = s.UserTypes
	return out
}

func (f Filter) allows(sch, name string) bool {
	if !f.allowsSchema(sch) {
		return false
	}
	qual := sch + "." + name
	if len(f.IncludeTables) > 0 {
		matched := false
		for _, pat := range f.IncludeTables {
			if matchPattern(pat, qual) || matchPattern(pat, name) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	for _, pat := range f.ExcludeTables {
		if matchPattern(pat, qual) || matchPattern(pat, name) {
			return false
		}
	}
	return true
}

func (f Filter) allowsSchema(sch string) bool {
	for _, pat := range f.ExcludeSchemas {
		if matchPattern(pat, sch) {
			return false
		}
	}
	return true
}

// matchPattern is path.Match with malformed patterns treated as literal equality.
// path.Match operates on filepath-like inputs; for our identifier strings the
// behaviour is the same as glob matching (`*`, `?`, character classes).
func matchPattern(pat, s string) bool {
	ok, err := path.Match(pat, s)
	if err != nil {
		return pat == s
	}
	return ok
}
