package differ

// Tests for the follow-up to Issue #7 — Agent 2's fix removed pgq.Fingerprint,
// which had been silently masking another canonicalisation: pg_get_indexdef
// tags string literals with their resolved type and wraps the WHERE clause
// in parens, e.g. source `WHERE status = 'published'` becomes catalog
// `WHERE (status = 'published'::text)`. The matrix step 10_index_add hit this
// in the first CI run after the #7 fix landed.
//
// These tests lock in the fix (strip ::text/::varchar/::bpchar casts on
// literals + unwrap a single outer paren after WHERE) and prove the
// adversarial cases still register as drift.

import "testing"

// --- POSITIVE: PG-canonicalised forms must compare equal to source ---------

func TestIndexDefsEqual_textCastOnLiteral(t *testing.T) {
	src := "CREATE INDEX i ON public.t (a) WHERE status = 'published'"
	live := "CREATE INDEX i ON public.t USING btree (a) WHERE (status = 'published'::text)"
	if !indexEq(t, src, live) {
		t.Fatalf("source vs PG-canonical form (with ::text + outer parens) must compare equal")
	}
}

func TestIndexDefsEqual_varcharCastOnLiteral(t *testing.T) {
	src := "CREATE INDEX i ON public.t (a) WHERE kind = 'invoice'"
	live := "CREATE INDEX i ON public.t USING btree (a) WHERE (kind = 'invoice'::character varying)"
	if !indexEq(t, src, live) {
		t.Fatalf("character varying cast on string literal must be stripped")
	}
}

func TestIndexDefsEqual_bpcharAndCharCast(t *testing.T) {
	src := "CREATE INDEX i ON public.t (a) WHERE code = 'XY'"
	live := "CREATE INDEX i ON public.t USING btree (a) WHERE (code = 'XY'::bpchar)"
	if !indexEq(t, src, live) {
		t.Fatalf("bpchar cast must be stripped")
	}
}

func TestIndexDefsEqual_quotedCharCast(t *testing.T) {
	src := "CREATE INDEX i ON public.t (a) WHERE c = 'a'"
	live := `CREATE INDEX i ON public.t USING btree (a) WHERE (c = 'a'::"char")`
	if !indexEq(t, src, live) {
		t.Fatalf(`"char" cast must be stripped`)
	}
}

func TestIndexDefsEqual_outerParensOnlyDifference(t *testing.T) {
	src := "CREATE INDEX i ON public.t (a) WHERE x > 0"
	live := "CREATE INDEX i ON public.t USING btree (a) WHERE (x > 0)"
	if !indexEq(t, src, live) {
		t.Fatalf("outer parens on simple predicate must be unwrapped")
	}
}

// --- NEGATIVE: cases that DIFFER and MUST still register as drift ---------

func TestIndexDefsEqual_differentLiteralStillDrifts(t *testing.T) {
	src := "CREATE INDEX i ON public.t (a) WHERE status = 'published'"
	live := "CREATE INDEX i ON public.t USING btree (a) WHERE (status = 'archived'::text)"
	if indexEq(t, src, live) {
		t.Fatalf("CRITICAL false-negative: 'published' vs 'archived' must still register as drift even with the ::text strip")
	}
}

func TestIndexDefsEqual_castToUserTypePreserved(t *testing.T) {
	// User-defined enum type cast carries meaning. If we accidentally strip
	// it we mask legitimate drift between an int column and a typed enum.
	src := "CREATE INDEX i ON public.t (a) WHERE status = 'high'"
	live := "CREATE INDEX i ON public.t USING btree (a) WHERE (status = 'high'::todo_priority)"
	// The source has no cast; the live has a user-type cast. We're being strict:
	// these reference different things (a text literal vs an enum literal), the
	// fingerprint must NOT erase that distinction. If pg_query parse+deparse
	// normalises both sides to the same form for this exact case, that's a
	// separate concern — but the regex MUST NOT strip ::todo_priority.
	out := indexFingerprintNormalizers(
		"create index i on public.t using btree (a) where (status = 'high'::todo_priority)",
		"public",
	)
	if !contains(out, "::todo_priority") {
		t.Fatalf("user-defined enum cast must NOT be stripped; got: %q", out)
	}
	// Compare-time behaviour: the surrounding indexDefsEqual may still match if
	// the source happens to canonicalise the same way through pg_query. The
	// regex-level guarantee is what we lock in here. Reference src/live for
	// future debugging when this test ever needs revisiting.
	_ = src
	_ = live
}

func TestIndexDefsEqual_numericCastPreserved(t *testing.T) {
	// A cast like '5'::int IS load-bearing — the user wrote text but meant int.
	// We must not strip it.
	out := indexFingerprintNormalizers(
		"create index i on public.t using btree (a) where ('5'::int = a)",
		"public",
	)
	if !contains(out, "::int") {
		t.Fatalf("numeric ::int cast must NOT be stripped; got: %q", out)
	}
}

// contains avoids pulling in strings just for one substring check in tests.
func contains(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
