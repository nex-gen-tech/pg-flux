package shadow

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nexg/pg-flux/pkg/plan"
)

func TestValidateSyntaxInTxn_NilOrEmpty(t *testing.T) {
	err := ValidateSyntaxInTxn(context.Background(), nil, nil)
	require.NoError(t, err)
	err = ValidateSyntaxInTxn(context.Background(), nil, &plan.ExecutionPlan{})
	require.NoError(t, err)
}
