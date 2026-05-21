package differ

// Tests for Issue #8 — drift false-positive when comparing a CREATE OR REPLACE
// VIEW source against the catalog form returned by pg_get_viewdef.
//
// Catalog never emits "CREATE OR REPLACE VIEW", normalises whitespace, drops
// table-name qualifiers (PG16+), and may add ::type casts. The differ must
// treat these as syntactic noise — but a real body change (different column
// list, different WHERE clause, different table) MUST still drift.

import "testing"

// --- POSITIVE TESTS: equivalent forms must compare equal ------------------

// `CREATE OR REPLACE VIEW` vs `CREATE VIEW` for an identical body — this is
// the literal Issue #8 reproducer.
func TestIssue8_CreateOrReplaceVsCreate(t *testing.T) {
	src := `CREATE OR REPLACE VIEW public.active_todos AS
  SELECT t.id, t.user_id, t.title
  FROM public.todos t
  WHERE t.done = false;`
	live := `CREATE VIEW public.active_todos AS  SELECT id,
    user_id,
    title
   FROM todos t
  WHERE done = false;`
	if createStmtDefFingerprint(src) != createStmtDefFingerprint(live) {
		t.Fatalf("Issue #8: CREATE OR REPLACE vs CREATE should fingerprint equal:\n  src=%s\n  live=%s",
			createStmtDefFingerprint(src), createStmtDefFingerprint(live))
	}
}

// Whitespace and newline differences in the SELECT list — catalog
// pretty-prints the column list one-per-line; source SQL inlines it.
func TestIssue8_WhitespaceNormalization(t *testing.T) {
	src := `CREATE VIEW v AS SELECT id, name FROM users;`
	live := `CREATE VIEW public.v AS  SELECT id,
    name
   FROM users;`
	if createStmtDefFingerprint(src) != createStmtDefFingerprint(live) {
		t.Fatalf("Whitespace-only differences should fingerprint equal:\n  src=%s\n  live=%s",
			createStmtDefFingerprint(src), createStmtDefFingerprint(live))
	}
}

// Column qualifier stripping — PG16+ pg_get_viewdef drops the table prefix
// on column refs that are unambiguous; PG15 and earlier keep it.
func TestIssue8_ColumnQualifierStripping(t *testing.T) {
	src := `CREATE VIEW v AS SELECT t.id, t.name FROM t;`
	live := `CREATE VIEW public.v AS  SELECT id, name FROM t;`
	if createStmtDefFingerprint(src) != createStmtDefFingerprint(live) {
		t.Fatalf("t.col vs col (PG16+ qualifier stripping) should fingerprint equal:\n  src=%s\n  live=%s",
			createStmtDefFingerprint(src), createStmtDefFingerprint(live))
	}
}

// --- NEGATIVE TESTS: real body changes must still register as drift -------

// Different column list: source SELECTs (id, name); live SELECTs (id, name, email).
// This MUST still drift.
func TestIssue8_NEGATIVE_extraColumnInSelect(t *testing.T) {
	src := `CREATE OR REPLACE VIEW v AS SELECT id, name FROM users;`
	live := `CREATE VIEW v AS SELECT id, name, email FROM users;`
	if createStmtDefFingerprint(src) == createStmtDefFingerprint(live) {
		t.Fatalf("FALSE NEGATIVE: extra column in SELECT must drift\n  fp=%s",
			createStmtDefFingerprint(src))
	}
}

// Different WHERE clause: `done = false` vs `done = true` — a real semantic
// difference that MUST drift.
func TestIssue8_NEGATIVE_differentWhereClause(t *testing.T) {
	src := `CREATE OR REPLACE VIEW v AS SELECT id FROM t WHERE done = false;`
	live := `CREATE VIEW v AS SELECT id FROM t WHERE done = true;`
	if createStmtDefFingerprint(src) == createStmtDefFingerprint(live) {
		t.Fatalf("FALSE NEGATIVE: done=false vs done=true must drift")
	}
}

// Different source table: source FROMs `todos`; live FROMs `archived_todos`.
func TestIssue8_NEGATIVE_differentSourceTable(t *testing.T) {
	src := `CREATE OR REPLACE VIEW v AS SELECT id FROM todos WHERE done = false;`
	live := `CREATE VIEW v AS SELECT id FROM archived_todos WHERE done = false;`
	if createStmtDefFingerprint(src) == createStmtDefFingerprint(live) {
		t.Fatalf("FALSE NEGATIVE: different source table must drift")
	}
}

// Missing WHERE clause: source has a predicate; live is unfiltered.
func TestIssue8_NEGATIVE_missingWhereClause(t *testing.T) {
	src := `CREATE OR REPLACE VIEW v AS SELECT id FROM t WHERE done = false;`
	live := `CREATE VIEW v AS SELECT id FROM t;`
	if createStmtDefFingerprint(src) == createStmtDefFingerprint(live) {
		t.Fatalf("FALSE NEGATIVE: dropping the WHERE clause must drift")
	}
}

// Different literal in the WHERE predicate (the kind of change Issue #8's
// fingerprint must catch even after CREATE-vs-CREATE-OR-REPLACE normalization).
func TestIssue8_NEGATIVE_differentLiteralInWhere(t *testing.T) {
	src := `CREATE OR REPLACE VIEW v AS SELECT id FROM posts WHERE status = 'draft';`
	live := `CREATE VIEW v AS SELECT id FROM posts WHERE status = 'published';`
	if createStmtDefFingerprint(src) == createStmtDefFingerprint(live) {
		t.Fatalf("FALSE NEGATIVE: 'draft' vs 'published' literal change must drift")
	}
}
