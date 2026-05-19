package differ

import (
	"strings"
	"testing"

	"github.com/nexg/pg-flux/pkg/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventTriggerDiff_create(t *testing.T) {
	des := &schema.SchemaState{EventTriggers: map[string]*schema.EventTrigger{
		"audit_dml": {Name: "audit_dml", Event: "ddl_command_end", Function: "public.audit()", Tags: []string{"CREATE TABLE"}},
	}}
	live := &schema.SchemaState{}
	dr, err := Diff(des, live, Options{})
	require.NoError(t, err)
	var saw bool
	for _, s := range dr.Plan.Statements {
		if strings.Contains(s.DDL, "CREATE EVENT TRIGGER audit_dml ON ddl_command_end") {
			saw = true
			assert.Contains(t, s.DDL, "WHEN TAG IN ('CREATE TABLE')")
			assert.Contains(t, s.DDL, "EXECUTE FUNCTION public.audit()")
		}
	}
	assert.True(t, saw)
}

func TestEventTriggerDiff_drop(t *testing.T) {
	des := &schema.SchemaState{}
	live := &schema.SchemaState{EventTriggers: map[string]*schema.EventTrigger{
		"audit_dml": {Name: "audit_dml", Event: "ddl_command_end", Function: "public.audit()"},
	}}
	dr, err := Diff(des, live, Options{})
	require.NoError(t, err)
	var saw bool
	for _, s := range dr.Plan.Statements {
		if strings.Contains(s.DDL, "DROP EVENT TRIGGER IF EXISTS audit_dml") {
			saw = true
		}
	}
	assert.True(t, saw)
}

func TestEventTriggerDiff_definitionChange(t *testing.T) {
	des := &schema.SchemaState{EventTriggers: map[string]*schema.EventTrigger{
		"audit_dml": {Name: "audit_dml", Event: "ddl_command_end", Function: "public.audit_v2()"},
	}}
	live := &schema.SchemaState{EventTriggers: map[string]*schema.EventTrigger{
		"audit_dml": {Name: "audit_dml", Event: "ddl_command_end", Function: "public.audit()"},
	}}
	dr, err := Diff(des, live, Options{})
	require.NoError(t, err)
	var drop, create bool
	for _, s := range dr.Plan.Statements {
		if strings.Contains(s.DDL, "DROP EVENT TRIGGER IF EXISTS audit_dml") {
			drop = true
		}
		if strings.Contains(s.DDL, "CREATE EVENT TRIGGER audit_dml") && strings.Contains(s.DDL, "audit_v2") {
			create = true
		}
	}
	assert.True(t, drop)
	assert.True(t, create)
}
