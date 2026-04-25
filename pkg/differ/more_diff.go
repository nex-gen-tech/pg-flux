package differ

import (
	"strings"

	"github.com/nexg/pg-flux/pkg/plan"
	"github.com/nexg/pg-flux/pkg/schema"
)

func ensureMoreMaps(s *schema.SchemaState) {
	if s == nil {
		return
	}
	if s.Views == nil {
		s.Views = make(map[string]*schema.View)
	}
	if s.Sequences == nil {
		s.Sequences = make(map[string]*schema.Sequence)
	}
	if s.Triggers == nil {
		s.Triggers = make(map[string]*schema.Trigger)
	}
}

func fpGenericSQL(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = reMultiSpace.ReplaceAllString(s, " ")
	return fpSQL(s)
}

func diffTableConstraints(dt, lt *schema.Table) []change {
	var out []change
	if dt == nil || lt == nil {
		return out
	}
	dm := map[string]struct{ kind, def string }{}
	for _, c := range dt.Checks {
		if c == nil {
			continue
		}
		dm[c.Name] = struct{ kind, def string }{"c", c.DefSQL}
	}
	for _, c := range dt.ForeignKeys {
		if c == nil {
			continue
		}
		dm[c.Name] = struct{ kind, def string }{"f", c.DefSQL}
	}
	for _, c := range dt.Uniques {
		if c == nil {
			continue
		}
		dm[c.Name] = struct{ kind, def string }{"u", c.DefSQL}
	}
	for _, c := range dt.Excludes {
		if c == nil {
			continue
		}
		dm[c.Name] = struct{ kind, def string }{"x", c.DefSQL}
	}
	lm := map[string]struct{ kind, def string }{}
	for _, c := range lt.Checks {
		if c == nil {
			continue
		}
		lm[c.Name] = struct{ kind, def string }{"c", c.DefSQL}
	}
	for _, c := range lt.ForeignKeys {
		if c == nil {
			continue
		}
		lm[c.Name] = struct{ kind, def string }{"f", c.DefSQL}
	}
	for _, c := range lt.Uniques {
		if c == nil {
			continue
		}
		lm[c.Name] = struct{ kind, def string }{"u", c.DefSQL}
	}
	for _, c := range lt.Excludes {
		if c == nil {
			continue
		}
		lm[c.Name] = struct{ kind, def string }{"x", c.DefSQL}
	}
	for n, dv := range dm {
		if lv, ok := lm[n]; !ok {
			out = append(out, change{
				kind:    plan.ChangeAddConstraint,
				sch:     dt.Schema,
				tbl:     dt.Name,
				conName: n,
				conKind: dv.kind,
				conDef:  dv.def,
			})
		} else {
			if tableConstraintDefFingerprint(dt.Schema, dt.Name, n, dv.def) != tableConstraintDefFingerprint(dt.Schema, dt.Name, n, lv.def) {
				out = append(out, change{
					kind:    plan.ChangeDropConstraint,
					sch:     lt.Schema,
					tbl:     lt.Name,
					conName: n,
					conKind: lv.kind,
					conDef:  lv.def,
				})
				out = append(out, change{
					kind:    plan.ChangeAddConstraint,
					sch:     dt.Schema,
					tbl:     dt.Name,
					conName: n,
					conKind: dv.kind,
					conDef:  dv.def,
				})
			}
		}
	}
	for n, lv := range lm {
		if _, ok := dm[n]; !ok {
			out = append(out, change{
				kind:    plan.ChangeDropConstraint,
				sch:     lt.Schema,
				tbl:     lt.Name,
				conName: n,
				conKind: lv.kind,
				conDef:  lv.def,
			})
		}
	}
	return out
}

func diffViews(d, l *schema.SchemaState) []change {
	var out []change
	ensureMoreMaps(d)
	ensureMoreMaps(l)
	for k, dv := range d.Views {
		if dv == nil {
			continue
		}
		lv := l.Views[k]
		if lv == nil {
			out = append(out, change{kind: plan.ChangeCreateView, v: dv})
			continue
		}
		if createStmtDefFingerprint(dv.DefSQL) != createStmtDefFingerprint(lv.DefSQL) {
			out = append(out, change{kind: plan.ChangeDropView, v: lv, viewKey: k})
			out = append(out, change{kind: plan.ChangeCreateView, v: dv})
		}
	}
	for k, lv := range l.Views {
		if lv == nil {
			continue
		}
		if d.Views[k] == nil {
			out = append(out, change{kind: plan.ChangeDropView, v: lv, viewKey: k})
		}
	}
	return out
}

func diffSequences(d, l *schema.SchemaState) []change {
	var out []change
	ensureMoreMaps(d)
	ensureMoreMaps(l)
	for k, ds := range d.Sequences {
		if ds == nil {
			continue
		}
		ls := l.Sequences[k]
		if ls == nil {
			out = append(out, change{kind: plan.ChangeCreateSequence, seq: ds})
			continue
		}
		if fpGenericSQL(ds.DefSQL) != fpGenericSQL(ls.DefSQL) {
			out = append(out, change{kind: plan.ChangeDropSequence, seq: ls, dropSeq: k})
			out = append(out, change{kind: plan.ChangeCreateSequence, seq: ds})
		}
	}
	for k, ls := range l.Sequences {
		if ls == nil {
			continue
		}
		if d.Sequences[k] == nil {
			out = append(out, change{kind: plan.ChangeDropSequence, seq: ls, dropSeq: k})
		}
	}
	return out
}

func diffTriggers(d, l *schema.SchemaState) []change {
	var out []change
	ensureMoreMaps(d)
	ensureMoreMaps(l)
	for k, dt := range d.Triggers {
		if dt == nil {
			continue
		}
		ltg := l.Triggers[k]
		if ltg == nil {
			out = append(out, change{kind: plan.ChangeCreateTrigger, trig: dt})
			continue
		}
		if createStmtDefFingerprint(dt.DefSQL) != createStmtDefFingerprint(ltg.DefSQL) {
			out = append(out, change{kind: plan.ChangeDropTrigger, trig: ltg, trigKey: k})
			out = append(out, change{kind: plan.ChangeCreateTrigger, trig: dt})
		}
	}
	for k, ltg := range l.Triggers {
		if ltg == nil {
			continue
		}
		if d.Triggers[k] == nil {
			out = append(out, change{kind: plan.ChangeDropTrigger, trig: ltg, trigKey: k})
		}
	}
	return out
}
