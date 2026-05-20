package hazard

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStagedSetNotNullSteps(t *testing.T) {
	st := StagedSetNotNullSteps("public.t", "ck", "c", "c IS NOT NULL")
	require.GreaterOrEqual(t, len(st), 4)
	joined := strings.Join(st, " ")
	require.Contains(t, joined, "NOT VALID")
	require.Contains(t, joined, "VALIDATE CONSTRAINT")
}
