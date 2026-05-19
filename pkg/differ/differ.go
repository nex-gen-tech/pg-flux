package differ

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/nexg/pg-flux/pkg/dag"
	"github.com/nexg/pg-flux/pkg/hazard"
	"github.com/nexg/pg-flux/pkg/plan"
	"github.com/nexg/pg-flux/pkg/schema"
)

// Options configures differ behavior.
type Options struct {
	// Reltuples maps "schema.table" to pg_class.reltuples estimates (live DB). Used for large-table SET NOT NULL advisories.
	Reltuples map[string]float64
	// SetNotNullReltupleThreshold triggers STAGED_SET_NOT_NULL advisory when reltuples exceeds this (0 = disabled).
	SetNotNullReltupleThreshold float64
	// AppendValidateAfterNotValid adds a follow-up ALTER TABLE … VALIDATE CONSTRAINT for each ADD CONSTRAINT that contains NOT VALID.
	AppendValidateAfterNotValid bool
	// AutoConstraintNotValid, when true, rewrites ADD CONSTRAINT CHECK/FK to use NOT VALID
	// plus a follow-up VALIDATE CONSTRAINT (per PRD P3-14). Default ON in the CLI: keeps
	// the AccessExclusive scan off the critical path.
	AutoConstraintNotValid bool
}

// DiffResult contains a migration plan.
type DiffResult struct {
	Plan         *plan.ExecutionPlan
	Deprecations []string
}

// Diff compares desired and live schema states.
func Diff(desired, live *schema.SchemaState, opt Options) (*DiffResult, error) {
	if err := dag.ValidateSchemaFKGraph(desired); err != nil {
		return nil, err
	}
	if desired == nil {
		desired = &schema.SchemaState{Tables: map[string]*schema.Table{}}
	}
	if live == nil {
		live = &schema.SchemaState{Tables: map[string]*schema.Table{}}
	}
	// Fail loud when desired schema uses a feature unsupported on the live server.
	if err := checkServerCompat(desired, live.PGVersion); err != nil {
		return nil, err
	}
	var dep []string
	var changes []change

	// 1) Table match + table renames
	liveKeyFor := map[string]string{} // desired key -> live key used for diff
	for dKey, dt := range desired.Tables {
		if dt == nil || dt.Deprecated {
			dep = append(dep, "skipping deprecated: "+dKey)
			continue
		}
		nk := schema.TableKey(dt.Schema, dt.Name)
		if live.Tables[nk] != nil {
			liveKeyFor[dKey] = nk
			if dt.OldName != "" {
				// new name already present; no rename
			}
			continue
		}
		if dt.OldName != "" {
			ok := schema.TableKey(dt.Schema, dt.OldName)
			if live.Tables[ok] == nil {
				// brand-new table
				changes = append(changes, change{kind: plan.ChangeCreateTable, sch: dt.Schema, tbl: dt.Name, t: dt})
				continue
			}
			changes = append(changes, change{kind: plan.ChangeRenameTable, sch: dt.Schema, tbl: dt.Name, fromTable: dt.OldName})
			liveKeyFor[dKey] = ok
			continue
		}
		// new table
		changes = append(changes, change{kind: plan.ChangeCreateTable, sch: dt.Schema, tbl: dt.Name, t: dt})
	}

	// 2) Column + property diff for each matched table
	for dKey, dt := range desired.Tables {
		if dt == nil || dt.Deprecated {
			continue
		}
		lk, hasMapping := liveKeyFor[dKey]
		if hasChange(changes, dKey, plan.ChangeCreateTable) {
			// For brand-new tables that have RLS enabled or forced in the source schema,
			// we must emit TOGGLE_RLS after CREATE_TABLE — PostgreSQL creates tables with
			// RLS disabled by default.
			if dt.RLSEnabled || dt.RLSForced {
				changes = append(changes, change{kind: plan.ChangeToggleRLS, sch: dt.Schema, tbl: dt.Name, wantRls: dt.RLSEnabled, wantForce: dt.RLSForced, had: false})
			}
			continue
		}
		var lt *schema.Table
		if hasMapping {
			lt = live.Tables[lk]
		} else {
			lt = live.Tables[schema.TableKey(dt.Schema, dt.Name)]
		}
		if lt == nil {
			lt = &schema.Table{Schema: dt.Schema, Name: dt.Name, Columns: nil}
		}
		cc, err := diffColumns(dt, lt, dKey)
		if err != nil {
			return nil, err
		}
		changes = append(changes, cc...)

		if hasMapping && (dt.RLSEnabled != lt.RLSEnabled || dt.RLSForced != lt.RLSForced) {
			changes = append(changes, change{kind: plan.ChangeToggleRLS, sch: dt.Schema, tbl: dt.Name, wantRls: dt.RLSEnabled, wantForce: dt.RLSForced, had: lt.RLSEnabled})
		}
	}

	// 3) Drop tables in live with no desired mapping
	for lKey, lt := range live.Tables {
		_ = lt
		if wantTable(desired, lKey, liveKeyFor) {
			continue
		}
		p := strings.SplitN(lKey, ".", 2)
		if len(p) != 2 {
			continue
		}
		changes = append(changes, change{kind: plan.ChangeDropTable, sch: p[0], tbl: p[1]})
	}

	// Build a per-table column rename map from changes accumulated above.
	// Used by constraint and index diffing to avoid spurious DROP/ADD when a column
	// is being renamed in the same migration (live DB still has old column names).
	tableColRenames := map[string]map[string]string{}
	for _, ch := range changes {
		if ch.kind != plan.ChangeRenameColumn {
			continue
		}
		key := schema.TableKey(ch.sch, ch.tbl)
		if tableColRenames[key] == nil {
			tableColRenames[key] = map[string]string{}
		}
		tableColRenames[key][ch.from] = ch.col
	}

	// Table-level CHECK / FOREIGN KEY (skip if table is created in this plan — embedded in CREATE TABLE)
	for dKey, dt := range desired.Tables {
		if dt == nil || dt.Deprecated {
			continue
		}
		if hasChange(changes, dKey, plan.ChangeCreateTable) {
			continue
		}
		var lk string
		if m, ok := liveKeyFor[dKey]; ok {
			lk = m
		} else {
			lk = schema.TableKey(dt.Schema, dt.Name)
		}
		lt := live.Tables[lk]
		if lt == nil {
			continue
		}
		changes = append(changes, diffTableConstraints(dt, lt, tableColRenames[dKey])...)
	}
	changes = append(changes, diffIndexes(desired, live, tableColRenames)...)
	changes = append(changes, diffFunctions(desired, live)...)
	changes = append(changes, diffPolicies(desired, live)...)
	changes = append(changes, diffViews(desired, live)...)
	changes = append(changes, diffSequences(desired, live)...)
	changes = append(changes, diffTriggers(desired, live)...)
	changes = append(changes, diffExtensions(desired, live)...)
	changes = append(changes, diffDomains(desired, live)...)
	changes = append(changes, diffExtraDDL(desired, live)...)
	changes = append(changes, diffMiscObjects(desired)...)
	changes = append(changes, diffComments(desired, live)...)
	changes = append(changes, diffOwners(desired, live)...)
	changes = append(changes, diffFunctionMetadata(desired, live)...)
	changes = append(changes, diffColumnAttrs(desired, live)...)
	changes = append(changes, diffTableAttrs(desired, live)...)
	changes = append(changes, diffViewAttrs(desired, live)...)
	changes = append(changes, diffPrivileges(desired, live)...)
	changes = append(changes, diffDefaultPrivileges(desired, live)...)
	changes = append(changes, diffEventTriggers(desired, live)...)
	changes = injectViewRefreshForTypeChanges(changes, desired, live)
	sortChangesDeterministic(desired, changes)
	stmts := buildStatements(changes, desired, live, opt)
	// attach hazard metadata
	for i := range stmts {
		attachHazard(&stmts[i])
	}
	enrichHazardsFromOptions(&stmts, opt)
	stmts, err := dag.TopoSort(stmts)
	if err != nil {
		return nil, err
	}
	renumber(&stmts)
	return &DiffResult{Plan: &plan.ExecutionPlan{Statements: stmts}, Deprecations: dep}, nil
}

type change struct {
	kind                    plan.ChangeType
	sch, tbl                string
	from                    string // RENAME_COLUMN from
	fromTable               string // RENAME_TABLE from
	col                     string
	t                       *schema.Table
	dc, lc                  *schema.Column
	alterKind               string // type, notnull, def
	wantRls, wantForce, had bool
	// index / function / policy
	idx            *schema.Index
	dropIdx        string
	ixName         string
	skipConcurrent bool // true when table is partitioned (CONCURRENTLY not supported)
	fn      *schema.Function
	dropFn  string
	pol     *schema.Policy
	polKey  string
	// constraints / view / sequence / trigger
	conName, conDef, conKind string
	v                        *schema.View
	viewKey                  string
	seq                      *schema.Sequence
	dropSeq                  string
	trig                     *schema.Trigger
	trigKey                  string
	// pass-through DDL (partition attach, etc.)
	rawSQL string
	// rawConcurrent marks a RAW_DDL as needing to run outside the main txn (e.g. COMMENT
	// ON INDEX, which has to follow CREATE INDEX CONCURRENTLY in the post-COMMIT section).
	rawConcurrent bool
	ext        *schema.Extension
	dropExt    string
	extLiveVer string
	// extraHazards are advisory notices attached to this change (e.g. column reorder).
	extraHazards []hazard.Detected
}

func wantTable(des *schema.SchemaState, lKey string, m map[string]string) bool {
	for dKey, dt := range des.Tables {
		if dt == nil || dt.Deprecated {
			continue
		}
		if lk, ok := m[dKey]; ok && lk == lKey {
			return true
		}
		if schema.TableKey(dt.Schema, dt.Name) == lKey {
			return true
		}
		if dt.OldName != "" && schema.TableKey(dt.Schema, dt.OldName) == lKey {
			return true
		}
	}
	return false
}

func hasChange(ch []change, dKey string, k plan.ChangeType) bool {
	for _, c := range ch {
		if c.kind != k {
			continue
		}
		if c.t != nil && schema.TableKey(c.sch, c.tbl) == dKey {
			return true
		}
	}
	return false
}

// columnOrderNotice returns a non-empty advisory message when the relative order of
// surviving columns (those present in both desired and live) differs between the two.
// PostgreSQL does not support column reordering without table recreation, so this is
// surfaced as an advisory notice rather than actionable DDL.
func columnOrderNotice(dt, lt *schema.Table) string {
	if dt == nil || lt == nil {
		return ""
	}
	// Build a lookup: live column name → position
	livePos := make(map[string]int, len(lt.Columns))
	for i, c := range lt.Columns {
		if c != nil {
			livePos[c.Name] = i
		}
	}
	// Build the desired position of columns that survive (exist in live after renames).
	type pair struct{ desiredPos, livePos int }
	var surviving []pair
	for di, dc := range dt.Columns {
		if dc == nil {
			continue
		}
		liveName := dc.Name
		if dc.RenameFrom != "" {
			if _, ok := livePos[dc.RenameFrom]; ok {
				liveName = dc.RenameFrom
			}
		}
		if lp, ok := livePos[liveName]; ok {
			surviving = append(surviving, pair{di, lp})
		}
	}
	if len(surviving) < 2 {
		return ""
	}
	// Check if live positions are already in the same relative order as desired.
	for i := 1; i < len(surviving); i++ {
		if surviving[i].livePos < surviving[i-1].livePos {
			// Reordering detected.
			desiredOrder := make([]string, 0, len(surviving))
			for _, p := range surviving {
				desiredOrder = append(desiredOrder, dt.Columns[p.desiredPos].Name)
			}
			return fmt.Sprintf(
				"Column order in %s.%s differs from desired schema; reordering requires table recreation. Desired order (surviving cols): %s",
				dt.Schema, dt.Name, strings.Join(desiredOrder, ", "),
			)
		}
	}
	return ""
}

func diffColumns(dt, lt *schema.Table, dKey string) ([]change, error) {
	var out []change
	byName := map[string]*schema.Column{}
	for _, c := range lt.Columns {
		if c != nil {
			byName[c.Name] = c
		}
	}
	used := map[string]bool{}
	for _, c := range dt.Columns {
		if c == nil {
			continue
		}
		if c.RenameFrom != "" {
			old, ok := byName[c.RenameFrom]
			if !ok {
				if exists, h := byName[c.Name]; h {
					// Rename already applied to live DB. The hint is a no-op for the rename
					// itself, but column-level diffs (type/notnull/default/generated) must
					// still surface — otherwise leaving the hint in source silently swallows
					// subsequent changes to this column.
					used[c.Name] = true
					if normDef(c.GeneratedExpr) != normDef(exists.GeneratedExpr) {
						out = append(out, change{kind: plan.ChangeDropColumn, sch: dt.Schema, tbl: dt.Name, col: exists.Name, lc: exists})
						out = append(out, change{kind: plan.ChangeAddColumn, sch: dt.Schema, tbl: dt.Name, col: c.Name, dc: c})
					} else if colDiff(exists, c) {
						out = append(out, altersFor(dt.Schema, dt.Name, c, exists)...)
					}
					continue
				}
				// Fresh database: no old or new column yet — add the desired column (rename hint is a no-op for bootstrap).
				out = append(out, change{kind: plan.ChangeAddColumn, sch: dt.Schema, tbl: dt.Name, col: c.Name, dc: c})
				used[c.Name] = true
				continue
			}
			out = append(out, change{kind: plan.ChangeRenameColumn, sch: dt.Schema, tbl: dt.Name, from: c.RenameFrom, col: c.Name})
			used[c.RenameFrom] = true
			used[c.Name] = true
			if colDiff(old, c) {
				out = append(out, altersFor(dt.Schema, dt.Name, c, old)...)
			}
			continue
		}
		exists, ex := byName[c.Name]
		if !ex {
			out = append(out, change{kind: plan.ChangeAddColumn, sch: dt.Schema, tbl: dt.Name, col: c.Name, dc: c})
			continue
		}
		used[c.Name] = true
		// Changing between generated and non-generated requires DROP + ADD (PostgreSQL
		// does not support ALTER COLUMN SET GENERATED or STORED removal).
		if normDef(c.GeneratedExpr) != normDef(exists.GeneratedExpr) {
			out = append(out, change{kind: plan.ChangeDropColumn, sch: dt.Schema, tbl: dt.Name, col: exists.Name, lc: exists})
			out = append(out, change{kind: plan.ChangeAddColumn, sch: dt.Schema, tbl: dt.Name, col: c.Name, dc: c})
		} else {
			out = append(out, altersFor(dt.Schema, dt.Name, c, exists)...)
		}
	}
	for n, oc := range byName {
		if !used[n] {
			out = append(out, change{kind: plan.ChangeDropColumn, sch: dt.Schema, tbl: dt.Name, col: oc.Name, lc: oc})
		}
	}
	// Detect column ordering differences between desired and live for columns that survive.
	// Build the relative order of surviving columns in desired vs live.
	if notice := columnOrderNotice(dt, lt); notice != "" {
		out = append(out, change{
			kind: plan.ChangeRawSQL,
			rawSQL: "",
			sch:  dt.Schema,
			tbl:  dt.Name,
			extraHazards: []hazard.Detected{{
				Type:     hazard.ColumnReorder,
				Severity: hazard.SeverityAdvisory,
				Message:  notice,
			}},
		})
	}
	return out, nil
}

func colDiff(a, b *schema.Column) bool {
	if a == nil || b == nil {
		return a != b
	}
	if normType(a.TypeSQL) != normType(b.TypeSQL) {
		return true
	}
	if a.NotNull != b.NotNull {
		return true
	}
	// Generated expression takes priority — compare it before DefaultSQL.
	if normDef(a.GeneratedExpr) != normDef(b.GeneratedExpr) {
		return true
	}
	if a.GeneratedExpr != "" {
		// Both have matching generated expressions; skip DefaultSQL comparison.
		return false
	}
	if normDef(a.DefaultSQL) != normDef(b.DefaultSQL) {
		// If one side has no default and the other is an implicit serial nextval
		// default (owned sequence), treat them as equal — the serial sequence was
		// created implicitly by bigserial/serial and is not tracked in the desired state.
		if isImplicitSerialDefault(a.DefaultSQL, b.DefaultSQL) {
			return false
		}
		return true
	}
	return false
}

// isImplicitSerialDefault returns true when one side is empty (no desired default) and
// the other is a nextval('...') expression that postgres injects for serial/bigserial columns.
func isImplicitSerialDefault(a, b string) bool {
	na, nb := normDef(a), normDef(b)
	if na == "" && strings.HasPrefix(nb, "nextval(") {
		return true
	}
	if nb == "" && strings.HasPrefix(na, "nextval(") {
		return true
	}
	return false
}

func altersFor(schema, table string, w, h *schema.Column) []change {
	if w == nil {
		return nil
	}
	var o []change
	if normType(h.TypeSQL) != normType(w.TypeSQL) {
		o = append(o, change{kind: plan.ChangeAlterColumn, sch: schema, tbl: table, col: w.Name, dc: w, lc: h, alterKind: "type"})
	}
	if h.NotNull != w.NotNull {
		o = append(o, change{kind: plan.ChangeAlterColumn, sch: schema, tbl: table, col: w.Name, dc: w, lc: h, alterKind: "notnull"})
	}
	if normDef(h.DefaultSQL) != normDef(w.DefaultSQL) && !isImplicitSerialDefault(h.DefaultSQL, w.DefaultSQL) {
		o = append(o, change{kind: plan.ChangeAlterColumn, sch: schema, tbl: table, col: w.Name, dc: w, lc: h, alterKind: "def"})
	}
	if h.Identity != w.Identity {
		o = append(o, change{kind: plan.ChangeAlterColumn, sch: schema, tbl: table, col: w.Name, dc: w, lc: h, alterKind: "identity"})
	}
	return o
}

func normType(s string) string { return schema.NormalizeTypeForCompare(s) }

// normDef normalises a default expression for comparison.
// PostgreSQL stores typed literals like 'active'::user_status or ('draft'::text)::post_status
// in the catalog, while the desired schema specifies bare literals like 'active'.
// We round-trip through the pg_query deparser (stripping all type casts) so both forms compare equal.
func normDef(s string) string {
	return normExprForCompare(s)
}

func buildStatements(ch []change, _ *schema.SchemaState, _ *schema.SchemaState, opt Options) []plan.Statement {
	var st []plan.Statement
	for _, c := range ch {
		st = append(st, stmtFor(c, opt)...)
	}
	return st
}

func stmtFor(c change, opt Options) []plan.Statement {
	qt := func(s, t string) string { return fmt.Sprintf("%s.%s", ident(s), ident(t)) }
	switch c.kind {
	case plan.ChangeCreateTable:
		if c.t == nil {
			return nil
		}
		return []plan.Statement{{
			OpType: string(c.kind), DDL: createTableSQL(c.t), Object: schema.TableKey(c.sch, c.tbl),
		}}
	case plan.ChangeDropTable:
		ddl := fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", qt(c.sch, c.tbl))
		return []plan.Statement{{
			OpType: string(c.kind), DDL: ddl, Object: schema.TableKey(c.sch, c.tbl),
			Hazards: []hazard.Detected{{Type: hazard.DataLoss, Severity: hazard.SeverityBlocking, Message: "Drops table and dependent objects"}},
		}}
	case plan.ChangeRenameTable:
		ddl := fmt.Sprintf("ALTER TABLE %s RENAME TO %s", qt(c.sch, c.fromTable), ident(c.tbl))
		return []plan.Statement{{OpType: string(c.kind), DDL: ddl, Object: schema.TableKey(c.sch, c.tbl)}}
	case plan.ChangeAddColumn:
		dc := c.dc
		ddl := fmt.Sprintf("ALTER TABLE %s ADD COLUMN IF NOT EXISTS %s %s", qt(c.sch, c.tbl), ident(c.col), dc.TypeSQL)
		if dc.GeneratedExpr != "" {
			ddl += fmt.Sprintf(" GENERATED ALWAYS AS (%s) STORED", dc.GeneratedExpr)
		} else {
			if dc.NotNull {
				ddl += " NOT NULL"
			}
			if dc.DefaultSQL != "" {
				ddl += " DEFAULT " + dc.DefaultSQL
			}
		}
		return []plan.Statement{{OpType: string(c.kind), DDL: ddl, Object: schema.TableKey(c.sch, c.tbl) + "." + c.col}}
	case plan.ChangeDropColumn:
		ddl := fmt.Sprintf("ALTER TABLE %s DROP COLUMN IF EXISTS %s CASCADE", qt(c.sch, c.tbl), ident(c.col))
		return []plan.Statement{{
			OpType: string(c.kind), DDL: ddl, Object: schema.TableKey(c.sch, c.tbl) + "." + c.col,
			Hazards: []hazard.Detected{{Type: hazard.DataLoss, Severity: hazard.SeverityBlocking, Message: "Drops column data"}},
		}}
	case plan.ChangeRenameColumn:
		ddl := fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s", qt(c.sch, c.tbl), ident(c.from), ident(c.col))
		return []plan.Statement{{OpType: string(c.kind), DDL: ddl, Object: schema.TableKey(c.sch, c.tbl) + "." + c.col}}
	case plan.ChangeAlterColumn:
		return alterStmt(c)
	case plan.ChangeToggleRLS:
		var st []plan.Statement
		if c.wantRls {
			st = append(st, plan.Statement{OpType: string(c.kind), DDL: fmt.Sprintf("ALTER TABLE %s ENABLE ROW LEVEL SECURITY", qt(c.sch, c.tbl)), Object: schema.TableKey(c.sch, c.tbl)})
		} else {
			st = append(st, plan.Statement{OpType: string(c.kind), DDL: fmt.Sprintf("ALTER TABLE %s DISABLE ROW LEVEL SECURITY", qt(c.sch, c.tbl)), Object: schema.TableKey(c.sch, c.tbl)})
		}
		if c.wantRls && c.wantForce {
			st = append(st, plan.Statement{OpType: string(c.kind) + "_FORCE", DDL: fmt.Sprintf("ALTER TABLE %s FORCE ROW LEVEL SECURITY", qt(c.sch, c.tbl)), Object: schema.TableKey(c.sch, c.tbl)})
		} else if c.wantRls && !c.wantForce {
			st = append(st, plan.Statement{OpType: string(c.kind) + "_NOFORCE", DDL: fmt.Sprintf("ALTER TABLE %s NO FORCE ROW LEVEL SECURITY", qt(c.sch, c.tbl)), Object: schema.TableKey(c.sch, c.tbl)})
		}
		return st
	case plan.ChangeCreateIndex:
		if c.idx == nil {
			return nil
		}
		ddl := rewriteIndexConcurrent(c.idx.CreateSQL)
		// Partitioned tables do not support CONCURRENTLY; strip it.
		if c.skipConcurrent {
			ddl = strings.ReplaceAll(ddl, " CONCURRENTLY", "")
		}
		st := plan.Statement{
			OpType:       string(c.kind),
			DDL:          ddl,
			Object:       schema.IndexKey(c.idx.Schema, c.idx.Name),
			IsConcurrent: !c.skipConcurrent && strings.Contains(strings.ToUpper(ddl), "CONCURRENTLY"),
			Hazards:      []hazard.Detected{{Type: hazard.IndexRebuild, Severity: hazard.SeverityAdvisory, Message: "Index build (concurrent) may affect I/O"}},
		}
		return []plan.Statement{st}
	case plan.ChangeDropIndex:
		ddl := fmt.Sprintf("DROP INDEX CONCURRENTLY IF EXISTS %s.%s", ident(c.sch), ident(c.ixName))
		return []plan.Statement{{
			OpType:       string(c.kind),
			DDL:          ddl,
			Object:       c.dropIdx,
			IsConcurrent: true,
			Hazards:      []hazard.Detected{{Type: hazard.IndexRebuild, Severity: hazard.SeverityAdvisory}},
		}}
	case plan.ChangeCreateFunction:
		if c.fn == nil {
			return nil
		}
		op := string(c.kind)
		kind := ""
		switch c.fn.Kind {
		case "a":
			op, kind = "CREATE_AGGREGATE", "aggregate"
		case "w":
			op, kind = "CREATE_WINDOW_FUNCTION", "window"
		default:
			kind = "function"
		}
		return []plan.Statement{{OpType: op, DDL: c.fn.DefSQL, Object: c.fn.Identity, ObjectKind: kind}}
	case plan.ChangeDropFunction:
		if c.fn == nil {
			return nil
		}
		ddl := fmt.Sprintf("DROP FUNCTION IF EXISTS %s CASCADE", c.fn.Identity)
		return []plan.Statement{{
			OpType:  string(c.kind),
			DDL:     ddl,
			Object:  c.fn.Identity,
			Hazards: []hazard.Detected{{Type: hazard.FunctionSignatureChange, Severity: hazard.SeverityBlocking, Message: "Drops function"}},
		}}
	case plan.ChangeCreatePolicy:
		if c.pol == nil {
			return nil
		}
		return []plan.Statement{{OpType: string(c.kind), DDL: c.pol.DefSQL, Object: c.polKey}}
	case plan.ChangeAlterPolicy:
		if c.pol == nil {
			return nil
		}
		return []plan.Statement{{OpType: string(c.kind), DDL: buildAlterPolicySQL(c.pol), Object: c.polKey}}
	case plan.ChangeDropPolicy:
		if c.pol == nil {
			return nil
		}
		ddl := fmt.Sprintf("DROP POLICY IF EXISTS %s ON %s.%s", ident(c.pol.Name), ident(c.pol.Schema), ident(c.pol.Table))
		return []plan.Statement{{
			OpType:  string(c.kind),
			DDL:     ddl,
			Object:  c.polKey,
			Hazards: []hazard.Detected{{Type: hazard.DataLoss, Severity: hazard.SeverityAdvisory, Message: "Drops RLS policy"}},
		}}
	case plan.ChangeAddConstraint:
		def := c.conDef
		hasNotValid := strings.Contains(strings.ToLower(def), "not valid")
		// Auto-rewrite (PRD P3-14): for CHECK and FOREIGN KEY, append NOT VALID so the ADD
		// only takes a short AccessExclusive lock; the long scan happens later under the
		// less-blocking ShareUpdateExclusive lock of VALIDATE CONSTRAINT.
		needsValidate := false
		isCheckOrFK := c.conKind == "c" || c.conKind == "f"
		if opt.AutoConstraintNotValid && isCheckOrFK && !hasNotValid {
			def = strings.TrimRight(def, " ;") + " NOT VALID"
			needsValidate = true
		}
		ddl := fmt.Sprintf("ALTER TABLE %s.%s ADD CONSTRAINT %s %s", ident(c.sch), ident(c.tbl), ident(c.conName), def)
		out := []plan.Statement{{
			OpType: string(c.kind), DDL: ddl, Object: schema.ConstraintKey(c.sch, c.tbl, c.conName),
			Hazards: []hazard.Detected{{Type: hazard.ConstraintScan, Severity: hazard.SeverityBlocking, Message: "Adding constraint may scan table"}},
		}}
		// Emit a follow-up VALIDATE CONSTRAINT when:
		//   - we just auto-appended NOT VALID, OR
		//   - the user wrote NOT VALID in source and explicitly opted into the synthetic follow-up.
		if needsValidate || (opt.AppendValidateAfterNotValid && hasNotValid) {
			vddl := fmt.Sprintf("ALTER TABLE %s.%s VALIDATE CONSTRAINT %s", ident(c.sch), ident(c.tbl), ident(c.conName))
			out = append(out, plan.Statement{
				OpType: "VALIDATE_TABLE_CONSTRAINT", DDL: vddl, Object: schema.ConstraintKey(c.sch, c.tbl, c.conName),
				IsConcurrent: true, // ShareUpdateExclusive lock; safer outside the main txn for large tables
				Hazards:      []hazard.Detected{{Type: hazard.ValidateConstraintScan, Severity: hazard.SeverityAdvisory, Message: "Validates constraint under ShareUpdateExclusive lock"}},
			})
		}
		return out
	case plan.ChangeDropConstraint:
		ddl := fmt.Sprintf("ALTER TABLE %s.%s DROP CONSTRAINT IF EXISTS %s", ident(c.sch), ident(c.tbl), ident(c.conName))
		return []plan.Statement{{
			OpType:  string(c.kind),
			DDL:     ddl,
			Object:  schema.ConstraintKey(c.sch, c.tbl, c.conName),
			Hazards: []hazard.Detected{{Type: hazard.DataLoss, Severity: hazard.SeverityBlocking, Message: "Drops constraint"}},
		}}
	case plan.ChangeRenameConstraint:
		ddl := fmt.Sprintf("ALTER TABLE %s.%s RENAME CONSTRAINT %s TO %s",
			ident(c.sch), ident(c.tbl), ident(c.from), ident(c.conName))
		return []plan.Statement{{
			OpType: string(c.kind),
			DDL:    ddl,
			Object: schema.ConstraintKey(c.sch, c.tbl, c.conName),
		}}
	case plan.ChangeCreateView:
		if c.v == nil {
			return nil
		}
		opType := string(c.kind)
		if c.v.Materialized {
			opType = "CREATE_MATERIALIZED_VIEW"
		}
		return []plan.Statement{{OpType: opType, DDL: c.v.DefSQL, Object: schema.ViewKey(c.v.Schema, c.v.Name)}}
	case plan.ChangeDropView, plan.ChangeDropViewEarly:
		if c.v == nil {
			return nil
		}
		var ddl string
		if c.v.Materialized {
			ddl = fmt.Sprintf("DROP MATERIALIZED VIEW IF EXISTS %s.%s CASCADE", ident(c.v.Schema), ident(c.v.Name))
		} else {
			ddl = fmt.Sprintf("DROP VIEW IF EXISTS %s.%s CASCADE", ident(c.v.Schema), ident(c.v.Name))
		}
		return []plan.Statement{{
			OpType:  string(c.kind),
			DDL:     ddl,
			Object:  c.viewKey,
			Hazards: []hazard.Detected{{Type: hazard.DataLoss, Severity: hazard.SeverityBlocking, Message: "Drops view"}},
		}}
	case plan.ChangeCreateSequence:
		if c.seq == nil {
			return nil
		}
		return []plan.Statement{{OpType: string(c.kind), DDL: c.seq.DefSQL, Object: schema.SeqKey(c.seq.Schema, c.seq.Name)}}
	case plan.ChangeDropSequence:
		if c.seq == nil {
			return nil
		}
		ddl := fmt.Sprintf("DROP SEQUENCE IF EXISTS %s.%s CASCADE", ident(c.seq.Schema), ident(c.seq.Name))
		return []plan.Statement{{
			OpType:  string(c.kind),
			DDL:     ddl,
			Object:  c.dropSeq,
			Hazards: []hazard.Detected{{Type: hazard.DataLoss, Severity: hazard.SeverityBlocking, Message: "Drops sequence"}},
		}}
	case plan.ChangeCreateExtension:
		if c.ext == nil {
			return nil
		}
		ddl := strings.TrimSpace(c.ext.DefSQL)
		if ddl == "" {
			ddl = "CREATE EXTENSION IF NOT EXISTS " + ident(c.ext.Name)
		}
		return []plan.Statement{{
			OpType: string(c.kind), DDL: ddl, Object: c.ext.Name,
			Hazards: []hazard.Detected{{Type: hazard.NotReplicaSafe, Severity: hazard.SeverityAdvisory, Message: "Extension install may not be replica-safe"}},
		}}
	case plan.ChangeDropExtension:
		if c.ext == nil && c.dropExt == "" {
			return nil
		}
		name := c.dropExt
		if c.ext != nil && c.ext.Name != "" {
			name = c.ext.Name
		}
		ddl := "DROP EXTENSION IF EXISTS " + ident(name) + " CASCADE"
		return []plan.Statement{{
			OpType:  string(c.kind),
			DDL:     ddl,
			Object:  name,
			Hazards: []hazard.Detected{{Type: hazard.DataLoss, Severity: hazard.SeverityBlocking, Message: "Drops extension and dependent objects"}},
		}}
	case plan.ChangeUpdateExtension:
		if c.ext == nil || strings.TrimSpace(c.ext.Version) == "" {
			return nil
		}
		ddl := fmt.Sprintf("ALTER EXTENSION %s UPDATE TO %s", ident(c.ext.Name), quoteSQLString(c.ext.Version))
		msg := "Extension version upgrade"
		if c.extLiveVer != "" {
			msg = fmt.Sprintf("Upgrade extension from %s to %s", c.extLiveVer, c.ext.Version)
		}
		return []plan.Statement{{
			OpType: string(c.kind), DDL: ddl, Object: c.ext.Name,
			Hazards: []hazard.Detected{{Type: hazard.NotReplicaSafe, Severity: hazard.SeverityAdvisory, Message: msg}},
		}}
	case plan.ChangeCreateType:
		if strings.TrimSpace(c.rawSQL) == "" {
			return nil
		}
		// Extract the type/domain name for the Object field (for plan output / logs).
		obj := "raw"
		if m := reExtractTypeName.FindStringSubmatch(c.rawSQL); len(m) == 3 {
			ns := m[1]
			if ns == "" {
				ns = "public"
			}
			obj = ns + "." + strings.ToLower(m[2])
		}
		return []plan.Statement{{
			OpType: string(c.kind), DDL: c.rawSQL, Object: obj,
		}}
	case plan.ChangeRawSQL:
		// A ChangeRawSQL with empty SQL but extraHazards is a pure advisory notice.
		if strings.TrimSpace(c.rawSQL) == "" {
			if len(c.extraHazards) > 0 {
				return []plan.Statement{{
					OpType:  string(c.kind),
					DDL:     "",
					Object:  schema.TableKey(c.sch, c.tbl),
					Hazards: c.extraHazards,
				}}
			}
			return nil
		}
		hazards := c.extraHazards
		if len(hazards) == 0 {
			hazards = []hazard.Detected{
				{Type: hazard.TableLock, Severity: hazard.SeverityAdvisory, Message: "Pass-through DDL: verify idempotency and lock impact"},
			}
		}
		return []plan.Statement{{
			OpType:       string(c.kind),
			DDL:          c.rawSQL,
			Object:       "raw",
			IsConcurrent: c.rawConcurrent,
			Hazards:      hazards,
		}}
	case plan.ChangeCreateTrigger:
		if c.trig == nil {
			return nil
		}
		oid := schema.TriggerKey(c.trig.Schema, c.trig.Table, c.trig.Name)
		return []plan.Statement{{OpType: string(c.kind), DDL: c.trig.DefSQL, Object: oid}}
	case plan.ChangeDropTrigger:
		if c.trig == nil {
			return nil
		}
		ddl := fmt.Sprintf("DROP TRIGGER IF EXISTS %s ON %s.%s CASCADE", ident(c.trig.Name), ident(c.trig.Schema), ident(c.trig.Table))
		return []plan.Statement{{
			OpType:  string(c.kind),
			DDL:     ddl,
			Object:  c.trigKey,
			Hazards: []hazard.Detected{{Type: hazard.DataLoss, Severity: hazard.SeverityBlocking, Message: "Drops trigger"}},
		}}
	}
	return nil
}

func alterStmt(c change) []plan.Statement {
	var out []plan.Statement
	w := c.dc
	if w == nil {
		return nil
	}
	switch c.alterKind {
	case "type":
		// Use the column's @using annotation when provided (required for incompatible casts
		// like boolean→enum where no implicit cast path exists).
		// Fall back to "col::newtype" for common widening casts (int→bigint, varchar→text).
		usingExpr := ident(c.col) + "::" + w.TypeSQL
		if w.CustomUsing != "" {
			usingExpr = w.CustomUsing
		}
		// If the live column has a non-serial default, PostgreSQL will refuse to change the
		// type because it cannot implicitly cast the default expression.  Drop the default
		// first; restore it after the type change either via altersFor's "def" alter (when
		// the desired default differs) or via an explicit SET DEFAULT here (when defs match
		// — altersFor emits no "def" alter, so without this the default is silently lost).
		droppedLiveDefault := false
		if c.lc != nil && c.lc.DefaultSQL != "" && !strings.HasPrefix(normDef(c.lc.DefaultSQL), "nextval(") {
			dropDef := fmt.Sprintf("ALTER TABLE %s.%s ALTER COLUMN %s DROP DEFAULT",
				ident(c.sch), ident(c.tbl), ident(c.col))
			out = append(out, plan.Statement{
				OpType: "ALTER_DEFAULT",
				DDL:    dropDef,
				Object: schema.TableKey(c.sch, c.tbl) + "." + c.col,
			})
			droppedLiveDefault = true
		}
		ddl := fmt.Sprintf("ALTER TABLE %s.%s ALTER COLUMN %s SET DATA TYPE %s USING %s",
			ident(c.sch), ident(c.tbl), ident(c.col), w.TypeSQL, usingExpr)
		out = append(out, plan.Statement{
			OpType:  "ALTER_COLUMN_TYPE",
			DDL:     ddl,
			Object:  schema.TableKey(c.sch, c.tbl) + "." + c.col,
			Hazards: []hazard.Detected{{Type: hazard.ColumnTypeChange, Severity: hazard.SeverityBlocking, Message: "Column type change may rewrite table"}},
		})
		if droppedLiveDefault && w.DefaultSQL != "" && c.lc != nil {
			defsMatch := normDef(c.lc.DefaultSQL) == normDef(w.DefaultSQL) || isImplicitSerialDefault(c.lc.DefaultSQL, w.DefaultSQL)
			if defsMatch {
				setDef := fmt.Sprintf("ALTER TABLE %s.%s ALTER COLUMN %s SET DEFAULT %s",
					ident(c.sch), ident(c.tbl), ident(c.col), w.DefaultSQL)
				out = append(out, plan.Statement{
					OpType: "ALTER_DEFAULT",
					DDL:    setDef,
					Object: schema.TableKey(c.sch, c.tbl) + "." + c.col,
				})
			}
		}
	case "notnull":
		if w.NotNull {
			ddl := fmt.Sprintf("ALTER TABLE %s.%s ALTER COLUMN %s SET NOT NULL", ident(c.sch), ident(c.tbl), ident(c.col))
			tk := schema.TableKey(c.sch, c.tbl)
			out = append(out, plan.Statement{OpType: "SET_NOT_NULL", DDL: ddl, Object: tk, Column: c.col, Hazards: []hazard.Detected{{Type: hazard.ConstraintScan, Severity: hazard.SeverityBlocking}}})
		} else {
			ddl := fmt.Sprintf("ALTER TABLE %s.%s ALTER COLUMN %s DROP NOT NULL", ident(c.sch), ident(c.tbl), ident(c.col))
			out = append(out, plan.Statement{OpType: "DROP_NOT_NULL", DDL: ddl, Object: schema.TableKey(c.sch, c.tbl)})
		}
	case "def":
		ddl := fmt.Sprintf("ALTER TABLE %s.%s ALTER COLUMN %s", ident(c.sch), ident(c.tbl), ident(c.col))
		if w.DefaultSQL == "" {
			ddl += " DROP DEFAULT"
		} else {
			ddl += " SET DEFAULT " + w.DefaultSQL
		}
		out = append(out, plan.Statement{OpType: "ALTER_DEFAULT", DDL: ddl, Object: schema.TableKey(c.sch, c.tbl) + "." + c.col})
	case "identity":
		base := fmt.Sprintf("ALTER TABLE %s.%s ALTER COLUMN %s", ident(c.sch), ident(c.tbl), ident(c.col))
		objKey := schema.TableKey(c.sch, c.tbl) + "." + c.col
		switch {
		case c.lc != nil && c.lc.Identity == "" && w.Identity != "":
			// Add identity to a previously-plain column.
			out = append(out, plan.Statement{
				OpType: "ADD_IDENTITY",
				DDL:    fmt.Sprintf("%s ADD GENERATED %s AS IDENTITY", base, identityClause(w.Identity)),
				Object: objKey,
			})
		case c.lc != nil && c.lc.Identity != "" && w.Identity == "":
			// Drop identity entirely. IF EXISTS so reruns are safe.
			out = append(out, plan.Statement{
				OpType: "DROP_IDENTITY",
				DDL:    base + " DROP IDENTITY IF EXISTS",
				Object: objKey,
			})
		case c.lc != nil && c.lc.Identity != "" && w.Identity != "" && c.lc.Identity != w.Identity:
			// Flip between ALWAYS and BY DEFAULT.
			out = append(out, plan.Statement{
				OpType: "SET_IDENTITY",
				DDL:    fmt.Sprintf("%s SET GENERATED %s", base, identityClause(w.Identity)),
				Object: objKey,
			})
		}
	}
	return out
}

// identityClause maps the schema.Column.Identity value to the SQL keyword form.
func identityClause(kind string) string {
	switch kind {
	case "always":
		return "ALWAYS"
	case "by-default":
		return "BY DEFAULT"
	}
	return ""
}

func createTableSQL(t *schema.Table) string {
	if t == nil {
		return ""
	}
	var b strings.Builder
	if t.Unlogged {
		fmt.Fprintf(&b, "CREATE UNLOGGED TABLE IF NOT EXISTS %s.%s (\n", ident(t.Schema), ident(t.Name))
	} else {
		fmt.Fprintf(&b, "CREATE TABLE IF NOT EXISTS %s.%s (\n", ident(t.Schema), ident(t.Name))
	}
	for i, c := range t.Columns {
		if c == nil {
			continue
		}
		if i > 0 {
			b.WriteString(",\n")
		}
		fmt.Fprintf(&b, "  %s %s", ident(c.Name), c.TypeSQL)
		if c.Collation != "" {
			fmt.Fprintf(&b, " COLLATE %s", ident(c.Collation))
		}
		if c.Storage != "" {
			fmt.Fprintf(&b, " STORAGE %s", strings.ToUpper(c.Storage))
		}
		if c.Compression != "" {
			fmt.Fprintf(&b, " COMPRESSION %s", c.Compression)
		}
		if c.GeneratedExpr != "" {
			kind := "STORED"
			if c.GeneratedKind == "virtual" {
				kind = "VIRTUAL"
			}
			fmt.Fprintf(&b, " GENERATED ALWAYS AS (%s) %s", c.GeneratedExpr, kind)
		} else if c.Identity != "" {
			fmt.Fprintf(&b, " GENERATED %s AS IDENTITY", identityClause(c.Identity))
			if c.IdentitySequenceOptions != "" {
				fmt.Fprintf(&b, " (%s)", c.IdentitySequenceOptions)
			}
		} else if c.DefaultSQL != "" {
			fmt.Fprintf(&b, " DEFAULT %s", c.DefaultSQL)
		}
		if c.IsPrimaryKey {
			b.WriteString(" PRIMARY KEY")
		} else if c.NotNull && c.Identity == "" {
			// IDENTITY implies NOT NULL — don't double-emit.
			b.WriteString(" NOT NULL")
		}
	}
	for _, ck := range t.Checks {
		if ck == nil {
			continue
		}
		fmt.Fprintf(&b, ",\n  CONSTRAINT %s %s", ident(ck.Name), ck.DefSQL)
	}
	for _, u := range t.Uniques {
		if u == nil {
			continue
		}
		fmt.Fprintf(&b, ",\n  CONSTRAINT %s %s", ident(u.Name), u.DefSQL)
	}
	for _, ex := range t.Excludes {
		if ex == nil {
			continue
		}
		fmt.Fprintf(&b, ",\n  CONSTRAINT %s %s", ident(ex.Name), ex.DefSQL)
	}
	for _, fk := range t.ForeignKeys {
		if fk == nil {
			continue
		}
		fmt.Fprintf(&b, ",\n  CONSTRAINT %s %s", ident(fk.Name), fk.DefSQL)
	}
	// Multi-column primary key stored at table level (not inline on a single column).
	if len(t.PrimaryKeyCols) > 0 {
		cols := make([]string, len(t.PrimaryKeyCols))
		for i, c := range t.PrimaryKeyCols {
			cols[i] = ident(c)
		}
		fmt.Fprintf(&b, ",\n  PRIMARY KEY (%s)", strings.Join(cols, ", "))
	}
	b.WriteString("\n)")
	if len(t.ReLOptions) > 0 {
		fmt.Fprintf(&b, " WITH (%s)", strings.Join(t.ReLOptions, ", "))
	}
	if t.PartitionBy != "" {
		fmt.Fprintf(&b, " PARTITION BY %s", t.PartitionBy)
	}
	b.WriteString(";")
	return b.String()
}

func ident(s string) string {
	if s == "" {
		return `""`
	}
	for _, r := range s {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '_' {
			return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
		}
	}
	if pgReservedKeywords[s] {
		return `"` + s + `"`
	}
	return s
}

// pgReservedKeywords is the set of PostgreSQL reserved keywords that cannot be
// used as unquoted identifiers (column/table/constraint names).
// Source: https://www.postgresql.org/docs/current/sql-keywords-appendix.html (marked "reserved")
var pgReservedKeywords = map[string]bool{
	"all": true, "analyse": true, "analyze": true, "and": true, "any": true,
	"array": true, "as": true, "asc": true, "asymmetric": true, "authorization": true,
	"binary": true, "both": true, "case": true, "cast": true, "check": true,
	"collate": true, "collation": true, "column": true, "concurrently": true,
	"constraint": true, "create": true, "cross": true, "current_catalog": true,
	"current_date": true, "current_role": true, "current_schema": true,
	"current_time": true, "current_timestamp": true, "current_user": true,
	"default": true, "deferrable": true, "desc": true, "distinct": true,
	"do": true, "else": true, "end": true, "except": true, "false": true,
	"fetch": true, "for": true, "foreign": true, "freeze": true, "from": true,
	"full": true, "grant": true, "group": true, "having": true, "ilike": true,
	"in": true, "initially": true, "inner": true, "intersect": true, "into": true,
	"is": true, "isnull": true, "join": true, "lateral": true, "leading": true,
	"left": true, "like": true, "limit": true, "localtime": true, "localtimestamp": true,
	"natural": true, "not": true, "notnull": true, "null": true, "offset": true,
	"on": true, "only": true, "or": true, "order": true, "outer": true,
	"overlaps": true, "placing": true, "primary": true, "references": true,
	"returning": true, "right": true, "select": true, "session_user": true,
	"similar": true, "some": true, "symmetric": true, "table": true,
	"tablesample": true, "then": true, "to": true, "trailing": true,
	"true": true, "union": true, "unique": true, "user": true, "using": true,
	"variadic": true, "verbose": true, "when": true, "where": true,
	"window": true, "with": true,
}

func attachHazard(s *plan.Statement) {
	s.Hazards = append(s.Hazards, hazard.EnrichFromDDL(s.DDL)...)
	recomputeStatementBlocking(s)
}

func recomputeStatementBlocking(s *plan.Statement) {
	s.BlockingHazards = false
	for i := range s.Hazards {
		if s.Hazards[i].Severity == "" {
			s.Hazards[i].Severity = hazard.DefaultSeverity(s.Hazards[i].Type)
		}
		if s.Hazards[i].Severity != hazard.SeverityAdvisory {
			s.BlockingHazards = true
		}
	}
}

func enrichHazardsFromOptions(stmts *[]plan.Statement, opt Options) {
	if stmts == nil {
		return
	}
	if opt.Reltuples == nil || opt.SetNotNullReltupleThreshold <= 0 {
		return
	}
	var out []plan.Statement
	changed := false
	for _, s := range *stmts {
		if s.OpType != "SET_NOT_NULL" {
			out = append(out, s)
			continue
		}
		n, ok := opt.Reltuples[strings.ToLower(strings.TrimSpace(s.Object))]
		if !ok || n < opt.SetNotNullReltupleThreshold {
			out = append(out, s)
			continue
		}
		// Large table: replace the single SET NOT NULL with the 4-step staged plan.
		// Column may be raw name; sanitize to a valid identifier for the constraint name.
		colSanitized := strings.Map(func(r rune) rune {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
				return r
			}
			return '_'
		}, strings.ToLower(s.Column))
		conName := "pgflux_notnull_" + colSanitized
		tbl := s.Object // "schema.table" already formatted
		colIdent := ident(s.Column)
		checkExpr := colIdent + " IS NOT NULL"
		advisory := hazard.Detected{
			Type:     hazard.StagedSetNotNull,
			Severity: hazard.SeverityAdvisory,
			Message:  fmt.Sprintf("Large table (~%.0f est. rows): using staged SET NOT NULL (4-step safe pattern)", n),
		}
		// Step 1: ADD CONSTRAINT ... NOT VALID (short lock, in transaction)
		out = append(out, plan.Statement{
			OpType:  "STAGED_NOT_NULL_ADD_CONSTRAINT",
			Object:  tbl,
			Column:  s.Column,
			DDL:     fmt.Sprintf("ALTER TABLE %s ADD CONSTRAINT %s CHECK (%s) NOT VALID", tbl, conName, checkExpr),
			Hazards: []hazard.Detected{advisory},
		})
		// Step 2: VALIDATE CONSTRAINT (long scan, ShareUpdateExclusiveLock, run concurrent/outside tx)
		out = append(out, plan.Statement{
			OpType:       "STAGED_NOT_NULL_VALIDATE",
			Object:       tbl,
			Column:       s.Column,
			DDL:          fmt.Sprintf("ALTER TABLE %s VALIDATE CONSTRAINT %s", tbl, conName),
			IsConcurrent: true,
			Hazards:      []hazard.Detected{advisory},
		})
		// Step 3: SET NOT NULL (very fast now that constraint is validated, in transaction)
		out = append(out, plan.Statement{
			OpType:  "STAGED_NOT_NULL_SET",
			Object:  tbl,
			Column:  s.Column,
			DDL:     fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s SET NOT NULL", tbl, colIdent),
			Hazards: []hazard.Detected{advisory},
		})
		// Step 4: DROP helper constraint (in transaction)
		out = append(out, plan.Statement{
			OpType: "STAGED_NOT_NULL_DROP_CONSTRAINT",
			Object: tbl,
			Column: s.Column,
			DDL:    fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT IF EXISTS %s", tbl, conName),
		})
		changed = true
	}
	if changed {
		*stmts = out
	}
}

func renumber(s *[]plan.Statement) {
	for i := range *s {
		(*s)[i].ID = i + 1
		(*s)[i].LockTimeoutMS = 3000
		if (*s)[i].IsConcurrent {
			(*s)[i].StatementTimeoutMS = 20 * 60 * 1000
		} else {
			(*s)[i].StatementTimeoutMS = 3000
		}
	}
}

// rewriteIndexConcurrent turns CREATE [UNIQUE] INDEX into CONCURRENT form (PRD safe default).
// It also ensures IF NOT EXISTS is present for idempotency.
func rewriteIndexConcurrent(sql string) string {
	s := strings.TrimSpace(sql)
	if s == "" {
		return s
	}
	upper := strings.ToUpper(s)
	if strings.Contains(upper, "CONCURRENTLY") {
		return ensureIndexIfNotExists(s)
	}
	if strings.HasPrefix(upper, "CREATE UNIQUE INDEX ") {
		s = strings.Replace(s, "CREATE UNIQUE INDEX", "CREATE UNIQUE INDEX CONCURRENTLY", 1)
		// After replace, "CREATE UNIQUE INDEX" changed, but the case may differ; just search in-place.
		if idx := strings.Index(strings.ToUpper(s), "CREATE UNIQUE INDEX"); idx >= 0 {
			_ = idx
		}
	} else if strings.HasPrefix(upper, "CREATE INDEX ") {
		s = strings.Replace(s, "CREATE INDEX", "CREATE INDEX CONCURRENTLY", 1)
	}
	return ensureIndexIfNotExists(s)
}

// ensureIndexIfNotExists rewrites CREATE [UNIQUE] INDEX [CONCURRENTLY] name ...
// to add IF NOT EXISTS before the index name if not already present.
// Matches: CREATE INDEX [CONCURRENTLY] <name> → CREATE INDEX [CONCURRENTLY] IF NOT EXISTS <name>
var reIndexPrefix = regexp.MustCompile(`(?i)^(CREATE\s+(?:UNIQUE\s+)?INDEX\s+(?:CONCURRENTLY\s+)?)`)

func ensureIndexIfNotExists(sql string) string {
	loc := reIndexPrefix.FindStringIndex(sql)
	if loc == nil {
		return sql
	}
	rest := sql[loc[1]:]
	upper := strings.ToUpper(strings.TrimSpace(rest))
	if strings.HasPrefix(upper, "IF NOT EXISTS") {
		return sql // already present
	}
	return sql[:loc[1]] + "IF NOT EXISTS " + rest
}
