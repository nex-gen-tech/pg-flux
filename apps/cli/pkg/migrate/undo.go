package migrate

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/nex-gen-tech/pg-flux/pkg/plan"
)

// WriteUndoFile generates an undo SQL file alongside the forward migration.
// The undo file is named <basename>_undo.sql.
// Returns the path of the written file.
func WriteUndoFile(res *GenerateResult) (string, error) {
	if res == nil || res.Filename == "" {
		return "", nil
	}
	undoSQL := GenerateUndoSQL(&plan.ExecutionPlan{Statements: res.Statements})
	dir := filepath.Dir(res.Filename)
	base := filepath.Base(res.Filename)
	// Strip .sql suffix, append _undo.sql
	undoBase := strings.TrimSuffix(base, ".sql") + "_undo.sql"
	undoPath := filepath.Join(dir, undoBase)
	if err := os.WriteFile(undoPath, []byte(undoSQL), 0o644); err != nil {
		return "", fmt.Errorf("write undo file: %w", err)
	}
	return undoPath, nil
}

// GenerateUndoSQL generates a best-effort undo/rollback script for the given
// execution plan. The returned SQL, when applied after the forward migration,
// attempts to restore the previous schema state.
//
// Not all operations can be automatically reversed; those that cannot are emitted
// as SQL comments with a "MANUAL UNDO REQUIRED" marker.
func GenerateUndoSQL(p *plan.ExecutionPlan) string {
	if p == nil || len(p.Statements) == 0 {
		return "-- pg-flux undo: nothing to undo\n"
	}

	var b strings.Builder
	b.WriteString("-- pg-flux generated UNDO migration\n")
	b.WriteString("-- Review carefully before applying. Some operations cannot be auto-reversed.\n\n")

	// Collect all undo statements in reverse order (last forward = first undo).
	type undoEntry struct {
		stmt  string
		notes string // if non-empty, this is a manual-undo comment
	}
	var entries []undoEntry

	for i := len(p.Statements) - 1; i >= 0; i-- {
		s := p.Statements[i]
		if s.DDL == "" {
			continue
		}
		undo, manual := undoStatement(s)
		entries = append(entries, undoEntry{stmt: undo, notes: manual})
	}

	// Split into manual and automatic
	var regular, manuals []undoEntry
	for _, e := range entries {
		if e.notes != "" {
			manuals = append(manuals, e)
		} else {
			regular = append(regular, e)
		}
	}

	// Manual undo notices first
	if len(manuals) > 0 {
		b.WriteString("-- ============================================================\n")
		b.WriteString("-- MANUAL UNDO REQUIRED for the following operations:\n")
		b.WriteString("-- ============================================================\n")
		for _, e := range manuals {
			fmt.Fprintf(&b, "-- %s\n", e.notes)
		}
		b.WriteString("\n")
	}

	if len(regular) == 0 {
		b.WriteString("-- No automatically-reversible statements.\n")
		return b.String()
	}

	b.WriteString("BEGIN;\n\n")
	for _, e := range regular {
		stmt := strings.TrimRight(e.stmt, ";")
		b.WriteString(stmt)
		b.WriteString(";\n\n")
	}
	b.WriteString("COMMIT;\n")

	return b.String()
}

// reObjectName matches a schema-qualified or bare identifier from DDL Object field.
// Object is typically "schema.table" or "schema.table.column".
var reSchemaTable = regexp.MustCompile(`^([^.]+)\.([^.]+)$`)
var reSchemaTableCol = regexp.MustCompile(`^([^.]+)\.([^.]+)\.([^.]+)$`)

// undoStatement returns the undo DDL for a single forward statement.
// Returns (ddl, "") for auto-reversible ops, or ("", note) for manual ones.
func undoStatement(s plan.Statement) (ddl string, manual string) {
	obj := s.Object

	switch plan.ChangeType(s.OpType) {

	// Table-level
	case plan.ChangeCreateTable:
		if m := reSchemaTable.FindStringSubmatch(obj); m != nil {
			return fmt.Sprintf(`DROP TABLE IF EXISTS "%s"."%s" CASCADE`, m[1], m[2]), ""
		}
		return fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", obj), ""

	case plan.ChangeDropTable:
		return "", fmt.Sprintf("MANUAL: re-create dropped table %s (original DDL not stored)", obj)

	case plan.ChangeRenameTable:
		// Object = "schema.new_name", DDL contains "RENAME ... TO new_name"
		// Parse "ALTER TABLE schema.old TO new" — extract old from DDL.
		if old, newName, ok := parseRenameTableDDL(s.DDL); ok {
			return fmt.Sprintf(`ALTER TABLE %s RENAME TO "%s"`, newName, old), ""
		}
		return "", fmt.Sprintf("MANUAL: reverse rename table %s (parse failed)", obj)

	// Column-level
	case plan.ChangeAddColumn:
		if m := reSchemaTableCol.FindStringSubmatch(obj); m != nil {
			return fmt.Sprintf(`ALTER TABLE "%s"."%s" DROP COLUMN IF EXISTS "%s" CASCADE`, m[1], m[2], m[3]), ""
		}
		return "", fmt.Sprintf("MANUAL: drop added column %s", obj)

	case plan.ChangeDropColumn:
		return "", fmt.Sprintf("MANUAL: re-add dropped column %s (original definition not stored)", obj)

	case plan.ChangeRenameColumn:
		if old, newCol, tbl, ok := parseRenameColumnDDL(s.DDL); ok {
			return fmt.Sprintf(`ALTER TABLE %s RENAME COLUMN "%s" TO "%s"`, tbl, newCol, old), ""
		}
		return "", fmt.Sprintf("MANUAL: reverse rename column %s (parse failed)", obj)

	case plan.ChangeAlterColumn:
		return "", fmt.Sprintf("MANUAL: reverse ALTER COLUMN %s — requires original type/default/nullability", obj)

	// Constraints
	case plan.ChangeAddConstraint:
		if m := reSchemaTable.FindStringSubmatch(obj); m != nil {
			// Extract constraint name from DDL: ADD CONSTRAINT "name" ...
			if cn := extractConstraintName(s.DDL); cn != "" {
				return fmt.Sprintf(`ALTER TABLE "%s"."%s" DROP CONSTRAINT IF EXISTS "%s"`, m[1], m[2], cn), ""
			}
		}
		return "", fmt.Sprintf("MANUAL: drop added constraint on %s", obj)

	case plan.ChangeDropConstraint:
		return "", fmt.Sprintf("MANUAL: re-add dropped constraint on %s (original DDL not stored)", obj)

	// Indexes
	case plan.ChangeCreateIndex:
		// Object is the index name (schema.idx_name or just name).
		if m := reSchemaTable.FindStringSubmatch(obj); m != nil {
			return fmt.Sprintf(`DROP INDEX IF EXISTS "%s"."%s"`, m[1], m[2]), ""
		}
		return fmt.Sprintf("DROP INDEX IF EXISTS %s", obj), ""

	case plan.ChangeDropIndex:
		return "", fmt.Sprintf("MANUAL: re-create dropped index %s (original DDL not stored)", obj)

	// Views
	case plan.ChangeCreateView, "CREATE_MATERIALIZED_VIEW":
		if m := reSchemaTable.FindStringSubmatch(obj); m != nil {
			keyword := "VIEW"
			if s.OpType == "CREATE_MATERIALIZED_VIEW" {
				keyword = "MATERIALIZED VIEW"
			}
			return fmt.Sprintf(`DROP %s IF EXISTS "%s"."%s" CASCADE`, keyword, m[1], m[2]), ""
		}
		return fmt.Sprintf("DROP VIEW IF EXISTS %s CASCADE", obj), ""

	case plan.ChangeDropView:
		return "", fmt.Sprintf("MANUAL: re-create dropped view %s (original DDL not stored)", obj)

	// Functions / Procedures
	case plan.ChangeCreateFunction:
		if m := reSchemaTable.FindStringSubmatch(obj); m != nil {
			return fmt.Sprintf(`DROP FUNCTION IF EXISTS "%s"."%s" CASCADE`, m[1], m[2]), ""
		}
		return fmt.Sprintf("DROP FUNCTION IF EXISTS %s CASCADE", obj), ""

	case plan.ChangeDropFunction:
		return "", fmt.Sprintf("MANUAL: re-create dropped function %s (original DDL not stored)", obj)

	// Triggers
	case plan.ChangeCreateTrigger:
		// Object = "schema.table.trigger_name"
		if m := reSchemaTableCol.FindStringSubmatch(obj); m != nil {
			return fmt.Sprintf(`DROP TRIGGER IF EXISTS "%s" ON "%s"."%s"`, m[3], m[1], m[2]), ""
		}
		return "", fmt.Sprintf("MANUAL: drop created trigger %s", obj)

	case plan.ChangeDropTrigger:
		return "", fmt.Sprintf("MANUAL: re-create dropped trigger %s (original DDL not stored)", obj)

	// Policies
	case plan.ChangeCreatePolicy:
		// Object = "schema.table.policy_name"
		if m := reSchemaTableCol.FindStringSubmatch(obj); m != nil {
			return fmt.Sprintf(`DROP POLICY IF EXISTS "%s" ON "%s"."%s"`, m[3], m[1], m[2]), ""
		}
		return "", fmt.Sprintf("MANUAL: drop created policy %s", obj)

	case plan.ChangeDropPolicy:
		return "", fmt.Sprintf("MANUAL: re-create dropped policy %s (original DDL not stored)", obj)

	// Sequences
	case plan.ChangeCreateSequence:
		if m := reSchemaTable.FindStringSubmatch(obj); m != nil {
			return fmt.Sprintf(`DROP SEQUENCE IF EXISTS "%s"."%s"`, m[1], m[2]), ""
		}
		return fmt.Sprintf("DROP SEQUENCE IF EXISTS %s", obj), ""

	case plan.ChangeDropSequence:
		return "", fmt.Sprintf("MANUAL: re-create dropped sequence %s (original DDL not stored)", obj)

	// Types / ENUMs / Domains
	case plan.ChangeCreateType:
		if m := reSchemaTable.FindStringSubmatch(obj); m != nil {
			return fmt.Sprintf(`DROP TYPE IF EXISTS "%s"."%s" CASCADE`, m[1], m[2]), ""
		}
		return fmt.Sprintf("DROP TYPE IF EXISTS %s CASCADE", obj), ""

	// Extensions
	case plan.ChangeCreateExtension:
		return fmt.Sprintf("DROP EXTENSION IF EXISTS %s CASCADE", obj), ""

	case plan.ChangeDropExtension:
		return "", fmt.Sprintf("MANUAL: re-install dropped extension %s", obj)

	// RLS toggles
	case plan.ChangeToggleRLS:
		// Reverse: if we enabled, disable; if we disabled, enable.
		if m := reSchemaTable.FindStringSubmatch(obj); m != nil {
			if strings.Contains(s.DDL, "ENABLE") {
				return fmt.Sprintf(`ALTER TABLE "%s"."%s" DISABLE ROW LEVEL SECURITY`, m[1], m[2]), ""
			}
			return fmt.Sprintf(`ALTER TABLE "%s"."%s" ENABLE ROW LEVEL SECURITY`, m[1], m[2]), ""
		}

	// SET NOT NULL / DROP NOT NULL
	case plan.ChangeAlterColumn + "_NOTNULL":
		if m := reSchemaTableCol.FindStringSubmatch(obj); m != nil {
			if strings.Contains(s.DDL, "SET NOT NULL") {
				return fmt.Sprintf(`ALTER TABLE "%s"."%s" ALTER COLUMN "%s" DROP NOT NULL`, m[1], m[2], m[3]), ""
			}
			return fmt.Sprintf(`ALTER TABLE "%s"."%s" ALTER COLUMN "%s" SET NOT NULL`, m[1], m[2], m[3]), ""
		}
	}

	return "", fmt.Sprintf("MANUAL: no known undo for op=%s obj=%s  DDL: %s", s.OpType, s.Object, s.DDL)
}

var reRenameTable = regexp.MustCompile(`(?i)ALTER\s+TABLE\s+(\S+)\s+RENAME\s+TO\s+"?([^";]+)"?`)
var reRenameColumn = regexp.MustCompile(`(?i)ALTER\s+TABLE\s+(\S+)\s+RENAME\s+COLUMN\s+"?([^"]+)"?\s+TO\s+"?([^";]+)"?`)
var reConstraintName = regexp.MustCompile(`(?i)ADD\s+CONSTRAINT\s+"?([^"]+)"?`)

func parseRenameTableDDL(ddl string) (oldName, newQualifiedName string, ok bool) {
	// DDL: ALTER TABLE schema.old_name RENAME TO new_name
	m := reRenameTable.FindStringSubmatch(ddl)
	if len(m) < 3 {
		return "", "", false
	}
	return strings.Trim(m[2], `"`), m[1], true
}

func parseRenameColumnDDL(ddl string) (oldCol, newCol, table string, ok bool) {
	m := reRenameColumn.FindStringSubmatch(ddl)
	if len(m) < 4 {
		return "", "", "", false
	}
	return strings.Trim(m[2], `"`), strings.Trim(m[3], `"`), m[1], true
}

func extractConstraintName(ddl string) string {
	m := reConstraintName.FindStringSubmatch(ddl)
	if len(m) < 2 {
		return ""
	}
	return strings.Trim(m[1], `"`)
}
