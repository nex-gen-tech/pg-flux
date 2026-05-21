package migrate

// Tests for COLUMN_REORDER advisory suppression across successive migrations.
// These tests exercise the helper functions directly (no live DB required) and
// the full Generate-level suppression by simulating the migrations directory.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nex-gen-tech/pg-flux/pkg/hazard"
	"github.com/nex-gen-tech/pg-flux/pkg/plan"
	"github.com/stretchr/testify/require"
)

// columnReorderAdvisoryMsg builds the advisory message in the same format that
// columnOrderNotice (in the differ) produces, for use in test expectations.
func columnReorderAdvisoryMsg(schema, table string, cols ...string) string {
	return fmt.Sprintf(
		"Column order in %s.%s differs from desired schema; reordering requires table recreation. Desired order (surviving cols): %s",
		schema, table, strings.Join(cols, ", "),
	)
}

// makeReorderAdvisoryStmt creates a plan.Statement representing a COLUMN_REORDER
// advisory (DDL="" , advisory hazard) matching what the differ emits.
func makeReorderAdvisoryStmt(id int, schema, table string, cols ...string) plan.Statement {
	return plan.Statement{
		ID:     id,
		OpType: "ADVISORY",
		Object: schema + "." + table,
		DDL:    "",
		Hazards: []hazard.Detected{{
			Type:     hazard.ColumnReorder,
			Severity: hazard.SeverityAdvisory,
			Message:  columnReorderAdvisoryMsg(schema, table, cols...),
		}},
	}
}

// makeAddColumnStmt creates a minimal ADD COLUMN statement (real DDL).
func makeAddColumnStmt(id int, schema, table, col string) plan.Statement {
	return plan.Statement{
		ID:     id,
		OpType: "ADD_COLUMN",
		Object: fmt.Sprintf("%s.%s.%s", schema, table, col),
		DDL:    fmt.Sprintf("ALTER TABLE %s.%s ADD COLUMN %s text", schema, table, col),
	}
}

// writeMigrationFile writes the given SQL content into dir with a predictable
// name (using the provided prefix so tests can control sort order).
func writeMigrationFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

// TestColumnReorderSuppression_M1M2M3 is the canonical test for the noise-fix:
//
//  1. M1: a plan with a COLUMN_REORDER advisory + real DDL is rendered; the
//     advisory comment must appear in M1.
//  2. M2: the same reorder advisory + different real DDL; after suppression the
//     advisory must NOT appear (the M1 file is the "previous migration").
//  3. M3: a NEW reorder advisory (different desired order) + real DDL; must
//     appear because the fingerprint changed.
func TestColumnReorderSuppression_M1M2M3(t *testing.T) {
	dir := t.TempDir()

	reorderMsg := columnReorderAdvisoryMsg("public", "orders", "id", "status", "created_at")

	// ---- M1: column reorder first detected ----
	m1Plan := &plan.ExecutionPlan{
		Statements: []plan.Statement{
			makeReorderAdvisoryStmt(1, "public", "orders", "id", "status", "created_at"),
			makeAddColumnStmt(2, "public", "orders", "note"),
		},
	}
	m1SQL := buildMigrationSQL(m1Plan, "")
	require.Contains(t, m1SQL, "[ADVISORY COLUMN_REORDER]", "M1 must contain the advisory")
	require.Contains(t, m1SQL, reorderMsg, "M1 must contain the exact advisory message")

	// Write M1 to the migrations directory (sorted first by timestamp prefix).
	writeMigrationFile(t, dir, "20260101_000001_000.sql", m1SQL)

	// ---- M2: same reorder mismatch, different DDL ----
	m2Plan := &plan.ExecutionPlan{
		Statements: []plan.Statement{
			makeReorderAdvisoryStmt(1, "public", "orders", "id", "status", "created_at"),
			makeAddColumnStmt(2, "public", "orders", "description"),
		},
	}

	// Simulate what Generate does: check previous migration, suppress.
	prevSeen := prevColumnReorderAdvisories(dir)
	require.NotEmpty(t, prevSeen, "should detect advisory from M1")
	suppressColumnReorderAdvisories(m2Plan, prevSeen)

	m2SQL := buildMigrationSQL(m2Plan, "")
	require.NotContains(t, m2SQL, "[ADVISORY COLUMN_REORDER]",
		"M2 must NOT contain the advisory for the same reorder")
	require.Contains(t, m2SQL, "ADD COLUMN", "M2 real DDL must still be present")

	// Write M2.
	writeMigrationFile(t, dir, "20260101_000002_000.sql", m2SQL)

	// ---- M3: new reorder (column added in different position = new divergence) ----
	newReorderMsg := columnReorderAdvisoryMsg("public", "orders", "id", "extra_col", "status", "created_at")
	m3Plan := &plan.ExecutionPlan{
		Statements: []plan.Statement{
			// Different desired order: extra_col inserted between id and status.
			{
				ID:     1,
				OpType: "ADVISORY",
				Object: "public.orders",
				DDL:    "",
				Hazards: []hazard.Detected{{
					Type:     hazard.ColumnReorder,
					Severity: hazard.SeverityAdvisory,
					Message:  newReorderMsg,
				}},
			},
			makeAddColumnStmt(2, "public", "orders", "extra_col"),
		},
	}

	// After writing M2, the previous migration is M2 (no COLUMN_REORDER advisory).
	prevSeen3 := prevColumnReorderAdvisories(dir)
	require.Empty(t, prevSeen3, "M2 has no advisory, so prevSeen3 should be empty")

	// No suppression needed (prevSeen3 is nil/empty).
	if len(prevSeen3) > 0 {
		suppressColumnReorderAdvisories(m3Plan, prevSeen3)
	}

	m3SQL := buildMigrationSQL(m3Plan, "")
	require.Contains(t, m3SQL, "[ADVISORY COLUMN_REORDER]",
		"M3 must contain the advisory because the reorder fingerprint changed")
	require.Contains(t, m3SQL, newReorderMsg, "M3 advisory must reference the new column order")
}

// TestPrevColumnReorderAdvisories_emptyDir returns nil on an empty directory.
func TestPrevColumnReorderAdvisories_emptyDir(t *testing.T) {
	dir := t.TempDir()
	got := prevColumnReorderAdvisories(dir)
	require.Nil(t, got)
}

// TestPrevColumnReorderAdvisories_noAdvisoryInLastFile returns nil when the last
// migration exists but has no COLUMN_REORDER comment.
func TestPrevColumnReorderAdvisories_noAdvisoryInLastFile(t *testing.T) {
	dir := t.TempDir()
	writeMigrationFile(t, dir, "20260101_000001_000.sql",
		"-- pg-flux generated migration\nBEGIN;\nALTER TABLE t ADD COLUMN c text;\nCOMMIT;\n")
	got := prevColumnReorderAdvisories(dir)
	require.Nil(t, got)
}

// TestPrevColumnReorderAdvisories_detectsAdvisory verifies the scanner picks up
// the advisory line from the last file.
func TestPrevColumnReorderAdvisories_detectsAdvisory(t *testing.T) {
	dir := t.TempDir()
	msg := columnReorderAdvisoryMsg("public", "foo", "a", "b", "c")
	content := fmt.Sprintf(
		"-- pg-flux generated migration\n%s%s\n\nBEGIN;\nALTER TABLE public.foo ADD COLUMN x text;\nCOMMIT;\n",
		columnReorderPrefix, msg,
	)
	writeMigrationFile(t, dir, "20260101_000001_000.sql", content)
	got := prevColumnReorderAdvisories(dir)
	require.NotNil(t, got)
	require.True(t, got[msg], "advisory message must be in the returned set")
}

// TestPrevColumnReorderAdvisories_usesLastFile verifies only the most recent
// (lexically last) migration is examined, not an older one.
func TestPrevColumnReorderAdvisories_usesLastFile(t *testing.T) {
	dir := t.TempDir()
	oldMsg := columnReorderAdvisoryMsg("public", "old", "x", "y")
	oldContent := fmt.Sprintf("-- pg-flux generated migration\n%s%s\n\nBEGIN;\nALTER TABLE public.old ADD COLUMN z text;\nCOMMIT;\n",
		columnReorderPrefix, oldMsg)
	writeMigrationFile(t, dir, "20260101_000001_000.sql", oldContent)

	// Newer file has no advisory.
	writeMigrationFile(t, dir, "20260101_000002_000.sql",
		"-- pg-flux generated migration\nBEGIN;\nALTER TABLE public.new ADD COLUMN z text;\nCOMMIT;\n")

	got := prevColumnReorderAdvisories(dir)
	require.Nil(t, got, "only the last file matters; older advisory must not bleed through")
}

// TestSuppressColumnReorderAdvisories_keepsOtherHazards verifies that non-COLUMN_REORDER
// hazards on the same advisory statement are preserved after suppression.
func TestSuppressColumnReorderAdvisories_keepsOtherHazards(t *testing.T) {
	msg := columnReorderAdvisoryMsg("public", "t", "a", "b")
	stmt := plan.Statement{
		ID:     1,
		OpType: "ADVISORY",
		Object: "public.t",
		DDL:    "",
		Hazards: []hazard.Detected{
			{Type: hazard.ColumnReorder, Severity: hazard.SeverityAdvisory, Message: msg},
			// A hypothetical second advisory hazard that should survive.
			{Type: hazard.TableLock, Severity: hazard.SeverityAdvisory, Message: "some other advisory"},
		},
	}
	p := &plan.ExecutionPlan{Statements: []plan.Statement{stmt}}
	prevSeen := map[string]bool{msg: true}
	suppressColumnReorderAdvisories(p, prevSeen)

	// Statement must still be present (has another hazard).
	require.Len(t, p.Statements, 1)
	require.Len(t, p.Statements[0].Hazards, 1)
	require.Equal(t, hazard.TableLock, p.Statements[0].Hazards[0].Type)
}

// TestSuppressColumnReorderAdvisories_removesStatementWhenAllHazardsSuppressed
// verifies that an advisory statement is dropped entirely when all its hazards
// are suppressed.
func TestSuppressColumnReorderAdvisories_removesStatementWhenAllHazardsSuppressed(t *testing.T) {
	msg := columnReorderAdvisoryMsg("public", "t", "a", "b")
	reorderStmt := makeReorderAdvisoryStmt(1, "public", "t", "a", "b")
	addColStmt := makeAddColumnStmt(2, "public", "t", "c")
	p := &plan.ExecutionPlan{Statements: []plan.Statement{reorderStmt, addColStmt}}

	prevSeen := map[string]bool{msg: true}
	suppressColumnReorderAdvisories(p, prevSeen)

	// Advisory statement gone, real DDL remains.
	require.Len(t, p.Statements, 1)
	require.Equal(t, "ADD_COLUMN", p.Statements[0].OpType)
}
