package src

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	pgq "github.com/pganalyze/pg_query_go/v6"
	"github.com/pganalyze/pg_query_go/v6/parser"

	"github.com/nexg/pg-flux/pkg/schema"
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
	default:
		return nil
	}
}

func addCreateTable(cs *pgq.CreateStmt, raw *pgq.RawStmt, content string, lines []string, st *schema.SchemaState) error {
	if cs == nil || cs.GetRelation() == nil {
		return nil
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
	createLine := lineIndex0ForByte(content, int(raw.GetStmtLocation()))
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
			loc := int(cd.GetLocation())
			cline := lineIndex0ForByte(content, loc)
			if p := previousNonEmptyLineIndex(lines, cline); p >= 0 {
				if from, ok := extractRenameFromComment(lineByIndex0(lines, p)); ok {
					col.RenameFrom = from
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
					t.Checks = append(t.Checks, &schema.TableCheck{Name: cn, DefSQL: def})
				case pgq.ConstrType_CONSTR_FOREIGN:
					t.ForeignKeys = append(t.ForeignKeys, &schema.TableForeignKey{Name: cn, DefSQL: def})
				case pgq.ConstrType_CONSTR_UNIQUE:
					t.Uniques = append(t.Uniques, &schema.TableUnique{Name: cn, DefSQL: def})
				case pgq.ConstrType_CONSTR_EXCLUSION:
					t.Excludes = append(t.Excludes, &schema.TableExclusion{Name: cn, DefSQL: def})
				}
			}
		}
	}
	if full, e := deparseOne(raw); e == nil {
		t.RLSEnabled, t.RLSForced = rlsFromCreateTableDeparse(full)
	}
	st.Tables[key] = t
	return nil
}
