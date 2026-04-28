package differ

import (
	"regexp"
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

func diffTableConstraints(dt, lt *schema.Table, colRenames map[string]string) []change {
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
			if tableConstraintDefFingerprint(dt.Schema, dt.Name, n, dv.def) != tableConstraintDefFingerprint(dt.Schema, dt.Name, n, applyColRenames(lv.def, colRenames)) {
				out = append(out, change{
					kind:    plan.ChangeDropConstraint,
					sch:     dt.Schema,
					tbl:     dt.Name,
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
				sch:     dt.Schema,
				tbl:     dt.Name,
				conName: n,
				conKind: lv.kind,
				conDef:  lv.def,
			})
		}
	}
	return out
}

// injectViewRefreshForTypeChanges ensures that views referencing a table whose column
// type is being changed are dropped before and re-created after the ALTER COLUMN TYPE.
// PostgreSQL refuses to ALTER the type of a column used by a view or rule.
func injectViewRefreshForTypeChanges(changes []change, desired, live *schema.SchemaState) []change {
	// Collect tables that have a column type change.
	typeChangedTables := map[string]bool{} // schema.table key
	for _, c := range changes {
		if c.kind == plan.ChangeAlterColumn && c.alterKind == "type" {
			typeChangedTables[schema.TableKey(c.sch, c.tbl)] = true
		}
	}
	if len(typeChangedTables) == 0 {
		return changes
	}

	// Determine which views need to be dropped early (before ALTER_COLUMN_TYPE).
	needsEarlyDrop := map[string]bool{} // view key
	for vk, dv := range desired.Views {
		if dv == nil {
			continue
		}
		for tk := range typeChangedTables {
			parts := strings.SplitN(tk, ".", 2)
			tblName := parts[len(parts)-1]
			re := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(tblName) + `\b`)
			if re.MatchString(dv.DefSQL) {
				needsEarlyDrop[vk] = true
				break
			}
		}
	}
	if len(needsEarlyDrop) == 0 {
		return changes
	}

	// Upgrade any existing ChangeDropView to ChangeDropViewEarly for views that
	// need an early drop. This handles the case where the view is also being modified
	// (diffViews already produced a DROP_VIEW at priority 16, but we need priority 3).
	alreadyHandled := map[string]bool{}
	for i := range changes {
		c := &changes[i]
		if c.kind == plan.ChangeDropView && c.v != nil {
			vk := schema.ViewKey(c.v.Schema, c.v.Name)
			if needsEarlyDrop[vk] {
				c.kind = plan.ChangeDropViewEarly
				alreadyHandled[vk] = true
			}
		}
		if c.kind == plan.ChangeDropViewEarly && c.v != nil {
			alreadyHandled[schema.ViewKey(c.v.Schema, c.v.Name)] = true
		}
	}

	// For views not yet in the change list at all, add DROP_VIEW_EARLY + CREATE_VIEW.
	var extra []change
	for vk := range needsEarlyDrop {
		if alreadyHandled[vk] {
			continue
		}
		dv := desired.Views[vk]
		if dv == nil {
			continue
		}
		lv := live.Views[vk]
		if lv == nil {
			continue // view is new — nothing to drop
		}
		extra = append(extra, change{kind: plan.ChangeDropViewEarly, v: lv, viewKey: vk})
		extra = append(extra, change{kind: plan.ChangeCreateView, v: dv})
	}
	if len(extra) == 0 {
		return changes
	}
	return append(changes, extra...)
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
		if !seqParamsEqual(ds.DefSQL, ls.DefSQL) {
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
