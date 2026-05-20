package differ

import (
	"fmt"
	"sort"
	"strings"

	"github.com/nexg/pg-flux/pkg/dag"
	"github.com/nexg/pg-flux/pkg/plan"
	"github.com/nexg/pg-flux/pkg/schema"
)

// sortChangesDeterministic orders changes so the same input graph always yields the same
// statement list after buildStatements+TopoSort (in addition to DDL string sort in buildStatements).
func sortChangesDeterministic(des *schema.SchemaState, ch []change) {
	rank, _ := dag.TableCreationRank(des) // same as pre-validate in Diff; idempotent
	vrank := viewRank(des)
	sort.SliceStable(ch, func(i, j int) bool {
		si, sj := opScore(ch[i]), opScore(ch[j])
		if si != sj {
			return si < sj
		}
		// Create parent tables before child tables when FK order is known
		if ch[i].kind == plan.ChangeCreateTable && ch[j].kind == plan.ChangeCreateTable {
			if ch[i].t != nil && ch[j].t != nil {
				ki := schema.TableKey(ch[i].t.Schema, ch[i].t.Name)
				kj := schema.TableKey(ch[j].t.Schema, ch[j].t.Name)
				ri, rj := rank[ki], rank[kj]
				if ri != rj {
					return ri < rj
				}
			}
		}
		// Views: dependency-aware order (best-effort).
		if ch[i].kind == plan.ChangeCreateView && ch[j].kind == plan.ChangeCreateView {
			if ch[i].v != nil && ch[j].v != nil {
				ki := schema.ViewKey(ch[i].v.Schema, ch[i].v.Name)
				kj := schema.ViewKey(ch[j].v.Schema, ch[j].v.Name)
				ri, rj := vrank[ki], vrank[kj]
				if ri != rj {
					return ri < rj
				}
			}
		}
		tki, tkj := changeTieKey(ch[i]), changeTieKey(ch[j])
		if tki != tkj {
			return tki < tkj
		}
		return false
	})
}

func opScore(c change) int { return dag.OpTypeScore(string(c.kind)) }

func changeTieKey(c change) string {
	var b strings.Builder
	// table-ish
	fmt.Fprintf(&b, "%q|%q|%q|%q|", c.sch, c.tbl, c.from, c.fromTable)
	fmt.Fprintf(&b, "%q|", c.col)
	fmt.Fprintf(&b, "%q|", c.conName)
	if c.kind == plan.ChangeDropView || c.v != nil {
		if c.v != nil {
			fmt.Fprintf(&b, "v:%s.%s|", c.v.Schema, c.v.Name)
		} else {
			b.WriteString(c.viewKey)
			b.WriteRune('|')
		}
	}
	if c.seq != nil {
		fmt.Fprintf(&b, "s:%s.%s|", c.seq.Schema, c.seq.Name)
	} else {
		b.WriteString(c.dropSeq)
		b.WriteRune('|')
	}
	if c.trig != nil {
		fmt.Fprintf(&b, "g:%s.%s|", c.trig.Schema, c.trig.Name)
	} else {
		b.WriteString(c.trigKey)
		b.WriteRune('|')
	}
	if c.fn != nil {
		b.WriteString(c.fn.Identity)
		b.WriteRune('|')
	}
	if c.idx != nil {
		b.WriteString(c.idx.Schema)
		b.WriteRune('.')
		b.WriteString(c.idx.Name)
		b.WriteRune('|')
	} else {
		b.WriteString(c.ixName)
		b.WriteRune('|')
	}
	if c.pol != nil {
		b.WriteString(c.pol.Name)
	} else {
		b.WriteString(c.polKey)
	}
	return b.String()
}
