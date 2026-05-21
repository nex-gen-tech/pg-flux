package differ

import (
	"fmt"
	"sort"
	"strings"

	"github.com/nex-gen-tech/pg-flux/pkg/plan"
	"github.com/nex-gen-tech/pg-flux/pkg/schema"
)

// diffForeignServers emits ALTER SERVER for option / version changes, and DROP
// SERVER for live-only. CREATE flows through ExtraDDL passthrough.
func diffForeignServers(d, l *schema.SchemaState) []change {
	var out []change
	if d == nil {
		d = &schema.SchemaState{}
	}
	if l == nil {
		l = &schema.SchemaState{}
	}
	for k, ds := range d.ForeignServers {
		if ds == nil {
			continue
		}
		ls := l.ForeignServers[k]
		if ls == nil {
			continue
		}
		out = append(out, foreignServerAlters(ds, ls)...)
	}
	for k, ls := range l.ForeignServers {
		if ls == nil {
			continue
		}
		if _, ok := d.ForeignServers[k]; ok {
			continue
		}
		out = append(out, change{
			kind:   plan.ChangeRawSQL,
			rawSQL: fmt.Sprintf("DROP SERVER IF EXISTS %s CASCADE", ident(ls.Name)),
			tbl:    "foreign-server/" + k,
		})
	}
	return out
}

func foreignServerAlters(d, l *schema.ForeignServer) []change {
	var out []change
	name := ident(d.Name)
	if d.Version != "" && d.Version != l.Version {
		out = append(out, change{
			kind:   plan.ChangeRawSQL,
			rawSQL: fmt.Sprintf("ALTER SERVER %s VERSION '%s'", name, strings.ReplaceAll(d.Version, "'", "''")),
			tbl:    "foreign-server/" + d.Name,
		})
	}
	// Options diff
	dm := optionsMap(d.Options)
	lm := optionsMap(l.Options)
	var setOps, addOps, dropOps []string
	for k, v := range dm {
		lv, ok := lm[k]
		if !ok {
			addOps = append(addOps, fmt.Sprintf("ADD %s '%s'", k, escapeSingleQuote(v)))
		} else if lv != v {
			setOps = append(setOps, fmt.Sprintf("SET %s '%s'", k, escapeSingleQuote(v)))
		}
	}
	for k := range lm {
		if _, ok := dm[k]; !ok {
			dropOps = append(dropOps, "DROP "+k)
		}
	}
	all := append(append(append([]string{}, addOps...), setOps...), dropOps...)
	sort.Strings(all)
	if len(all) > 0 {
		out = append(out, change{
			kind:   plan.ChangeRawSQL,
			rawSQL: fmt.Sprintf("ALTER SERVER %s OPTIONS (%s)", name, strings.Join(all, ", ")),
			tbl:    "foreign-server/" + d.Name,
		})
	}
	return out
}

// diffForeignTables: simple OPTIONS + DROP. Column-level diff falls through
// regular table diff if foreign tables are also tracked in st.Tables (they are
// not currently — kept separate intentionally). Column-level edits to foreign
// tables are not auto-diffed in this pass.
func diffForeignTables(d, l *schema.SchemaState) []change {
	var out []change
	if d == nil {
		d = &schema.SchemaState{}
	}
	if l == nil {
		l = &schema.SchemaState{}
	}
	for k, dt := range d.ForeignTables {
		if dt == nil {
			continue
		}
		lt := l.ForeignTables[k]
		if lt == nil {
			continue
		}
		dm := optionsMap(dt.Options)
		lm := optionsMap(lt.Options)
		var setOps, addOps, dropOps []string
		for k2, v := range dm {
			lv, ok := lm[k2]
			if !ok {
				addOps = append(addOps, fmt.Sprintf("ADD %s '%s'", k2, escapeSingleQuote(v)))
			} else if lv != v {
				setOps = append(setOps, fmt.Sprintf("SET %s '%s'", k2, escapeSingleQuote(v)))
			}
		}
		for k2 := range lm {
			if _, ok := dm[k2]; !ok {
				dropOps = append(dropOps, "DROP "+k2)
			}
		}
		all := append(append(append([]string{}, addOps...), setOps...), dropOps...)
		sort.Strings(all)
		if len(all) > 0 {
			out = append(out, change{
				kind: plan.ChangeRawSQL,
				rawSQL: fmt.Sprintf("ALTER FOREIGN TABLE %s.%s OPTIONS (%s)",
					ident(dt.Schema), ident(dt.Name), strings.Join(all, ", ")),
				tbl: "foreign-table/" + k,
			})
		}
	}
	for k, lt := range l.ForeignTables {
		if lt == nil {
			continue
		}
		if _, ok := d.ForeignTables[k]; ok {
			continue
		}
		out = append(out, change{
			kind:   plan.ChangeRawSQL,
			rawSQL: fmt.Sprintf("DROP FOREIGN TABLE IF EXISTS %s.%s CASCADE", ident(lt.Schema), ident(lt.Name)),
			tbl:    "foreign-table/" + k,
		})
	}
	return out
}

// optionsMap parses a list of "key=value" strings into a map.
func optionsMap(opts []string) map[string]string {
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

func escapeSingleQuote(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}
