package differ

import (
	"fmt"

	"github.com/nex-gen-tech/pg-flux/pkg/plan"
	"github.com/nex-gen-tech/pg-flux/pkg/schema"
)

// diffCompositeTypes emits per-attribute ALTER TYPE statements for composite types
// that exist on both sides but differ in attributes, plus DROP TYPE for types only
// in live. CREATE TYPE on first apply flows through diffExtraDDL pass-through and
// is suppressed here.
func diffCompositeTypes(d, l *schema.SchemaState) []change {
	var out []change
	if d == nil {
		d = &schema.SchemaState{}
	}
	if l == nil {
		l = &schema.SchemaState{}
	}
	for k, dc := range d.CompositeTypes {
		if dc == nil {
			continue
		}
		lc := l.CompositeTypes[k]
		if lc == nil {
			// CREATE handled by passthrough; nothing to do.
			continue
		}
		out = append(out, diffCompositeAttrs(dc, lc)...)
	}
	for k, lc := range l.CompositeTypes {
		if lc == nil {
			continue
		}
		if _, ok := d.CompositeTypes[k]; ok {
			continue
		}
		out = append(out, change{
			kind:   plan.ChangeRawSQL,
			rawSQL: fmt.Sprintf("DROP TYPE IF EXISTS %s.%s", ident(lc.Schema), ident(lc.Name)),
			tbl:    "composite/" + k,
		})
	}
	return out
}

// diffCompositeAttrs compares attribute lists and emits the minimal ALTER TYPE
// statements: ADD ATTRIBUTE for new, DROP ATTRIBUTE for missing, ALTER ATTRIBUTE
// for type change at the same position. No rename detection — name change appears
// as DROP+ADD (which is what PG would also produce without a RENAME hint).
func diffCompositeAttrs(d, l *schema.CompositeType) []change {
	var out []change
	lidx := make(map[string]int, len(l.Attributes))
	for i, a := range l.Attributes {
		lidx[a.Name] = i
	}
	didx := make(map[string]int, len(d.Attributes))
	for i, a := range d.Attributes {
		didx[a.Name] = i
	}
	qual := fmt.Sprintf("%s.%s", ident(d.Schema), ident(d.Name))
	// DROP first to leave the list compact.
	for _, la := range l.Attributes {
		if _, ok := didx[la.Name]; !ok {
			out = append(out, change{
				kind:   plan.ChangeRawSQL,
				rawSQL: fmt.Sprintf("ALTER TYPE %s DROP ATTRIBUTE %s", qual, ident(la.Name)),
				tbl:    "composite/" + d.Schema + "." + d.Name,
			})
		}
	}
	for _, da := range d.Attributes {
		li, ok := lidx[da.Name]
		if !ok {
			out = append(out, change{
				kind:   plan.ChangeRawSQL,
				rawSQL: fmt.Sprintf("ALTER TYPE %s ADD ATTRIBUTE %s %s", qual, ident(da.Name), da.Type),
				tbl:    "composite/" + d.Schema + "." + d.Name,
			})
			continue
		}
		// Normalize types so varchar(10) ≡ pg_catalog.varchar(10) ≡ character varying(10).
		if normType(da.Type) != normType(l.Attributes[li].Type) {
			out = append(out, change{
				kind:   plan.ChangeRawSQL,
				rawSQL: fmt.Sprintf("ALTER TYPE %s ALTER ATTRIBUTE %s SET DATA TYPE %s", qual, ident(da.Name), da.Type),
				tbl:    "composite/" + d.Schema + "." + d.Name,
			})
		}
	}
	return out
}
