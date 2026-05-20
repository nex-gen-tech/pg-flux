package differ

import (
	"fmt"
	"sort"
	"strings"

	"github.com/nexg/pg-flux/pkg/plan"
	"github.com/nexg/pg-flux/pkg/schema"
)

// diffPublications emits ALTER PUBLICATION statements when desired/live differ.
// CREATE PUBLICATION lands via the ExtraDDL pass-through on first apply. This
// pass handles incremental edits: add/drop tables, add/drop schemas (PG15+),
// publish-action set, allTables flip.
//
// DROP PUBLICATION fires for publications only in live.
func diffPublications(d, l *schema.SchemaState) []change {
	var out []change
	if d == nil {
		d = &schema.SchemaState{}
	}
	if l == nil {
		l = &schema.SchemaState{}
	}
	for k, dp := range d.Publications {
		if dp == nil {
			continue
		}
		lp := l.Publications[k]
		if lp == nil {
			continue // CREATE via passthrough
		}
		out = append(out, publicationAlters(dp, lp)...)
	}
	for k, lp := range l.Publications {
		if lp == nil {
			continue
		}
		if _, ok := d.Publications[k]; ok {
			continue
		}
		out = append(out, change{
			kind:   plan.ChangeRawSQL,
			rawSQL: fmt.Sprintf("DROP PUBLICATION IF EXISTS %s", ident(lp.Name)),
			tbl:    "publication/" + k,
		})
	}
	return out
}

func publicationAlters(d, l *schema.Publication) []change {
	var out []change
	name := ident(d.Name)
	// AllTables flip: PG can't ALTER FOR ALL TABLES <-> FOR TABLES once set, so DROP+CREATE.
	if d.AllTables != l.AllTables {
		out = append(out, change{
			kind:   plan.ChangeRawSQL,
			rawSQL: fmt.Sprintf("DROP PUBLICATION IF EXISTS %s", name),
			tbl:    "publication/" + d.Name,
		})
		// CREATE is left to ExtraDDL passthrough (which re-emits the desired CREATE).
		return out
	}
	dTables := sortedCopy(d.Tables)
	lTables := sortedCopy(l.Tables)
	add, drop := stringSetDiff(dTables, lTables)
	for _, t := range drop {
		out = append(out, change{
			kind:   plan.ChangeRawSQL,
			rawSQL: fmt.Sprintf("ALTER PUBLICATION %s DROP TABLE %s", name, t),
			tbl:    "publication/" + d.Name,
		})
	}
	for _, t := range add {
		out = append(out, change{
			kind:   plan.ChangeRawSQL,
			rawSQL: fmt.Sprintf("ALTER PUBLICATION %s ADD TABLE %s", name, t),
			tbl:    "publication/" + d.Name,
		})
	}
	dSchemas := sortedCopy(d.Schemas)
	lSchemas := sortedCopy(l.Schemas)
	addS, dropS := stringSetDiff(dSchemas, lSchemas)
	for _, s := range dropS {
		out = append(out, change{
			kind:   plan.ChangeRawSQL,
			rawSQL: fmt.Sprintf("ALTER PUBLICATION %s DROP TABLES IN SCHEMA %s", name, ident(s)),
			tbl:    "publication/" + d.Name,
		})
	}
	for _, s := range addS {
		out = append(out, change{
			kind:   plan.ChangeRawSQL,
			rawSQL: fmt.Sprintf("ALTER PUBLICATION %s ADD TABLES IN SCHEMA %s", name, ident(s)),
			tbl:    "publication/" + d.Name,
		})
	}
	// publish action set
	if normPublish(d.Publish) != normPublish(l.Publish) && d.Publish != "" {
		out = append(out, change{
			kind:   plan.ChangeRawSQL,
			rawSQL: fmt.Sprintf("ALTER PUBLICATION %s SET (publish = '%s')", name, normPublish(d.Publish)),
			tbl:    "publication/" + d.Name,
		})
	}
	return out
}

// stringSetDiff returns elements in a not in b (add) and b not in a (drop).
func stringSetDiff(a, b []string) (add, drop []string) {
	bm := make(map[string]bool, len(b))
	for _, v := range b {
		bm[v] = true
	}
	am := make(map[string]bool, len(a))
	for _, v := range a {
		am[v] = true
	}
	for _, v := range a {
		if !bm[v] {
			add = append(add, v)
		}
	}
	for _, v := range b {
		if !am[v] {
			drop = append(drop, v)
		}
	}
	sort.Strings(add)
	sort.Strings(drop)
	return
}

func normPublish(s string) string {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(s)), ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return strings.Join(out, ",")
}
