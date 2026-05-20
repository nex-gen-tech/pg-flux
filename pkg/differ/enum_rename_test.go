package differ

import (
	"reflect"
	"testing"
)

func setOf(ss ...string) map[string]struct{} {
	m := make(map[string]struct{}, len(ss))
	for _, s := range ss {
		m[s] = struct{}{}
	}
	return m
}

func TestDetectEnumRenames_singleRenameSamePosition(t *testing.T) {
	live := []string{"draft", "review", "published"}
	desired := []string{"draft", "in_review", "published"}
	got := detectEnumRenames(desired, live, setOf(live...), setOf(desired...))
	want := []enumRename{{From: "review", To: "in_review"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v want %+v", got, want)
	}
}

func TestDetectEnumRenames_multipleRenames(t *testing.T) {
	live := []string{"a", "b", "c"}
	desired := []string{"alpha", "beta", "gamma"}
	got := detectEnumRenames(desired, live, setOf(live...), setOf(desired...))
	want := []enumRename{
		{From: "a", To: "alpha"},
		{From: "b", To: "beta"},
		{From: "c", To: "gamma"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v want %+v", got, want)
	}
}

func TestDetectEnumRenames_reorderNotRename(t *testing.T) {
	// Swapping positions of existing values should NOT be detected as rename
	// (PG doesn't support reorder anyway, but we shouldn't emit RENAME VALUE
	// statements that would corrupt the enum).
	live := []string{"a", "b"}
	desired := []string{"b", "a"}
	got := detectEnumRenames(desired, live, setOf(live...), setOf(desired...))
	if len(got) != 0 {
		t.Fatalf("reorder must not infer rename; got %+v", got)
	}
}

func TestDetectEnumRenames_lengthMismatch(t *testing.T) {
	// Pure add — different lengths — defer to ADD VALUE path.
	live := []string{"a", "b"}
	desired := []string{"a", "b", "c"}
	got := detectEnumRenames(desired, live, setOf(live...), setOf(desired...))
	if len(got) != 0 {
		t.Fatalf("length mismatch must skip rename detection; got %+v", got)
	}
}

func TestDetectEnumRenames_addAndRenameNotInferred(t *testing.T) {
	// Mixed: a real rename + an insertion. Length differs → skip rename
	// inference entirely; the user will get an ADD VALUE for the new label
	// and a DROP-not-supported advisory for the old. Safer than guessing.
	live := []string{"draft", "review"}
	desired := []string{"draft", "in_review", "published"}
	got := detectEnumRenames(desired, live, setOf(live...), setOf(desired...))
	if len(got) != 0 {
		t.Fatalf("ambiguous shape must not infer; got %+v", got)
	}
}

func TestDetectEnumRenames_identicalListsNoop(t *testing.T) {
	live := []string{"a", "b", "c"}
	desired := []string{"a", "b", "c"}
	got := detectEnumRenames(desired, live, setOf(live...), setOf(desired...))
	if len(got) != 0 {
		t.Fatalf("identical lists must produce zero renames; got %+v", got)
	}
}
