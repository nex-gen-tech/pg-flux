package src

import (
	"fmt"
	"strings"

	pgq "github.com/pganalyze/pg_query_go/v6"

	"github.com/nex-gen-tech/pg-flux/pkg/schema"
)

// captureCreateEventTrigger parses CREATE EVENT TRIGGER ... and registers it on
// SchemaState.EventTriggers. Database-wide objects, not schema-qualified.
func captureCreateEventTrigger(s *pgq.CreateEventTrigStmt, st *schema.SchemaState) error {
	if s == nil || st == nil {
		return nil
	}
	if st.EventTriggers == nil {
		st.EventTriggers = make(map[string]*schema.EventTrigger)
	}
	et := &schema.EventTrigger{
		Name:  strings.ToLower(s.GetTrigname()),
		Event: strings.ToLower(s.GetEventname()),
	}
	// Function reference: List of String parts → schema.name.
	parts := s.GetFuncname()
	switch len(parts) {
	case 1:
		et.Function = "public." + strings.ToLower(parts[0].GetString_().GetSval()) + "()"
	case 2:
		et.Function = strings.ToLower(parts[0].GetString_().GetSval()) + "." +
			strings.ToLower(parts[1].GetString_().GetSval()) + "()"
	}
	// WHEN clauses: list of DefElem with defname="tag"/value=List of strings.
	for _, w := range s.GetWhenclause() {
		el := w.GetDefElem()
		if el == nil {
			continue
		}
		if strings.ToLower(el.GetDefname()) != "tag" {
			continue
		}
		lst := el.GetArg().GetList()
		if lst == nil {
			continue
		}
		for _, it := range lst.GetItems() {
			if str := it.GetString_(); str != nil {
				et.Tags = append(et.Tags, str.GetSval())
			}
		}
	}
	st.EventTriggers[et.Name] = et
	return nil
}

// RenderCreateEventTriggerSQL produces a CREATE EVENT TRIGGER statement for the
// given model record. Exported so the differ can call it.
func RenderCreateEventTriggerSQL(et *schema.EventTrigger) string {
	var b strings.Builder
	fmt.Fprintf(&b, "CREATE EVENT TRIGGER %s ON %s", et.Name, et.Event)
	if len(et.Tags) > 0 {
		// Tags are wrapped in single quotes — escape internal quotes.
		quoted := make([]string, 0, len(et.Tags))
		for _, t := range et.Tags {
			quoted = append(quoted, "'"+strings.ReplaceAll(t, "'", "''")+"'")
		}
		fmt.Fprintf(&b, " WHEN TAG IN (%s)", strings.Join(quoted, ", "))
	}
	fmt.Fprintf(&b, " EXECUTE FUNCTION %s", et.Function)
	return b.String()
}
