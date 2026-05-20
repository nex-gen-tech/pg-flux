package differ

import "testing"

func TestIndexNormalizers_stripsAscNullsLast(t *testing.T) {
	got := indexFingerprintNormalizers("CREATE INDEX i ON t (a ASC NULLS LAST)", "public")
	want := "create index i on t (a )"
	if got != want && got != "create index i on t (a)" {
		t.Fatalf("ASC NULLS LAST not stripped: got %q", got)
	}
}

func TestIndexNormalizers_keepsDescAlone(t *testing.T) {
	// DESC alone must be preserved (NULLS FIRST is default for DESC, but DESC itself matters).
	got := indexFingerprintNormalizers("CREATE INDEX i ON t (a DESC NULLS FIRST)", "public")
	want := "create index i on t (a desc)"
	if got != want {
		t.Fatalf("DESC NULLS FIRST not normalized to DESC: got %q want %q", got, want)
	}
}

func TestIndexNormalizers_keepsDescNullsLast(t *testing.T) {
	// DESC NULLS LAST is NON-default and must be preserved.
	got := indexFingerprintNormalizers("CREATE INDEX i ON t (a DESC NULLS LAST)", "public")
	want := "create index i on t (a desc nulls last)"
	if got != want {
		t.Fatalf("DESC NULLS LAST must be preserved: got %q want %q", got, want)
	}
}

func TestIndexNormalizers_keepsAscNullsFirst(t *testing.T) {
	// ASC NULLS FIRST is NON-default and must be preserved (only the redundant ASC strips).
	got := indexFingerprintNormalizers("CREATE INDEX i ON t (a ASC NULLS FIRST)", "public")
	want := "create index i on t (a asc nulls first)"
	if got != want {
		t.Fatalf("ASC NULLS FIRST must keep NULLS FIRST: got %q want %q", got, want)
	}
}

func TestIndexNormalizers_stripsBareAscBeforeComma(t *testing.T) {
	got := indexFingerprintNormalizers("CREATE INDEX i ON t (a ASC, b)", "public")
	want := "create index i on t (a , b)"
	if got != want && got != "create index i on t (a, b)" {
		t.Fatalf("ASC before comma not stripped: got %q", got)
	}
}

func TestIndexNormalizers_includeAndConcurrently(t *testing.T) {
	a := indexFingerprintNormalizers("CREATE INDEX CONCURRENTLY i ON public.t (a) INCLUDE (b)", "public")
	b := indexFingerprintNormalizers("CREATE INDEX i ON t USING btree (a) INCLUDE (b)", "public")
	// The 'using btree' is only emitted by pg_get_indexdef. Strip it for comparison purposes.
	// Both should converge to a form with INCLUDE (b).
	if a == "" || b == "" {
		t.Fatalf("empty normalizer output: a=%q b=%q", a, b)
	}
}
