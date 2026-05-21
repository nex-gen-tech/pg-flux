package differ

import (
	"fmt"
	"sort"
	"strings"

	"github.com/nex-gen-tech/pg-flux/pkg/plan"
	"github.com/nex-gen-tech/pg-flux/pkg/schema"
	"github.com/nex-gen-tech/pg-flux/pkg/src"
)

// diffEventTriggers compares desired vs live event triggers.
//   - Present in desired, absent in live → CREATE EVENT TRIGGER
//   - Present in live, absent in desired → DROP EVENT TRIGGER
//   - Both present, definitions match → no-op
//   - Both present, definitions differ → DROP + CREATE (PG has no ALTER for fn/event/tags)
//
// Event triggers are non-disruptive metadata so no hazards are emitted.
func diffEventTriggers(d, l *schema.SchemaState) []change {
	var out []change
	if d == nil {
		d = &schema.SchemaState{}
	}
	if l == nil {
		l = &schema.SchemaState{}
	}
	dKeys := make([]string, 0, len(d.EventTriggers))
	for k := range d.EventTriggers {
		dKeys = append(dKeys, k)
	}
	sort.Strings(dKeys)
	for _, k := range dKeys {
		dEt := d.EventTriggers[k]
		lEt := l.EventTriggers[k]
		if lEt == nil {
			out = append(out, change{
				kind:   plan.ChangeRawSQL,
				rawSQL: src.RenderCreateEventTriggerSQL(dEt),
				tbl:    "event_trigger/" + k,
			})
			continue
		}
		if !eventTriggersEqual(dEt, lEt) {
			out = append(out, change{
				kind:   plan.ChangeRawSQL,
				rawSQL: fmt.Sprintf("DROP EVENT TRIGGER IF EXISTS %s", dEt.Name),
				tbl:    "event_trigger/" + k,
			})
			out = append(out, change{
				kind:   plan.ChangeRawSQL,
				rawSQL: src.RenderCreateEventTriggerSQL(dEt),
				tbl:    "event_trigger/" + k,
			})
		}
	}
	for k, lEt := range l.EventTriggers {
		if lEt == nil {
			continue
		}
		if _, ok := d.EventTriggers[k]; ok {
			continue
		}
		out = append(out, change{
			kind:   plan.ChangeRawSQL,
			rawSQL: fmt.Sprintf("DROP EVENT TRIGGER IF EXISTS %s", lEt.Name),
			tbl:    "event_trigger/" + k,
		})
	}
	return out
}

// eventTriggersEqual compares two event triggers for semantic equality. Order of
// Tags is normalized.
func eventTriggersEqual(a, b *schema.EventTrigger) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.Event != b.Event {
		return false
	}
	if !strings.EqualFold(a.Function, b.Function) {
		return false
	}
	at := append([]string(nil), a.Tags...)
	bt := append([]string(nil), b.Tags...)
	sort.Strings(at)
	sort.Strings(bt)
	if len(at) != len(bt) {
		return false
	}
	for i := range at {
		if at[i] != bt[i] {
			return false
		}
	}
	return true
}
