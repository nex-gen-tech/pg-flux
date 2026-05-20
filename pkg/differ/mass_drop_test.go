package differ

import (
	"errors"
	"strings"
	"testing"

	"github.com/nexg/pg-flux/pkg/plan"
	"github.com/nexg/pg-flux/pkg/schema"
)

// liveWith builds a SchemaState with N tables for guard tests.
func liveWith(n int) *schema.SchemaState {
	s := &schema.SchemaState{Tables: map[string]*schema.Table{}}
	for i := 0; i < n; i++ {
		nm := string(rune('a' + i))
		s.Tables["public."+nm] = &schema.Table{Schema: "public", Name: nm}
	}
	return s
}

func dropChange(sch, name string) change {
	return change{kind: plan.ChangeDropTable, sch: sch, tbl: name}
}

func TestMassDrop_blocksFullWipeOfNonEmptyDB(t *testing.T) {
	live := liveWith(3)
	changes := []change{
		dropChange("public", "a"),
		dropChange("public", "b"),
		dropChange("public", "c"),
	}
	err := checkMassDrop(changes, live, false, 25)
	var mde *MassDropError
	if !errors.As(err, &mde) {
		t.Fatalf("expected *MassDropError, got %T (%v)", err, err)
	}
	if mde.DropCount != 3 || mde.LiveCount != 3 {
		t.Fatalf("counts off: %+v", mde)
	}
	if !strings.Contains(err.Error(), "--allow-mass-drop") {
		t.Fatalf("error should mention the override flag: %s", err)
	}
}

func TestMassDrop_blocksAboveThreshold(t *testing.T) {
	live := liveWith(10)
	// Drop 4 of 10 = 40% > 25% threshold.
	var changes []change
	for _, n := range []string{"a", "b", "c", "d"} {
		changes = append(changes, dropChange("public", n))
	}
	err := checkMassDrop(changes, live, false, 25)
	if err == nil {
		t.Fatalf("expected MassDropError above 25%% threshold")
	}
}

func TestMassDrop_allowsBelowThreshold(t *testing.T) {
	live := liveWith(10)
	// Drop 2 of 10 = 20% < 25% threshold.
	changes := []change{
		dropChange("public", "a"),
		dropChange("public", "b"),
	}
	if err := checkMassDrop(changes, live, false, 25); err != nil {
		t.Fatalf("expected no error below threshold, got %v", err)
	}
}

func TestMassDrop_overrideAllowsAnyDrop(t *testing.T) {
	live := liveWith(3)
	changes := []change{
		dropChange("public", "a"),
		dropChange("public", "b"),
		dropChange("public", "c"),
	}
	if err := checkMassDrop(changes, live, true, 25); err != nil {
		t.Fatalf("--allow-mass-drop should bypass guard, got %v", err)
	}
}

func TestMassDrop_zeroLiveIsNoop(t *testing.T) {
	live := &schema.SchemaState{Tables: map[string]*schema.Table{}}
	if err := checkMassDrop(nil, live, false, 25); err != nil {
		t.Fatalf("empty live = noop, got %v", err)
	}
}

func TestMassDrop_zeroThresholdUsesDefault(t *testing.T) {
	live := liveWith(4)
	// 2 of 4 = 50%; default threshold (25) should trip.
	changes := []change{
		dropChange("public", "a"),
		dropChange("public", "b"),
	}
	if err := checkMassDrop(changes, live, false, 0); err == nil {
		t.Fatalf("threshold=0 must fall back to 25%% default and trip")
	}
}

func TestMassDrop_DiffReturnsErrorOnEmptyDesired(t *testing.T) {
	live := liveWith(3)
	desired := &schema.SchemaState{Tables: map[string]*schema.Table{}}
	_, err := Diff(desired, live, Options{})
	var mde *MassDropError
	if !errors.As(err, &mde) {
		t.Fatalf("Diff should refuse empty desired vs non-empty live, got %T (%v)", err, err)
	}
}

func TestMassDrop_DiffSucceedsWithOverride(t *testing.T) {
	live := liveWith(3)
	desired := &schema.SchemaState{Tables: map[string]*schema.Table{}}
	res, err := Diff(desired, live, Options{AllowMassDrop: true})
	if err != nil {
		t.Fatalf("Diff with --allow-mass-drop should succeed, got %v", err)
	}
	if res == nil || res.Plan == nil {
		t.Fatalf("expected a plan, got nil")
	}
}
