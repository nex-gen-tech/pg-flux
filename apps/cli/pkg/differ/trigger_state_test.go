package differ

import (
	"strings"
	"testing"

	"github.com/nex-gen-tech/pg-flux/pkg/plan"
	"github.com/nex-gen-tech/pg-flux/pkg/schema"
)

func mkTrigState(enabled string) *schema.Trigger {
	return &schema.Trigger{
		Schema: "public", Table: "orders", Name: "audit_orders",
		DefSQL:  "CREATE TRIGGER audit_orders BEFORE UPDATE ON public.orders FOR EACH ROW EXECUTE FUNCTION public.f()",
		Enabled: enabled,
	}
}

func stateChange(t *testing.T, des, live string) []change {
	t.Helper()
	d := &schema.SchemaState{Triggers: map[string]*schema.Trigger{
		"public.orders/audit_orders": mkTrigState(des),
	}}
	l := &schema.SchemaState{Triggers: map[string]*schema.Trigger{
		"public.orders/audit_orders": mkTrigState(live),
	}}
	return diffTriggers(d, l)
}

func TestTriggerEnable_disabledToEnabled(t *testing.T) {
	chs := stateChange(t, "O", "D")
	if len(chs) != 1 {
		t.Fatalf("expected 1 change, got %d (%+v)", len(chs), chs)
	}
	c := chs[0]
	if c.kind != plan.ChangeRawSQL {
		t.Fatalf("expected RawSQL, got %s", c.kind)
	}
	if !strings.Contains(c.rawSQL, "ENABLE TRIGGER") || strings.Contains(c.rawSQL, "REPLICA") || strings.Contains(c.rawSQL, "ALWAYS") {
		t.Fatalf("expected plain ENABLE TRIGGER, got %q", c.rawSQL)
	}
}

func TestTriggerEnable_enabledToDisabled(t *testing.T) {
	chs := stateChange(t, "D", "O")
	if len(chs) != 1 {
		t.Fatalf("expected 1, got %d", len(chs))
	}
	if !strings.Contains(chs[0].rawSQL, "DISABLE TRIGGER") {
		t.Fatalf("got %q", chs[0].rawSQL)
	}
}

func TestTriggerEnable_toReplica(t *testing.T) {
	chs := stateChange(t, "R", "O")
	if !strings.Contains(chs[0].rawSQL, "ENABLE REPLICA TRIGGER") {
		t.Fatalf("got %q", chs[0].rawSQL)
	}
}

func TestTriggerEnable_toAlways(t *testing.T) {
	chs := stateChange(t, "A", "O")
	if !strings.Contains(chs[0].rawSQL, "ENABLE ALWAYS TRIGGER") {
		t.Fatalf("got %q", chs[0].rawSQL)
	}
}

func TestTriggerEnable_emptyEqualsOrigin(t *testing.T) {
	// Empty string and "O" are equivalent; no change.
	chs := stateChange(t, "", "O")
	if len(chs) != 0 {
		t.Fatalf("empty desired == 'O' live should be no-op, got %d changes", len(chs))
	}
}

func TestTriggerEnable_createWithNonDefaultEmitsAlter(t *testing.T) {
	// New trigger with Enabled='D' should emit CREATE + DISABLE.
	d := &schema.SchemaState{Triggers: map[string]*schema.Trigger{
		"public.orders/audit_orders": mkTrigState("D"),
	}}
	l := &schema.SchemaState{}
	chs := diffTriggers(d, l)
	if len(chs) != 2 {
		t.Fatalf("expected CREATE + DISABLE, got %d (%+v)", len(chs), chs)
	}
	if chs[0].kind != plan.ChangeCreateTrigger {
		t.Fatalf("expected CREATE first, got %s", chs[0].kind)
	}
	if !strings.Contains(chs[1].rawSQL, "DISABLE TRIGGER") {
		t.Fatalf("expected DISABLE follow-up, got %q", chs[1].rawSQL)
	}
}

func TestTriggerEnable_defChangeRebuildsAndAppliesState(t *testing.T) {
	// Body change + non-default Enabled: DROP + CREATE + DISABLE.
	d := &schema.SchemaState{Triggers: map[string]*schema.Trigger{
		"public.orders/audit_orders": {
			Schema: "public", Table: "orders", Name: "audit_orders",
			DefSQL:  "CREATE TRIGGER audit_orders BEFORE INSERT ON public.orders FOR EACH ROW EXECUTE FUNCTION public.f()",
			Enabled: "A",
		},
	}}
	l := &schema.SchemaState{Triggers: map[string]*schema.Trigger{
		"public.orders/audit_orders": mkTrigState("O"),
	}}
	chs := diffTriggers(d, l)
	if len(chs) != 3 {
		t.Fatalf("expected DROP + CREATE + ALWAYS, got %d", len(chs))
	}
	if chs[0].kind != plan.ChangeDropTrigger || chs[1].kind != plan.ChangeCreateTrigger {
		t.Fatalf("order off: %+v", chs)
	}
	if !strings.Contains(chs[2].rawSQL, "ENABLE ALWAYS TRIGGER") {
		t.Fatalf("expected ALWAYS, got %q", chs[2].rawSQL)
	}
}
