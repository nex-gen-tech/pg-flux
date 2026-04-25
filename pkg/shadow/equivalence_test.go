package shadow

import (
	"context"
	"testing"
	"time"

	"github.com/nexg/pg-flux/pkg/differ"
	"github.com/nexg/pg-flux/pkg/plan"
	"github.com/nexg/pg-flux/pkg/schema"
	"github.com/stretchr/testify/require"
)

func TestValidateStructuralEquivalence_validation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := ValidateStructuralEquivalence(ctx, "", nil, &plan.ExecutionPlan{}, differ.Options{})
	require.Error(t, err)
	err = ValidateStructuralEquivalence(ctx, "postgres://x@y/z", nil, &plan.ExecutionPlan{}, differ.Options{})
	require.Error(t, err)
	err = ValidateStructuralEquivalence(ctx, "postgres://x@y/z", &schema.SchemaState{}, nil, differ.Options{})
	require.Error(t, err)
}

func TestTargetSchemasFromDesired_includesPublic(t *testing.T) {
	s := &schema.SchemaState{Tables: map[string]*schema.Table{
		"app.m":   {Schema: "app", Name: "m"},
		"public.p": {Schema: "public", Name: "p"},
	}}
	out := targetSchemasFromDesired(s)
	require.Contains(t, out, "public")
	require.Contains(t, out, "app")
}
