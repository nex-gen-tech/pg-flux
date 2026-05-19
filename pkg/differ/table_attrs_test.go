package differ

import (
	"strings"
	"testing"

	"github.com/nexg/pg-flux/pkg/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiffTableAttrs_unloggedFlip(t *testing.T) {
	des := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.t": {Schema: "public", Name: "t", Unlogged: true},
	}}
	live := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.t": {Schema: "public", Name: "t", Unlogged: false},
	}}
	dr, err := Diff(des, live, Options{})
	require.NoError(t, err)
	var saw bool
	for _, s := range dr.Plan.Statements {
		if strings.Contains(s.DDL, "ALTER TABLE public.t SET UNLOGGED") {
			saw = true
		}
	}
	assert.True(t, saw)
}

func TestDiffTableAttrs_reloptionsAddAndReset(t *testing.T) {
	des := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.t": {Schema: "public", Name: "t", ReLOptions: []string{"fillfactor=70"}},
	}}
	live := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.t": {Schema: "public", Name: "t", ReLOptions: []string{"autovacuum_enabled=false"}},
	}}
	dr, err := Diff(des, live, Options{})
	require.NoError(t, err)
	var setSeen, resetSeen bool
	for _, s := range dr.Plan.Statements {
		if strings.Contains(s.DDL, "SET (fillfactor = 70)") {
			setSeen = true
		}
		if strings.Contains(s.DDL, "RESET (autovacuum_enabled)") {
			resetSeen = true
		}
	}
	assert.True(t, setSeen, "expected SET reloption fillfactor=70")
	assert.True(t, resetSeen, "expected RESET autovacuum_enabled")
}

func TestDiffSeq_ownedByAndAs(t *testing.T) {
	des := &schema.SchemaState{Sequences: map[string]*schema.Sequence{
		"public.s": {Schema: "public", Name: "s", DefSQL: "CREATE SEQUENCE public.s", AsType: "integer", OwnedBy: "public.users.id"},
	}}
	live := &schema.SchemaState{Sequences: map[string]*schema.Sequence{
		"public.s": {Schema: "public", Name: "s", DefSQL: "CREATE SEQUENCE public.s", AsType: "bigint", OwnedBy: ""},
	}}
	dr, err := Diff(des, live, Options{})
	require.NoError(t, err)
	var ownedBy, asType bool
	for _, s := range dr.Plan.Statements {
		if strings.Contains(s.DDL, "OWNED BY public.users.id") {
			ownedBy = true
		}
		if strings.Contains(s.DDL, "ALTER SEQUENCE public.s AS integer") {
			asType = true
		}
	}
	assert.True(t, ownedBy)
	assert.True(t, asType)
}
