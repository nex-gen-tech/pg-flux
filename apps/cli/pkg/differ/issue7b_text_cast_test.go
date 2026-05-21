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
	// B6 fix: user-defined enum type casts on quoted literals are now stripped
	// to eliminate false drift between source SQL (`WHERE status = 'active'`)
	// and pg_get_indexdef output (`WHERE (status = 'active'::product_status)`).
	// PG adds the resolved type cast automatically; it carries no user intent.
	// Source SQL and the catalog form must fingerprint identically after stripping.
	src := "CREATE INDEX i ON public.t (a) WHERE status = 'high'"
	live := "CREATE INDEX i ON public.t USING btree (a) WHERE (status = 'high'::todo_priority)"
	if !indexEq(t, src, live) {
		t.Fatalf("source and PG-canonical form with user-defined enum cast must compare equal (B6)")
	}
	// Confirm that the normalizer itself strips the cast.
	out := indexFingerprintNormalizers(
		"create index i on public.t using btree (a) where (status = 'high'::todo_priority)",
		"public",
	)
	if contains(out, "::todo_priority") {
		t.Fatalf("user-defined enum cast ::todo_priority should be stripped by normalizer (B6); got: %q", out)
	}
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

// --- B6: partial-index enum cast drift ----------------------------------

// TestB6_enumCastStripped verifies that a user-defined enum cast on a quoted
// literal is stripped by the normalizer so source WHERE and catalog WHERE match.
// Source: `WHERE status = 'active'`
// Catalog: `WHERE (status = 'active'::product_status)`
func TestB6_enumCastStripped(t *testing.T) {
	out := indexFingerprintNormalizers(
		"create index i on public.t (s) where (status = 'active'::product_status)",
		"public",
	)
	if contains(out, "::product_status") {
		t.Fatalf("'active'::product_status — enum cast should be stripped; got: %q", out)
	}
	if !contains(out, "'active'") {
		t.Fatalf("literal 'active' must survive stripping; got: %q", out)
	}
}

// TestB6_numericCastKept verifies that a numeric cast on a quoted literal is
// NOT stripped (it carries semantic meaning: explicit string-to-int coercion).
func TestB6_numericCastKept(t *testing.T) {
	out := indexFingerprintNormalizers(
		"create index i on public.t (a) where ('42'::int = a)",
		"public",
	)
	if !contains(out, "::int") {
		t.Fatalf("'42'::int — numeric cast must NOT be stripped; got: %q", out)
	}
}

// TestB6_textCastStripped verifies that the existing text-family cast stripping
// still works after the B6 change (regression guard).
func TestB6_textCastStripped(t *testing.T) {
	out := indexFingerprintNormalizers(
		"create index i on public.t (a) where (status = 'active'::text)",
		"public",
	)
	if contains(out, "::text") {
		t.Fatalf("'active'::text — text cast should be stripped; got: %q", out)
	}
}

// TestB6_domainCastStripped verifies that a user-defined domain cast is also
// stripped (domains are structurally the same as enums from the cast perspective).
func TestB6_domainCastStripped(t *testing.T) {
	out := indexFingerprintNormalizers(
		"create index i on public.t (a) where (code = 'hello'::my_custom_domain)",
		"public",
	)
	if contains(out, "::my_custom_domain") {
		t.Fatalf("'hello'::my_custom_domain — domain cast should be stripped; got: %q", out)
	}
}

// TestB6_differentLiteralsStillDrift verifies that two different literal values
// still register as drift after normalisation (the strip must not collapse
// semantically-distinct predicates).
func TestB6_differentLiteralsStillDrift(t *testing.T) {
	src := "CREATE INDEX i ON public.t (s) WHERE status = 'active'"
	live := "CREATE INDEX i ON public.t USING btree (s) WHERE (status = 'archived'::product_status)"
	if indexEq(t, src, live) {
		t.Fatalf("CRITICAL: 'active' vs 'archived' must still register as drift after enum cast strip")
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
