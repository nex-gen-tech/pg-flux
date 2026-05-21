package differ

import (
	"strings"
	"testing"

	"github.com/nex-gen-tech/pg-flux/pkg/schema"
)

// helper: build a SchemaState with just the given enum types.
func stateWithEnums(enums ...*schema.EnumType) *schema.SchemaState {
	st := &schema.SchemaState{
		Enums: make(map[string]*schema.EnumType, len(enums)),
	}
	for _, e := range enums {
		if e != nil {
			st.Enums[e.Key()] = e
		}
	}
	return st
}

func mkEnum(sch, name string, vals ...string) *schema.EnumType {
	return &schema.EnumType{Schema: sch, Name: name, Values: vals}
}

// ───── Case 1: new enum ─────────────────────────────────────────────────────

func TestDiffEnums_newEnum_emitCreateType(t *testing.T) {
	desired := stateWithEnums(mkEnum("public", "status", "draft", "active", "archived"))
	live := &schema.SchemaState{}

	changes := diffEnums(desired, live)
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d: %+v", len(changes), changes)
	}
	ch := changes[0]
	if ch.kind != "CREATE_TYPE" {
		t.Errorf("expected CREATE_TYPE, got %q", ch.kind)
	}
	if !strings.Contains(ch.rawSQL, "CREATE TYPE public.status AS ENUM") {
		t.Errorf("rawSQL missing CREATE TYPE: %q", ch.rawSQL)
	}
	if !strings.Contains(ch.rawSQL, "'draft'") || !strings.Contains(ch.rawSQL, "'active'") {
		t.Errorf("rawSQL missing enum values: %q", ch.rawSQL)
	}
	if !strings.Contains(ch.rawSQL, "duplicate_object") {
		t.Errorf("rawSQL missing idempotency guard: %q", ch.rawSQL)
	}
}

func TestDiffEnums_newEnum_idempotentDOBlock(t *testing.T) {
	desired := stateWithEnums(mkEnum("myschema", "color", "red", "green", "blue"))
	live := &schema.SchemaState{}

	changes := diffEnums(desired, live)
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(changes))
	}
	sql := changes[0].rawSQL
	if !strings.HasPrefix(sql, "DO $pgflux$") {
		t.Errorf("expected DO block; got: %q", sql)
	}
	if !strings.Contains(sql, "EXCEPTION WHEN duplicate_object THEN NULL") {
		t.Errorf("expected duplicate_object guard; got: %q", sql)
	}
}

// ───── Case 2: added values ──────────────────────────────────────────────────

func TestDiffEnums_addedValue_emitADDVALUE(t *testing.T) {
	desired := stateWithEnums(mkEnum("public", "priority", "low", "normal", "high", "urgent"))
	live := stateWithEnums(mkEnum("public", "priority", "low", "normal", "high"))

	changes := diffEnums(desired, live)
	if len(changes) != 1 {
		t.Fatalf("expected 1 change, got %d: %+v", len(changes), changes)
	}
	ch := changes[0]
	if ch.rawSQL == "" {
		t.Fatal("expected ADD VALUE SQL, got empty rawSQL")
	}
	if !strings.Contains(ch.rawSQL, "ADD VALUE IF NOT EXISTS 'urgent'") {
		t.Errorf("unexpected ADD VALUE SQL: %q", ch.rawSQL)
	}
}

func TestDiffEnums_addedValueWithBEFOREHint(t *testing.T) {
	// 'pending' is new and 'active' already exists after it.
	desired := stateWithEnums(mkEnum("public", "state", "pending", "active"))
	live := stateWithEnums(mkEnum("public", "state", "active"))

	changes := diffEnums(desired, live)
	// May have 1 ADD VALUE with BEFORE hint.
	found := false
	for _, ch := range changes {
		if strings.Contains(ch.rawSQL, "ADD VALUE IF NOT EXISTS 'pending'") {
			found = true
			if !strings.Contains(ch.rawSQL, "BEFORE 'active'") {
				t.Errorf("expected BEFORE 'active'; got: %q", ch.rawSQL)
			}
		}
	}
	if !found {
		t.Errorf("expected ADD VALUE for 'pending'; changes: %+v", changes)
	}
}

// ───── Case 3: removed values → DATA_LOSS hazard, no DROP ───────────────────

func TestDiffEnums_removedValue_dataLossHazard(t *testing.T) {
	desired := stateWithEnums(mkEnum("public", "status", "draft", "active"))
	live := stateWithEnums(mkEnum("public", "status", "draft", "active", "deleted"))

	changes := diffEnums(desired, live)

	// Expect at least one change with a DATA_LOSS hazard, no DROP statement.
	var foundHazard bool
	for _, ch := range changes {
		for _, h := range ch.extraHazards {
			if strings.Contains(h.Message, "'deleted'") && strings.Contains(h.Message, "DROP VALUE") {
				foundHazard = true
			}
		}
		if strings.Contains(ch.rawSQL, "DROP") {
			t.Errorf("must not emit DROP for removed enum value, got: %q", ch.rawSQL)
		}
	}
	if !foundHazard {
		t.Errorf("expected DATA_LOSS hazard for removed enum value; changes: %+v", changes)
	}
}

func TestDiffEnums_removedValue_noDropStatement(t *testing.T) {
	desired := stateWithEnums(mkEnum("public", "foo", "a"))
	live := stateWithEnums(mkEnum("public", "foo", "a", "b", "c"))

	changes := diffEnums(desired, live)
	for _, ch := range changes {
		if strings.Contains(ch.rawSQL, "DROP VALUE") {
			t.Errorf("must not emit ALTER TYPE DROP VALUE (PG doesn't support it): %q", ch.rawSQL)
		}
	}
}

// ───── Case 4: enum only in live → DROP hazard ───────────────────────────────

func TestDiffEnums_enumOnlyInLive_dropHazard(t *testing.T) {
	desired := &schema.SchemaState{Enums: map[string]*schema.EnumType{}}
	live := stateWithEnums(mkEnum("public", "old_status", "a", "b"))

	changes := diffEnums(desired, live)
	if len(changes) != 1 {
		t.Fatalf("expected 1 change (DROP hazard), got %d: %+v", len(changes), changes)
	}
	ch := changes[0]
	if !strings.Contains(ch.rawSQL, "DROP TYPE") {
		t.Errorf("expected DROP TYPE in rawSQL: %q", ch.rawSQL)
	}
	var hasDataLoss bool
	for _, h := range ch.extraHazards {
		if h.Type == "DATA_LOSS" {
			hasDataLoss = true
		}
	}
	if !hasDataLoss {
		t.Errorf("expected DATA_LOSS hazard for dropped enum; got: %+v", ch.extraHazards)
	}
}

// ───── No change ─────────────────────────────────────────────────────────────

func TestDiffEnums_noChange_emitNothing(t *testing.T) {
	desired := stateWithEnums(mkEnum("public", "color", "red", "green", "blue"))
	live := stateWithEnums(mkEnum("public", "color", "red", "green", "blue"))

	changes := diffEnums(desired, live)
	// Filter out changes that have no DDL and no hazards (pure no-ops from renames detector).
	var meaningful []change
	for _, ch := range changes {
		if ch.rawSQL != "" || len(ch.extraHazards) > 0 {
			meaningful = append(meaningful, ch)
		}
	}
	if len(meaningful) != 0 {
		t.Errorf("expected no changes for identical enum; got: %+v", meaningful)
	}
}

// ───── buildCreateEnumSQL ─────────────────────────────────────────────────────

func TestBuildCreateEnumSQL_singleQuoteEscape(t *testing.T) {
	e := mkEnum("public", "test", "it's here", "normal")
	sql := buildCreateEnumSQL(e)
	// Single quote in label must be doubled.
	if !strings.Contains(sql, "it''s here") {
		t.Errorf("single quote not escaped in enum label; got: %q", sql)
	}
}

func TestBuildCreateEnumSQL_nilSafe(t *testing.T) {
	if s := buildCreateEnumSQL(nil); s != "" {
		t.Errorf("expected empty string for nil EnumType; got: %q", s)
	}
}
