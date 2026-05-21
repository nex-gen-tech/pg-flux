package differ

import (
	"fmt"

	"github.com/nex-gen-tech/pg-flux/pkg/plan"
	"github.com/nex-gen-tech/pg-flux/pkg/schema"
)

// diffSubscriptions emits ALTER SUBSCRIPTION for publication-list changes and
// enable/disable flips. Connection-info changes are not auto-diffed (security
// risk and rarely correct). CREATE via passthrough; DROP fires for live-only.
func diffSubscriptions(d, l *schema.SchemaState) []change {
	var out []change
	if d == nil {
		d = &schema.SchemaState{}
	}
	if l == nil {
		l = &schema.SchemaState{}
	}
	for k, ds := range d.Subscriptions {
		if ds == nil {
			continue
		}
		ls := l.Subscriptions[k]
		if ls == nil {
			continue
		}
		out = append(out, subscriptionAlters(ds, ls)...)
	}
	for k, ls := range l.Subscriptions {
		if ls == nil {
			continue
		}
		if _, ok := d.Subscriptions[k]; ok {
			continue
		}
		// Subscriptions must be DISABLED before they can be DROPPED (PG safety).
		out = append(out, change{
			kind:   plan.ChangeRawSQL,
			rawSQL: fmt.Sprintf("ALTER SUBSCRIPTION %s DISABLE", ident(ls.Name)),
			tbl:    "subscription/" + k,
		})
		out = append(out, change{
			kind:   plan.ChangeRawSQL,
			rawSQL: fmt.Sprintf("DROP SUBSCRIPTION IF EXISTS %s", ident(ls.Name)),
			tbl:    "subscription/" + k,
		})
	}
	return out
}

func subscriptionAlters(d, l *schema.Subscription) []change {
	var out []change
	name := ident(d.Name)
	if d.Enabled != l.Enabled {
		verb := "ENABLE"
		if !d.Enabled {
			verb = "DISABLE"
		}
		out = append(out, change{
			kind:   plan.ChangeRawSQL,
			rawSQL: fmt.Sprintf("ALTER SUBSCRIPTION %s %s", name, verb),
			tbl:    "subscription/" + d.Name,
		})
	}
	// Publication list diff
	dPubs := sortedCopy(d.Publications)
	lPubs := sortedCopy(l.Publications)
	add, drop := stringSetDiff(dPubs, lPubs)
	for _, p := range drop {
		out = append(out, change{
			kind:   plan.ChangeRawSQL,
			rawSQL: fmt.Sprintf("ALTER SUBSCRIPTION %s DROP PUBLICATION %s", name, ident(p)),
			tbl:    "subscription/" + d.Name,
		})
	}
	for _, p := range add {
		out = append(out, change{
			kind:   plan.ChangeRawSQL,
			rawSQL: fmt.Sprintf("ALTER SUBSCRIPTION %s ADD PUBLICATION %s", name, ident(p)),
			tbl:    "subscription/" + d.Name,
		})
	}
	return out
}
