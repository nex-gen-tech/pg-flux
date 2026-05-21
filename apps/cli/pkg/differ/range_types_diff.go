package differ

import (
	"fmt"

	"github.com/nex-gen-tech/pg-flux/pkg/plan"
	"github.com/nex-gen-tech/pg-flux/pkg/schema"
)

// diffRangeTypes emits DROP TYPE for range types removed from desired. Creates
// flow through ExtraDDL pass-through on first apply. Range types have no useful
// ALTER (subtype is fixed at creation), so any subtype change requires DROP+CREATE
// by the user.
func diffRangeTypes(d, l *schema.SchemaState) []change {
	var out []change
	if l == nil || len(l.RangeTypes) == 0 {
		return out
	}
	if d == nil {
		d = &schema.SchemaState{}
	}
	for k, lt := range l.RangeTypes {
		if lt == nil {
			continue
		}
		if _, ok := d.RangeTypes[k]; ok {
			continue
		}
		out = append(out, change{
			kind:   plan.ChangeRawSQL,
			rawSQL: fmt.Sprintf("DROP TYPE IF EXISTS %s.%s", ident(lt.Schema), ident(lt.Name)),
			tbl:    "rangetype/" + k,
		})
	}
	return out
}
