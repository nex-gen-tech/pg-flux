package differ

import (
	"fmt"
	"sort"
	"strings"

	"github.com/nexg/pg-flux/pkg/plan"
	"github.com/nexg/pg-flux/pkg/schema"
)

// diffTableAttrs emits:
//   - ALTER TABLE ... SET UNLOGGED / SET LOGGED when persistence differs
//   - ALTER TABLE ... SET (key = val) / RESET (key) when WITH reloptions differ
// Both are catalog-only on idle tables; SET UNLOGGED/LOGGED on a populated table
// is a full rewrite under AccessExclusive (we mark with no special hazard since
// the operation is rare and intentional — users opt in by toggling the flag).
//
// Sequence OWNED BY and AS type are also handled here for symmetry.
func diffTableAttrs(d, l *schema.SchemaState) []change {
	var out []change
	if d == nil || l == nil {
		return out
	}
	for k, dt := range d.Tables {
		if dt == nil {
			continue
		}
		lt := l.Tables[k]
		if lt == nil {
			continue
		}
		if dt.Unlogged != lt.Unlogged {
			persistence := "LOGGED"
			if dt.Unlogged {
				persistence = "UNLOGGED"
			}
			out = append(out, change{
				kind:   plan.ChangeRawSQL,
				rawSQL: fmt.Sprintf("ALTER TABLE %s.%s SET %s", ident(dt.Schema), ident(dt.Name), persistence),
				tbl:    schema.TableKey(dt.Schema, dt.Name),
			})
		}
		// reloptions: set added / changed keys, reset removed keys
		dm := parseReLOptions(dt.ReLOptions)
		lm := parseReLOptions(lt.ReLOptions)
		var setKeys, resetKeys []string
		for k, v := range dm {
			if lv, ok := lm[k]; !ok || lv != v {
				setKeys = append(setKeys, fmt.Sprintf("%s = %s", k, v))
			}
		}
		for k := range lm {
			if _, ok := dm[k]; !ok {
				resetKeys = append(resetKeys, k)
			}
		}
		sort.Strings(setKeys)
		sort.Strings(resetKeys)
		if len(setKeys) > 0 {
			out = append(out, change{
				kind:   plan.ChangeRawSQL,
				rawSQL: fmt.Sprintf("ALTER TABLE %s.%s SET (%s)", ident(dt.Schema), ident(dt.Name), strings.Join(setKeys, ", ")),
				tbl:    schema.TableKey(dt.Schema, dt.Name),
			})
		}
		if len(resetKeys) > 0 {
			out = append(out, change{
				kind:   plan.ChangeRawSQL,
				rawSQL: fmt.Sprintf("ALTER TABLE %s.%s RESET (%s)", ident(dt.Schema), ident(dt.Name), strings.Join(resetKeys, ", ")),
				tbl:    schema.TableKey(dt.Schema, dt.Name),
			})
		}
	}
	// Sequence OWNED BY + AS type
	for k, ds := range d.Sequences {
		if ds == nil {
			continue
		}
		ls := l.Sequences[k]
		if ls == nil {
			continue
		}
		if ds.OwnedBy != "" && !strings.EqualFold(ds.OwnedBy, ls.OwnedBy) {
			out = append(out, change{
				kind:   plan.ChangeRawSQL,
				rawSQL: fmt.Sprintf("ALTER SEQUENCE %s.%s OWNED BY %s", ident(ds.Schema), ident(ds.Name), ds.OwnedBy),
				tbl:    schema.SeqKey(ds.Schema, ds.Name),
			})
		}
		if ds.AsType != "" && !strings.EqualFold(ds.AsType, ls.AsType) {
			out = append(out, change{
				kind:   plan.ChangeRawSQL,
				rawSQL: fmt.Sprintf("ALTER SEQUENCE %s.%s AS %s", ident(ds.Schema), ident(ds.Name), ds.AsType),
				tbl:    schema.SeqKey(ds.Schema, ds.Name),
			})
		}
	}
	return out
}

// parseReLOptions converts a pg_class.reloptions text[] (each entry "key=value")
// into a map. Values are kept as-is (no quoting).
func parseReLOptions(opts []string) map[string]string {
	m := make(map[string]string, len(opts))
	for _, kv := range opts {
		i := strings.IndexByte(kv, '=')
		if i <= 0 {
			continue
		}
		m[strings.ToLower(strings.TrimSpace(kv[:i]))] = strings.TrimSpace(kv[i+1:])
	}
	return m
}
