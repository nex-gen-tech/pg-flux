package differ

import (
	"strings"
	"testing"

	"github.com/nexg/pg-flux/pkg/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiffFunctionMetadata_volatilityChange(t *testing.T) {
	des := &schema.SchemaState{Functions: map[string]*schema.Function{
		"public.f()": {
			Schema: "public", Name: "f", Identity: "public.f()", DefSQL: "X", Kind: "f",
			Volatility: "IMMUTABLE", Security: "INVOKER", Parallel: "SAFE",
		},
	}}
	live := &schema.SchemaState{Functions: map[string]*schema.Function{
		"public.f()": {
			Schema: "public", Name: "f", Identity: "public.f()", DefSQL: "X", Kind: "f",
			Volatility: "VOLATILE", Security: "INVOKER", Parallel: "SAFE",
		},
	}}
	dr, err := Diff(des, live, Options{})
	require.NoError(t, err)
	var saw bool
	for _, s := range dr.Plan.Statements {
		if strings.Contains(s.DDL, "ALTER FUNCTION public.f() IMMUTABLE") {
			saw = true
		}
	}
	assert.True(t, saw)
}

func TestDiffFunctionMetadata_securityAndLeakproof(t *testing.T) {
	des := &schema.SchemaState{Functions: map[string]*schema.Function{
		"public.f()": {
			Identity: "public.f()", DefSQL: "X", Kind: "f",
			Security: "DEFINER", LeakProof: true, Parallel: "SAFE",
		},
	}}
	live := &schema.SchemaState{Functions: map[string]*schema.Function{
		"public.f()": {
			Identity: "public.f()", DefSQL: "X", Kind: "f",
			Security: "INVOKER", LeakProof: false, Parallel: "SAFE",
		},
	}}
	dr, err := Diff(des, live, Options{})
	require.NoError(t, err)
	var sec, leak bool
	for _, s := range dr.Plan.Statements {
		if strings.Contains(s.DDL, "SECURITY DEFINER") {
			sec = true
		}
		if strings.Contains(s.DDL, "LEAKPROOF") && !strings.Contains(s.DDL, "NOT LEAKPROOF") {
			leak = true
		}
	}
	assert.True(t, sec, "expected SECURITY DEFINER alter")
	assert.True(t, leak, "expected LEAKPROOF alter")
}

func TestDiffFunctionMetadata_searchPathSET(t *testing.T) {
	des := &schema.SchemaState{Functions: map[string]*schema.Function{
		"public.f()": {
			Identity: "public.f()", DefSQL: "X", Kind: "f",
			Config: []string{"search_path=public, pg_temp"},
		},
	}}
	live := &schema.SchemaState{Functions: map[string]*schema.Function{
		"public.f()": {Identity: "public.f()", DefSQL: "X", Kind: "f"},
	}}
	dr, err := Diff(des, live, Options{})
	require.NoError(t, err)
	var saw bool
	for _, s := range dr.Plan.Statements {
		if strings.Contains(s.DDL, "SET search_path TO") {
			saw = true
		}
	}
	assert.True(t, saw, "expected SET search_path alter")
}

func TestConfigDiff_addAndRemove(t *testing.T) {
	out := configDiff(
		[]string{"search_path=public"},
		[]string{"timezone=UTC"},
	)
	// Should add search_path and reset timezone
	joined := strings.Join(out, " ")
	assert.Contains(t, joined, "SET search_path TO")
	assert.Contains(t, joined, "RESET timezone")
}
