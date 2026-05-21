package dump

import (
	"testing"

	"github.com/nex-gen-tech/pg-flux/pkg/schema"
)

// helper: build SchemaState with just the given enum keys in the Enums map.
func stateWithEnumMap(keys ...string) *schema.SchemaState {
	st := &schema.SchemaState{
		Enums: make(map[string]*schema.EnumType, len(keys)),
	}
	for _, k := range keys {
		st.Enums[k] = &schema.EnumType{Schema: "public", Name: k}
	}
	return st
}

// helper: build SchemaState using the legacy EnumValues map (no Enums field).
func stateWithEnumValues(keys ...string) *schema.SchemaState {
	st := &schema.SchemaState{
		EnumValues: make(map[string][]string, len(keys)),
	}
	for _, k := range keys {
		st.EnumValues[k] = nil
	}
	return st
}

// ───── Structured Enums path ──────────────────────────────────────────────────

func TestVerify_enumDeclaredInSource_notReported(t *testing.T) {
	desired := stateWithEnumMap("public.my_status")
	live := stateWithEnumMap("public.my_status")

	r := Verify(desired, live)
	for _, k := range r.Enums {
		if k == "public.my_status" {
			t.Errorf("declared enum must not be reported as undeclared; got %v", r.Enums)
		}
	}
}

func TestVerify_liveEnumMissingFromSource_reported(t *testing.T) {
	desired := stateWithEnumMap("public.declared")
	live := stateWithEnumMap("public.declared", "public.undeclared")

	r := Verify(desired, live)
	found := false
	for _, k := range r.Enums {
		if k == "public.undeclared" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'public.undeclared' in report.Enums; got %v", r.Enums)
	}
}

func TestVerify_liveEnumPresentInSource_notReported(t *testing.T) {
	desired := stateWithEnumMap("public.x", "public.y")
	live := stateWithEnumMap("public.x", "public.y")

	r := Verify(desired, live)
	if len(r.Enums) != 0 {
		t.Errorf("expected no undeclared enums; got %v", r.Enums)
	}
}

// ───── Legacy EnumValues fallback path ───────────────────────────────────────

func TestVerify_legacyEnumValues_declaredInSource_notReported(t *testing.T) {
	desired := stateWithEnumValues("public.todo_status")
	live := stateWithEnumValues("public.todo_status")

	r := Verify(desired, live)
	if len(r.Enums) != 0 {
		t.Errorf("expected no undeclared enums; got %v", r.Enums)
	}
}

func TestVerify_legacyEnumValues_liveNotInSource_reported(t *testing.T) {
	desired := stateWithEnumValues("public.a")
	live := stateWithEnumValues("public.a", "public.b")

	r := Verify(desired, live)
	found := false
	for _, k := range r.Enums {
		if k == "public.b" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'public.b' in report.Enums; got %v", r.Enums)
	}
}

// ───── Mixed: live has Enums, desired has only EnumValues (incremental rollout) ──

func TestVerify_liveEnums_desiredEnumValues_acceptsMatch(t *testing.T) {
	// Live has Enums (new inspector), desired has only legacy EnumValues.
	// Verify should accept the match and not report undeclared.
	desired := stateWithEnumValues("public.compat_enum")
	live := stateWithEnumMap("public.compat_enum")

	r := Verify(desired, live)
	for _, k := range r.Enums {
		if k == "public.compat_enum" {
			t.Errorf("enum present in legacy desired.EnumValues must not be reported; got %v", r.Enums)
		}
	}
}
