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
	return collapseConstraintRenames(out, dt.Schema, dt.Name, colRenames)
}

// collapseConstraintRenames replaces (DROP X, ADD Y) pairs on the same table — where X and Y
// share the same kind and have semantically identical definitions after applying any column
// renames — with a single ALTER TABLE ... RENAME CONSTRAINT. Avoids unnecessary scans/locks
// and preserves the constraint's underlying object identity.
func collapseConstraintRenames(in []change, schemaName, tableName string, colRenames map[string]string) []change {
	if len(in) < 2 {
		return in
	}
	type addInfo struct {
		idx  int
		kind string
		def  string
	}
	adds := make([]addInfo, 0, len(in))
	for i, c := range in {
		if c.kind == plan.ChangeAddConstraint {
			adds = append(adds, addInfo{idx: i, kind: c.conKind, def: c.conDef})
		}
	}
	if len(adds) == 0 {
		return in
	}
	used := make(map[int]bool, len(adds))
	out := make([]change, 0, len(in))
	for i, c := range in {
		if c.kind != plan.ChangeDropConstraint {
			out = append(out, c)
			continue
		}
		matched := -1
		for j, a := range adds {
			if used[j] || a.kind != c.conKind {
				continue
			}
			// Fingerprint the def text only — pass the same placeholder name on both sides
			// so a rename (name differs, def matches) is not masked by name-in-the-hash.
			const placeholder = "__pgflux_cmp__"
			dropFp := tableConstraintDefFingerprint(schemaName, tableName, placeholder, applyColRenames(c.conDef, colRenames))
			addFp := tableConstraintDefFingerprint(schemaName, tableName, placeholder, a.def)
			if dropFp != "" && dropFp == addFp {
				matched = j
				break
			}
		}
		if matched == -1 {
			out = append(out, c)
			continue
		}
		used[matched] = true
		newName := in[adds[matched].idx].conName
		out = append(out, change{
			kind:    plan.ChangeRenameConstraint,
			sch:     schemaName,
			tbl:     tableName,
			from:    c.conName,
			conName: newName,
			conKind: c.conKind,
			conDef:  c.conDef,
		})
		// suppress the unused fields by hint
		_ = i
	}
	// Filter out the matched ADDs that were collapsed.
	if len(used) == 0 {
		return out
	}
	filtered := out[:0]
	for _, c := range out {
		if c.kind != plan.ChangeAddConstraint {
			filtered = append(filtered, c)
			continue
		}
		drop := false
		for j, a := range adds {
			if used[j] && a.idx < len(in) && in[a.idx].conName == c.conName && in[a.idx].conDef == c.conDef {
				drop = true
				break
			}
		}
		if !drop {
			filtered = append(filtered, c)
		}
	}
	return filtered
}

// injectViewRefreshForTypeChanges ensures that views whose body references a column whose
// type is being changed are dropped before and re-created after the ALTER COLUMN TYPE.
// PostgreSQL refuses to ALTER the type of a column when a view's definition references
// that specific column (column-level dependency, not table-level), so we only flag a
// view when both the table name AND the changed column name appear in the view body.
func injectViewRefreshForTypeChanges(changes []change, desired, live *schema.SchemaState) []change {
	// Collect tables-and-columns that have a column type change.
	// Map: schema.table -> set of column names whose type changes.
	typeChangedCols := map[string]map[string]bool{}
	for _, c := range changes {
		if c.kind == plan.ChangeAlterColumn && c.alterKind == "type" {
			tk := schema.TableKey(c.sch, c.tbl)
			if typeChangedCols[tk] == nil {
				typeChangedCols[tk] = map[string]bool{}
			}
			typeChangedCols[tk][c.col] = true
		}
	}
	if len(typeChangedCols) == 0 {
		return changes
	}

	// viewBodyReferencesCol returns true when `body` references column `col` somewhere
	// that PostgreSQL treats as a column-level dependency. We use a column-name regex
	// guarded by word boundaries; this errs on the side of dropping (correct + safe)
	// when the same identifier appears in an unrelated context — but eliminates the
	// far more common false-positive of "view uses table T but not col C".
	colReferenced := func(body, tblSchema, tblName string, cols map[string]bool) bool {
		reQual := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(tblSchema) + `\s*\.\s*` + regexp.QuoteMeta(tblName) + `\b`)
		reBare := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(tblName) + `\b`)
		if !reQual.MatchString(body) && !reBare.MatchString(body) {
			return false
		}
		for col := range cols {
			reCol := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(col) + `\b`)
			if reCol.MatchString(body) {
				return true
			}
		}
		return false
	}

	// Determine which views need to be dropped early (before ALTER_COLUMN_TYPE).
	needsEarlyDrop := map[string]bool{} // view key
	for vk, dv := range desired.Views {
		if dv == nil {
			continue
		}
		for tk, cols := range typeChangedCols {
			parts := strings.SplitN(tk, ".", 2)
			tblSchema := parts[0]
			tblName := parts[len(parts)-1]
			if colReferenced(dv.DefSQL, tblSchema, tblName, cols) {
				needsEarlyDrop[vk] = true
				break
			}
		}
	}
	// Also scan live-only views (being dropped) that reference type-changed columns —
	// PostgreSQL still requires them to be dropped before ALTER COLUMN TYPE.
	for vk, lv := range live.Views {
		if lv == nil {
			continue
		}
		if desired.Views[vk] != nil {
			continue // already handled above
		}
		for tk, cols := range typeChangedCols {
			parts := strings.SplitN(tk, ".", 2)
			tblSchema := parts[0]
			tblName := parts[len(parts)-1]
			if colReferenced(lv.DefSQL, tblSchema, tblName, cols) {
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
			// Emit ALTER SEQUENCE instead of DROP+CREATE to preserve the current value.
			if ddl := buildAlterSequenceSQL(ds); ddl != "" {
				out = append(out, change{kind: plan.ChangeRawSQL, rawSQL: ddl})
			} else {
				// Fallback: cannot parse params; use DROP+CREATE (destructive but rare).
				out = append(out, change{kind: plan.ChangeDropSequence, seq: ls, dropSeq: k})
				out = append(out, change{kind: plan.ChangeCreateSequence, seq: ds})
			}
		}
	}
	for k, ls := range l.Sequences {
		if ls == nil {
			continue
		}
		if d.Sequences[k] == nil {
			// Suppress DROP for sequences PG auto-created for a bigserial/serial column
			// (live OwnedBy is set + the owning column has a nextval default OR an IDENTITY).
			if isImplicitOwnedSequence(ls, d) {
				continue
			}
			out = append(out, change{kind: plan.ChangeDropSequence, seq: ls, dropSeq: k})
		}
	}
	return out
}

// isImplicitOwnedSequence reports whether a live sequence is a serial/bigserial
// helper that the user can't reasonably be expected to declare in source. Heuristic:
// the sequence is OwnedBy schema.table.column AND the desired table has that column
// as IDENTITY or with a nextval(...) default referencing this sequence.
func isImplicitOwnedSequence(ls *schema.Sequence, d *schema.SchemaState) bool {
	if ls == nil || ls.OwnedBy == "" || d == nil {
		return false
	}
	// OwnedBy is "schema.table.column"; split.
	parts := strings.Split(ls.OwnedBy, ".")
	if len(parts) < 3 {
		return false
	}
	sch, tbl, col := parts[0], parts[1], parts[2]
	t := d.Tables[schema.TableKey(sch, tbl)]
	if t == nil {
		return false
	}
	for _, c := range t.Columns {
		if c == nil || c.Name != col {
			continue
		}
		if c.Identity != "" {
			return true
		}
		if strings.HasPrefix(strings.ToLower(c.DefaultSQL), "nextval(") {
			return true
		}
		// bigserial/serial pseudo-types: the parser preserves the type as-is in TypeSQL,
		// so a column declared `bigserial` keeps that text and PG auto-creates the sequence.
		ts := strings.ToLower(c.TypeSQL)
		if ts == "serial" || ts == "bigserial" || ts == "smallserial" {
			return true
		}
	}
	return false
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
			// New trigger created with non-default state — emit a follow-up ALTER.
			// PG creates triggers in ENABLE/origin mode regardless of intent.
			if normTriggerEnabled(dt.Enabled) != "O" {
				out = append(out, triggerEnableChange(dt, normTriggerEnabled(dt.Enabled)))
			}
			continue
		}
		if createStmtDefFingerprint(dt.DefSQL) != createStmtDefFingerprint(ltg.DefSQL) {
			out = append(out, change{kind: plan.ChangeDropTrigger, trig: ltg, trigKey: k})
			out = append(out, change{kind: plan.ChangeCreateTrigger, trig: dt})
			if normTriggerEnabled(dt.Enabled) != "O" {
				out = append(out, triggerEnableChange(dt, normTriggerEnabled(dt.Enabled)))
			}
			continue
		}
		// Definition matched — check the enable/disable state separately so an
		// ENABLE/DISABLE/REPLICA/ALWAYS flip emits a single targeted ALTER, no
		// destructive DROP+CREATE that would lose ON/OFF history.
		if normTriggerEnabled(dt.Enabled) != normTriggerEnabled(ltg.Enabled) {
			out = append(out, triggerEnableChange(dt, normTriggerEnabled(dt.Enabled)))
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

// normTriggerEnabled returns the canonical tgenabled letter; empty defaults to "O" (origin/enabled).
func normTriggerEnabled(s string) string {
	if s == "" {
		return "O"
	}
	return s
}

// triggerEnableChange returns a raw ALTER TABLE statement flipping a trigger's
// enable state to `state` ("O","D","R","A"). Uses ChangeRawSQL so it slots into
// the existing pass-through emit path without a new ChangeType.
func triggerEnableChange(dt *schema.Trigger, state string) change {
	var verb string
	switch state {
	case "D":
		verb = "DISABLE TRIGGER"
	case "R":
		verb = "ENABLE REPLICA TRIGGER"
	case "A":
		verb = "ENABLE ALWAYS TRIGGER"
	default:
		verb = "ENABLE TRIGGER" // "O"
	}
	return change{
		kind: plan.ChangeRawSQL,
		rawSQL: "ALTER TABLE " + ident(dt.Schema) + "." + ident(dt.Table) +
			" " + verb + " " + ident(dt.Name),
		tbl: schema.TableKey(dt.Schema, dt.Table),
	}
}
