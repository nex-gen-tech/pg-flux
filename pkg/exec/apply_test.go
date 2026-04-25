package exec

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nexg/pg-flux/pkg/plan"
)

func TestApply_EmptyPlan(t *testing.T) {
	err := Apply(context.Background(), nil, &plan.ExecutionPlan{Statements: []plan.Statement{}}, Options{})
	require.NoError(t, err)
	err = Apply(context.Background(), nil, nil, Options{})
	require.NoError(t, err)
}

func TestApply_DryRunNoDB(t *testing.T) {
	p := &plan.ExecutionPlan{Statements: []plan.Statement{
		{ID: 1, DDL: "SELECT 1"},
	}}
	err := Apply(context.Background(), nil, p, Options{DryRun: true})
	require.NoError(t, err)
}
