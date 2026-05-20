package hazard

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultSeverity(t *testing.T) {
	require.Equal(t, SeverityBlocking, DefaultSeverity(DataLoss))
	require.Equal(t, SeverityAdvisory, DefaultSeverity(TableLock))
	require.Equal(t, SeverityBlocking, DefaultSeverity(Type("UNKNOWN_HAZARD")))
}
