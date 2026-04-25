package shadow

import (
	"context"
	"testing"

	"github.com/nexg/pg-flux/pkg/plan"
	"github.com/stretchr/testify/require"
)

func TestValidateSemanticApply_Empty(t *testing.T) {
	require.NoError(t, ValidateSemanticApply(context.Background(), nil, nil))
	require.NoError(t, ValidateSemanticApply(context.Background(), nil, &plan.ExecutionPlan{}))
}
