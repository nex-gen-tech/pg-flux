package src

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	pgq "github.com/pganalyze/pg_query_go/v6"
	"github.com/pganalyze/pg_query_go/v6/parser"

	"github.com/nex-gen-tech/pg-flux/pkg/schema"
)

// LoadOptions configure schema loading.
type LoadOptions struct {
	SchemaDir  string
	SchemaFile string
	// ValidatePlpgsql runs pg_query's Pl/pgSQL parser on each CREATE FUNCTION whose deparse includes LANGUAGE plpgsql.
	ValidatePlpgsql bool
	// ValidateSQL re-parses each top-level statement (catches some edge cases beyond file-level parse).
	ValidateSQL bool
}

// LoadDesiredState loads and parses .sql into a SchemaState.
func LoadDesiredState(opt LoadOptions) (*schema.SchemaState, error) {
	var files []string
	if opt.SchemaFile != "" {
		files = []string{opt.SchemaFile}
	} else {
		base := opt.SchemaDir
		if base == "" {
			base = "./schema"
		}
		err := filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if strings.HasSuffix(strings.ToLower(path), ".sql") {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		sort.Strings(files)
	}
	st := &schema.SchemaState{
		Tables:    make(map[string]*schema.Table),
		Indexes:   make(map[string]*schema.Index),
		Functions: make(map[string]*schema.Function),
		Policies:  make(map[string]*schema.Policy),
		Views:     make(map[string]*schema.View),
		Sequences: make(map[string]*schema.Sequence),
		Triggers:  make(map[string]*schema.Trigger),
	}
	for _, f := range files {
		if err := parseFileInto(f, st, opt); err != nil {
			return nil, fmt.Errorf("%s: %w", f, err)
		}
	}
	// Second pass: apply deferred GRANT/REVOKE entries whose target was not loaded yet
	// when the grant file was parsed (e.g. grants.sql sorts before products.sql/views.sql).
	for _, pg := range st.PendingGrants {
		switch pg.ObjKind {
		case "table_or_view":
			key := schema.TableKey(pg.Schema, pg.Name)
			if t := st.Tables[key]; t != nil {
				t.Privileges = mergePrivileges(t.Privileges, pg.Privs, pg.Grantees, pg.WGO, pg.IsGrant)
			} else if v := st.Views[key]; v != nil {
				v.Privileges = mergePrivileges(v.Privileges, pg.Privs, pg.Grantees, pg.WGO, pg.IsGrant)
			}
		case "sequence":
			if s := st.Sequences[schema.SeqKey(pg.Schema, pg.Name)]; s != nil {
				s.Privileges = mergePrivileges(s.Privileges, pg.Privs, pg.Grantees, pg.WGO, pg.IsGrant)
			}
		case "function":
			if f := st.Functions[schema.FunctionKey(pg.Name)]; f != nil {
				f.Privileges = mergePrivileges(f.Privileges, pg.Privs, pg.Grantees, pg.WGO, pg.IsGrant)
			}
		}
	}
	st.PendingGrants = nil
	// Second pass: apply any pending RLS flags to tables that now exist
	// (ALTER TABLE ... ENABLE ROW LEVEL SECURITY may be in a file that sorts
	// before the CREATE TABLE file, so the table did not exist on first pass).
	for key, flags := range st.PendingRLS {
		if t := st.Tables[key]; t != nil {
			if flags.EnabledSet {
				t.RLSEnabled = flags.Enabled
			}
			if flags.ForcedSet {
				t.RLSForced = flags.Forced
			}
		}
	}
	st.PendingRLS = nil
	// Second pass: apply any buffered ALTER TABLE ... ENABLE/DISABLE TRIGGER directives
	// to triggers that now exist (CREATE TRIGGER may have been in a later-sorted file).
	for k, state := range st.PendingTriggerState {
		if tg := st.Triggers[k]; tg != nil {
			tg.Enabled = state
		}
	}
	st.PendingTriggerState = nil
	// Second pass: apply any buffered ALTER POLICY statements whose CREATE POLICY
	// was in a file that sorted after the ALTER POLICY file (cross-file ordering).
	for _, p := range st.PendingAlterPolicy {
		prev := st.Policies[p.Key]
		if prev == nil {
			return nil, fmt.Errorf("ALTER POLICY %q: CREATE POLICY not found in schema after all files loaded", p.Key)
		}
		if p.UsingSQL != "" {
			prev.UsingSQL = p.UsingSQL
		}
		if p.WithCheck != "" {
			prev.WithCheck = p.WithCheck
		}
		if len(p.Roles) > 0 {
			prev.Roles = p.Roles
		}
		prev.DefSQL = rebuildCreatePolicySQL(prev)
	}
	st.PendingAlterPolicy = nil
	return st, nil
}

func parseFileInto(path string, st *schema.SchemaState, opt LoadOptions) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	content := string(b)
	content = strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(content, "\n")

	result, err := pgq.Parse(content)
	if err != nil {
		return wrapParseError(content, err)
	}
	for _, raw := range result.GetStmts() {
		if err := processRawStmt(raw, content, lines, st); err != nil {
			return err
		}
		if err := processExtraNode(raw, st, opt); err != nil {
			return err
		}
		if opt.ValidateSQL {
			if sql, e := deparseOne(raw); e == nil {
				if e := CheckPostgresSQLParse(sql); e != nil {
					return fmt.Errorf("validate sql: %w", e)
				}
			}
		}
	}
	return nil
}

func wrapParseError(content string, err error) error {
	var pe *parser.Error
	if errors.As(err, &pe) {
		line := 1
		if pe.Cursorpos > 0 && pe.Cursorpos < len(content) {
			line = 1 + strings.Count(content[:pe.Cursorpos], "\n")
		} else if pe.Lineno > 0 {
			line = pe.Lineno
		}
		return fmt.Errorf("parse error at line %d: %v", line, err)
	}
	return fmt.Errorf("parse: %w", err)
}

func processRawStmt(raw *pgq.RawStmt, content string, lines []string, st *schema.SchemaState) error {
	if raw == nil || raw.GetStmt() == nil {
		return nil
	}
	switch n := raw.GetStmt().GetNode().(type) {
	case *pgq.Node_CreateStmt:
		return addCreateTable(n.CreateStmt, raw, content, lines, st)
	case *pgq.Node_ViewStmt:
		return captureView(n.ViewStmt, raw, st)
	case *pgq.Node_CreateSeqStmt:
		return captureSequence(n.CreateSeqStmt, raw, st)
	case *pgq.Node_CreateTrigStmt:
		return captureTrigger(n.CreateTrigStmt, raw, st)
	case *pgq.Node_CreateTableAsStmt:
		return captureMatView(n.CreateTableAsStmt, raw, st)
	case *pgq.Node_AlterTableStmt:
		return captureAlterTable(n.AlterTableStmt, raw, st)
	case *pgq.Node_RefreshMatViewStmt:
		// Pass through REFRESH MATERIALIZED VIEW [CONCURRENTLY] as ExtraDDL so the
		// user can manually trigger refreshes from declarative schema files. Each
		// REFRESH lives in ExtraDDL and is emitted in the next migration; subsequent
		// applies skip it (idempotent: REFRESH is always safe to re-run).
		sql, err := deparseOne(raw)
		if err != nil {
			return err
		}
		st.ExtraDDL = append(st.ExtraDDL, strings.TrimSpace(sql))
		return nil
	default:
		return nil
	}
}

// defElemValueToString unwraps a DefElem argument node into a SQL-rendered text value.
// Used for WITH (key = value, ...) options on CREATE TABLE / CREATE VIEW etc. The grammar
// allows strings, integers, floats, booleans, type names, and lists of nodes.
func defElemValueToString(n *pgq.Node) string {
	if n == nil {
		return ""
	}
	if s := n.GetString_(); s != nil {
		return s.GetSval()
	}
	if i := n.GetInteger(); i != nil {
		return fmt.Sprintf("%d", i.GetIval())
	}
	if f := n.GetFloat(); f != nil {
		return f.GetFval()
	}
	if b := n.GetBoolean(); b != nil {
		if b.GetBoolval() {
			return "true"
		}
		return "false"
	}
	if tn := n.GetTypeName(); tn != nil {
		s, _ := typeNameToSQL(tn)
		return s
	}
	if lst := n.GetList(); lst != nil {
		parts := make([]string, 0, len(lst.GetItems()))
		for _, it := range lst.GetItems() {
			if v := defElemValueToString(it); v != "" {
				parts = append(parts, v)
			}
		}
		return strings.Join(parts, ", ")
	}
	return ""
}

func addCreateTable(cs *pgq.CreateStmt, raw *pgq.RawStmt, content string, lines []string, st *schema.SchemaState) error {
	if cs == nil || cs.GetRelation() == nil {
		return nil
	}
	// Partition children (CREATE TABLE t PARTITION OF parent FOR VALUES ...) are not
	// modeled as first-class schema objects — they inherit structure from the parent.
	// Capture as ExtraDDL so they are applied as raw SQL in plan order.
	if cs.GetPartbound() != nil {
		return captureDeparsedExtraDDL(raw, st)
	}
	rv := cs.GetRelation()
	schemaName := rv.GetSchemaname()
	if schemaName == "" {
		schemaName = "public"
	}
	tableName := strings.ToLower(rv.GetRelname())
	if tableName == "" {
		return fmt.Errorf("CREATE TABLE: missing table name")
	}
	key := schema.TableKey(schemaName, tableName)
	if st.Tables[key] != nil {
		return fmt.Errorf("duplicate table definition for %q (conflict across files?)", key)
	}

	t := &schema.Table{
		Schema: schemaName,
		Name:   tableName,
	}
	// Persistence: 'u' = UNLOGGED, 't' = TEMPORARY, 'p' = PERMANENT (default).
	if rv.GetRelpersistence() == "u" {
		t.Unlogged = true
	}
	// WITH (key = val, ...) reloptions: each is a DefElem; render as "key=val".
	for _, opt := range cs.GetOptions() {
		el := opt.GetDefElem()
		if el == nil {
			continue
		}
		val := defElemValueToString(el.GetArg())
		if val == "" {
			continue
		}
		t.ReLOptions = append(t.ReLOptions, el.GetDefname()+"="+val)
	}
	// raw.GetStmtLocation() returns 0 for the first statement in a file even when
	// the CREATE TABLE keyword appears after leading comments (a pg_query quirk).
	// Scan forward from StmtLocation to find the actual "CREATE TABLE" keyword.
	stmtLoc := int(raw.GetStmtLocation())
	createKWOff := strings.Index(strings.ToUpper(content[stmtLoc:]), "CREATE TABLE")
	if createKWOff < 0 {
		createKWOff = 0
	}
	createLine := lineIndex0ForByte(content, stmtLoc+createKWOff)
	if p := previousNonEmptyLineIndex(lines, createLine); p >= 0 {
		ln := lineByIndex0(lines, p)
		if from, ok := extractRenameFromComment(ln); ok {
			t.OldName = from
		}
		if isDeprecatedTableComment(ln) {
			t.Deprecated = true
		}
	}
	for _, elt := range cs.GetTableElts() {
		if elt == nil {
			continue
		}
		if cd := elt.GetColumnDef(); cd != nil {
			col := &schema.Column{
				Name:    strings.ToLower(cd.GetColname()),
				NotNull: cd.GetIsNotNull(),
			}
			typ, err := typeNameToSQL(cd.GetTypeName())
			if err != nil {
				return fmt.Errorf("column %q: %w", col.Name, err)
			}
			col.TypeSQL = typ
			if cd.GetRawDefault() != nil {
				ds, err := defaultExprToSQL(cd.GetRawDefault())
				if err != nil {
					return err
				}
				col.DefaultSQL = strings.TrimSpace(ds)
			}
			// COLLATE clause on the column. Collation names are case-sensitive
			// in pg_collation ("C" and "c" are distinct), so we preserve case.
			if cc := cd.GetCollClause(); cc != nil {
				parts := cc.GetCollname()
				if len(parts) > 0 {
					last := parts[len(parts)-1].GetString_()
					if last != nil {
						col.Collation = last.GetSval()
					}
				}
			}
			// STORAGE — two encodings depending on source form:
			//   - ColumnDef.Storage  : one-char code matching pg_attribute.attstorage ('p','e','m','x')
			//   - ColumnDef.StorageName: PG16+ keyword form from `STORAGE <name>` clause in CREATE TABLE
			switch strings.ToLower(cd.GetStorageName()) {
			case "plain":
				col.Storage = "PLAIN"
			case "external":
				col.Storage = "EXTERNAL"
			case "main":
				col.Storage = "MAIN"
			case "extended":
				col.Storage = "EXTENDED"
			default:
				switch cd.GetStorage() {
				case "p":
					col.Storage = "PLAIN"
				case "e":
					col.Storage = "EXTERNAL"
				case "m":
					col.Storage = "MAIN"
				case "x":
					col.Storage = "EXTENDED"
				}
			}
			// COMPRESSION (PG14+): ColumnDef.compression is a free-form text.
			if comp := strings.ToLower(cd.GetCompression()); comp != "" {
				col.Compression = comp
			}
			loc := int(cd.GetLocation())
			cline := lineIndex0ForByte(content, loc)
			if p := previousNonEmptyLineIndex(lines, cline); p >= 0 {
				commentLine := lineByIndex0(lines, p)
				if from, ok := extractRenameFromComment(commentLine); ok {
					col.RenameFrom = from
				}
				if usingExpr, ok := extractUsingComment(commentLine); ok {
					col.CustomUsing = usingExpr
				}
			}
			for _, c := range cd.GetConstraints() {
				if c == nil {
					continue
				}
				if cc := c.GetConstraint(); cc != nil {
					switch cc.GetContype() {
					case pgq.ConstrType_CONSTR_PRIMARY:
						col.IsPrimaryKey = true
						col.NotNull = true // PRIMARY KEY implies NOT NULL
					case pgq.ConstrType_CONSTR_NOTNULL:
						col.NotNull = true
					case pgq.ConstrType_CONSTR_DEFAULT:
						if cc.GetRawExpr() != nil {
							ds, err := defaultExprToSQL(cc.GetRawExpr())
							if err != nil {
								return err
							}
							col.DefaultSQL = strings.TrimSpace(ds)
						}
					case pgq.ConstrType_CONSTR_FOREIGN:
						// Inline column-level REFERENCES: auto-generate a name following the
						// PostgreSQL convention of {table}_{column}_fkey.
						cn := strings.ToLower(t.Name) + "_" + strings.ToLower(col.Name) + "_fkey"
						def := buildInlineForeignKeySQL(col.Name, cc)
						t.ForeignKeys = append(t.ForeignKeys, &schema.TableForeignKey{Name: cn, DefSQL: def})
					case pgq.ConstrType_CONSTR_GENERATED:
						// GENERATED ALWAYS AS (expr) STORED — capture the expression.
						if cc.GetRawExpr() != nil {
							if genExpr, err := deparseExprToSQL(cc.GetRawExpr()); err == nil {
								col.GeneratedExpr = strings.TrimSpace(genExpr)
							}
						}
					case pgq.ConstrType_CONSTR_IDENTITY:
						// GENERATED ALWAYS / BY DEFAULT AS IDENTITY [(...)].
						// generated_when is 'a' (always) or 'd' (by default), matching pg_attribute.attidentity.
						switch cc.GetGeneratedWhen() {
						case "a":
							col.Identity = "always"
						case "d":
							col.Identity = "by-default"
						}
						// IDENTITY implies NOT NULL.
						col.NotNull = true
					}
				}
			}
			t.Columns = append(t.Columns, col)
		} else if cst := elt.GetConstraint(); cst != nil {
			switch cst.GetContype() {
			case pgq.ConstrType_CONSTR_PRIMARY:
				for _, k := range cst.GetKeys() {
					if k == nil {
						continue
					}
					if s := k.GetString_(); s != nil {
						t.PrimaryKeyCols = append(t.PrimaryKeyCols, strings.ToLower(s.GetSval()))
					}
				}
			case pgq.ConstrType_CONSTR_CHECK, pgq.ConstrType_CONSTR_FOREIGN, pgq.ConstrType_CONSTR_UNIQUE, pgq.ConstrType_CONSTR_EXCLUSION:
				cn, def, err := constraintToTableSQL(cst)
				if err != nil {
					return fmt.Errorf("table %s constraint: %w", key, err)
				}
				if cn == "" {
					return fmt.Errorf("table %s: named table constraints (CHECK, FOREIGN KEY, UNIQUE, EXCLUDE) require CONSTRAINT name", key)
				}
				switch cst.GetContype() {
				case pgq.ConstrType_CONSTR_CHECK:
					t.Checks = append(t.Checks, &schema.TableCheck{
						Name: cn, DefSQL: def,
						Deferrable:        cst.GetDeferrable(),
						InitiallyDeferred: cst.GetInitdeferred(),
					})
				case pgq.ConstrType_CONSTR_FOREIGN:
					match := ""
					switch strings.ToLower(cst.GetFkMatchtype()) {
					case "f":
						match = "FULL"
					case "p":
						match = "PARTIAL"
					}
					t.ForeignKeys = append(t.ForeignKeys, &schema.TableForeignKey{
						Name: cn, DefSQL: def,
						Deferrable:        cst.GetDeferrable(),
						InitiallyDeferred: cst.GetInitdeferred(),
						MatchType:         match,
					})
				case pgq.ConstrType_CONSTR_UNIQUE:
					t.Uniques = append(t.Uniques, &schema.TableUnique{
						Name: cn, DefSQL: def,
						Deferrable:        cst.GetDeferrable(),
						InitiallyDeferred: cst.GetInitdeferred(),
						NullsNotDistinct:  cst.GetNullsNotDistinct(),
					})
				case pgq.ConstrType_CONSTR_EXCLUSION:
					t.Excludes = append(t.Excludes, &schema.TableExclusion{Name: cn, DefSQL: def})
				}
			}
		}
	}
	if full, e := deparseOne(raw); e == nil {
		t.RLSEnabled, t.RLSForced = rlsFromCreateTableDeparse(full)
		// Extract PARTITION BY clause (e.g. "RANGE (ts)") for partitioned tables.
		if cs.GetPartspec() != nil {
			upper := strings.ToUpper(full)
			if idx := strings.Index(upper, "PARTITION BY "); idx >= 0 {
				spec := strings.TrimSpace(full[idx+len("PARTITION BY "):])
				spec = strings.TrimSuffix(spec, ";")
				t.PartitionBy = strings.TrimSpace(spec)
			}
		}
	}
	// Fix: after parsing @renamed hints on columns, update any constraint DefSQL that
	// still references the old column name. This ensures the source model's constraint
	// definition matches what the live DB will have after the rename is applied — so
	// subsequent calls to "migrate generate" do not emit spurious DROP/ADD CONSTRAINT.
	applyColumnRenameHintsToConstraints(t)
	st.Tables[key] = t
	return nil
}

// applyColumnRenameHintsToConstraints rewrites constraint DefSQL fields on t to replace
// any old column name (from a @renamed hint) with the new column name. This keeps the
// source model's constraint definitions in sync with what the live DB has after the
// rename migration has been applied.
func applyColumnRenameHintsToConstraints(t *schema.Table) {
	if t == nil {
		return
	}
	// Collect old->new mappings from columns that carry a @renamed hint.
	type renameEntry struct {
		re      *regexp.Regexp
		newName string
	}
	var renames []renameEntry
	for _, col := range t.Columns {
		if col == nil || col.RenameFrom == "" {
			continue
		}
		re := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(col.RenameFrom) + `\b`)
		renames = append(renames, renameEntry{re: re, newName: col.Name})
	}
	if len(renames) == 0 {
		return
	}
	applyRenames := func(s string) string {
		for _, r := range renames {
			s = r.re.ReplaceAllString(s, r.newName)
		}
		return s
	}
	for _, c := range t.Checks {
		if c != nil {
			c.DefSQL = applyRenames(c.DefSQL)
		}
	}
	for _, c := range t.Uniques {
		if c != nil {
			c.DefSQL = applyRenames(c.DefSQL)
		}
	}
	for _, c := range t.Excludes {
		if c != nil {
			c.DefSQL = applyRenames(c.DefSQL)
		}
	}
	for _, c := range t.ForeignKeys {
		if c != nil {
			c.DefSQL = applyRenames(c.DefSQL)
		}
	}
}
