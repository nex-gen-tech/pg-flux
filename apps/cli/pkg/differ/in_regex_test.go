package differ

import (
	"strings"
	"testing"
)

// TestAnyArrayInList_bracketInLiteral confirms the IN-list normalizer survives
// closing brackets inside string literals like ARRAY['a]b'].
// The old regex `[^\]]+?` stopped at the first `]` even when it was inside a
// quoted value, leaving the fingerprint un-normalized and producing spurious drift.
func TestAnyArrayInList_bracketInLiteral(t *testing.T) {
	in := `x = ANY (ARRAY['a]b'::text, 'c[d'::text])`
	got := normalizeAnyArrayForFingerprint(in)
	// The expectation: after normalization the form is `x IN ('a]b', 'c[d')`.
	// We test for the IN form to confirm the rewriter recognised the construct.
	if !strings.Contains(got, "IN") || strings.Contains(got, "ARRAY") {
		t.Fatalf("expected IN-list normalization with brackets in literal; got %q", got)
	}
}

func TestAnyArrayInList_simple(t *testing.T) {
	in := `x = ANY (ARRAY['a'::text, 'b'::text])`
	got := normalizeAnyArrayForFingerprint(in)
	if !strings.Contains(got, "IN") {
		t.Fatalf("simple IN form lost: %q", got)
	}
}

func TestAnyArrayInList_noMatch(t *testing.T) {
	// A regular SELECT with ANY(SELECT ...) must not be molested.
	in := `WHERE x = ANY(SELECT id FROM other)`
	got := normalizeAnyArrayForFingerprint(in)
	if got != in {
		t.Fatalf("non-array ANY changed: %q -> %q", in, got)
	}
}
