package differ

// Tests for Issue #7 — drift false-positive when a partial index uses an IN()
// list in the WHERE predicate. pg_get_indexdef rewrites
//     WHERE col IN ('a', 'b')
// to
//     WHERE col = ANY (ARRAY['a'::sometype, 'b'::sometype])
// (and may strip ::type casts the user wrote). Both forms are syntactically
// equivalent and must compare equal — but adding or removing a list element
// (or swapping in a different literal) MUST still register as drift.
//
// We exercise the real comparison entrypoint (`indexDefsEqualWithRenames`)
// rather than the diagnostic `fpIndexSQL` helper, because the production
// diff code goes through that path and it's the only path that correctly
// preserves literal values (pgq.Fingerprint replaces them with $n
// placeholders, hiding all IN-list literal differences).

import (
	"testing"

	"github.com/nex-gen-tech/pg-flux/pkg/schema"
)

func mkIdx(create string) *schema.Index {
	return &schema.Index{
		Schema:      "public",
		Name:        "i",
		TableSchema: "public",
		Table:       "t",
		CreateSQL:   create,
	}
}

func indexEq(t *testing.T, src, live string) bool {
	t.Helper()
	return indexDefsEqualWithRenames(mkIdx(src), mkIdx(live), nil)
}

// --- POSITIVE TESTS: equivalent forms must compare equal ------------------

// Two-element predicate: IN vs = ANY (ARRAY[..]) with explicit ::text casts.
func TestIssue7_INvsAnyArray_textTwoElements(t *testing.T) {
	src := "CREATE INDEX i ON public.t (priority) WHERE priority IN ('high', 'urgent')"
	live := "CREATE INDEX i ON t USING btree (priority) WHERE priority = ANY (ARRAY['high'::text, 'urgent'::text])"
	if !indexEq(t, src, live) {
		t.Fatalf("Issue #7: IN-list vs = ANY(ARRAY[...]) should be equal")
	}
}

// Three-element predicate: the canonicalization must scale beyond two elements.
func TestIssue7_INvsAnyArray_threeElements(t *testing.T) {
	src := "CREATE INDEX i ON public.t (status) WHERE status IN ('draft', 'published', 'archived')"
	live := "CREATE INDEX i ON t USING btree (status) WHERE status = ANY (ARRAY['draft'::text, 'published'::text, 'archived'::text])"
	if !indexEq(t, src, live) {
		t.Fatalf("3-element IN-list vs ANY-array should be equal")
	}
}

// User-defined enum type cast in the array — appears in the real fastapi-todo
// example: `priority = ANY (ARRAY['high'::todo_priority, 'urgent'::todo_priority])`.
func TestIssue7_INvsAnyArray_userDefinedEnumCast(t *testing.T) {
	src := "CREATE INDEX i ON public.t (priority) WHERE priority IN ('high', 'urgent')"
	live := "CREATE INDEX i ON t USING btree (priority) WHERE priority = ANY (ARRAY['high'::todo_priority, 'urgent'::todo_priority])"
	if !indexEq(t, src, live) {
		t.Fatalf("enum-typed IN-list should match catalog form with ::enumtype casts")
	}
}

// Integer literal list (no quotes, no casts on source side).
func TestIssue7_INvsAnyArray_integerLiterals(t *testing.T) {
	src := "CREATE INDEX i ON public.t (kind) WHERE kind IN (1, 2, 3)"
	live := "CREATE INDEX i ON t USING btree (kind) WHERE kind = ANY (ARRAY[1, 2, 3])"
	if !indexEq(t, src, live) {
		t.Fatalf("integer IN-list vs ANY-array should be equal")
	}
}

// Schema-qualified type cast.
func TestIssue7_INvsAnyArray_schemaQualifiedTypeCast(t *testing.T) {
	src := "CREATE INDEX i ON public.t (priority) WHERE priority IN ('high', 'urgent')"
	live := "CREATE INDEX i ON t USING btree (priority) WHERE priority = ANY (ARRAY['high'::public.todo_priority, 'urgent'::public.todo_priority])"
	if !indexEq(t, src, live) {
		t.Fatalf("schema-qualified type cast on enum should normalize away")
	}
}

// --- NEGATIVE TESTS: semantically-different predicates must still diff ----
//
// These are the regression net. The canonicalization must not be so eager
// that it hides a real drift. The fix paired removal of `pgq.Fingerprint`
// from indexDefsEqualWithRenames — pgq.Fingerprint normalizes literal
// constants to $n placeholders, which would silently mask all of these.

// Extra element in live: catalog has 3 values, source has 2 — DRIFT.
func TestIssue7_NEGATIVE_extraElement(t *testing.T) {
	src := "CREATE INDEX i ON public.t (priority) WHERE priority IN ('high', 'urgent')"
	live := "CREATE INDEX i ON t USING btree (priority) WHERE priority = ANY (ARRAY['high'::text, 'urgent'::text, 'critical'::text])"
	if indexEq(t, src, live) {
		t.Fatalf("FALSE NEGATIVE: adding 'critical' to live MUST be detected as drift")
	}
}

// Missing element in live.
func TestIssue7_NEGATIVE_missingElement(t *testing.T) {
	src := "CREATE INDEX i ON public.t (priority) WHERE priority IN ('high', 'urgent', 'critical')"
	live := "CREATE INDEX i ON t USING btree (priority) WHERE priority = ANY (ARRAY['high'::text, 'urgent'::text])"
	if indexEq(t, src, live) {
		t.Fatalf("FALSE NEGATIVE: removing 'critical' from live MUST be detected as drift")
	}
}

// Different literal value (one element swapped).
func TestIssue7_NEGATIVE_differentLiteral(t *testing.T) {
	src := "CREATE INDEX i ON public.t (priority) WHERE priority IN ('high', 'urgent')"
	live := "CREATE INDEX i ON t USING btree (priority) WHERE priority = ANY (ARRAY['high'::text, 'critical'::text])"
	if indexEq(t, src, live) {
		t.Fatalf("FALSE NEGATIVE: swapping 'urgent' → 'critical' MUST be detected as drift")
	}
}

// Different predicate column.
func TestIssue7_NEGATIVE_differentPredicateColumn(t *testing.T) {
	src := "CREATE INDEX i ON public.t (priority) WHERE status IN ('high', 'urgent')"
	live := "CREATE INDEX i ON t USING btree (priority) WHERE priority = ANY (ARRAY['high'::text, 'urgent'::text])"
	if indexEq(t, src, live) {
		t.Fatalf("FALSE NEGATIVE: predicate column change (status vs priority) MUST be detected as drift")
	}
}

// Different indexed column (predicate match, but column index changed).
func TestIssue7_NEGATIVE_differentIndexedColumn(t *testing.T) {
	src := "CREATE INDEX i ON public.t (priority) WHERE priority IN ('high', 'urgent')"
	live := "CREATE INDEX i ON t USING btree (status) WHERE priority = ANY (ARRAY['high'::text, 'urgent'::text])"
	if indexEq(t, src, live) {
		t.Fatalf("FALSE NEGATIVE: indexed-column change (priority vs status) MUST be detected as drift")
	}
}

// Different IN-list values entirely.
func TestIssue7_NEGATIVE_completelyDifferentLiterals(t *testing.T) {
	src := "CREATE INDEX i ON public.t (priority) WHERE priority IN ('high', 'urgent')"
	live := "CREATE INDEX i ON t USING btree (priority) WHERE priority = ANY (ARRAY['low'::text, 'medium'::text])"
	if indexEq(t, src, live) {
		t.Fatalf("FALSE NEGATIVE: completely different literal set MUST be detected as drift")
	}
}

// --- DEFENSIVE TESTS ------------------------------------------------------
// Ensure unrelated predicate shapes (no IN-list) are untouched.

// A simple equality predicate must continue to work.
func TestIssue7_DEFENSIVE_simpleEqualityUnchanged(t *testing.T) {
	src := "CREATE INDEX i ON public.t (deleted_at) WHERE deleted_at IS NULL"
	live := "CREATE INDEX i ON t USING btree (deleted_at) WHERE deleted_at IS NULL"
	if !indexEq(t, src, live) {
		t.Fatalf("simple IS NULL predicate must still compare equal")
	}
}

// Bool equality literal preserved.
func TestIssue7_DEFENSIVE_boolEqualityUnchanged(t *testing.T) {
	src := "CREATE INDEX i ON public.t (user_id) WHERE done = false"
	live := "CREATE INDEX i ON t USING btree (user_id) WHERE done = false"
	if !indexEq(t, src, live) {
		t.Fatalf("simple boolean equality must still compare equal")
	}
}

// `= ANY (SELECT ...)` (a real subquery — NOT an array literal) must not
// be touched by the IN-list normalizer.
func TestIssue7_DEFENSIVE_anySubqueryUnchanged(t *testing.T) {
	got := normalizeAnyArrayForFingerprint("priority = ANY(SELECT id FROM other)")
	if got != "priority = ANY(SELECT id FROM other)" {
		t.Fatalf("subquery ANY form was molested: %q", got)
	}
}
