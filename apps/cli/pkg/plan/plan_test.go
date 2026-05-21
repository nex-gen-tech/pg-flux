package plan

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nex-gen-tech/pg-flux/pkg/hazard"
)

func TestHasBlockingHazards_Allow(t *testing.T) {
	p := &ExecutionPlan{Statements: []Statement{{
		Hazards: []hazard.Detected{{Type: hazard.DataLoss, Severity: hazard.SeverityBlocking}},
	}}}
	allow := map[hazard.Type]bool{hazard.DataLoss: true}
	require.False(t, p.HasBlockingHazards(allow))
}

func TestHasBlockingHazards_NilPlan(t *testing.T) {
	require.False(t, (*ExecutionPlan)(nil).HasBlockingHazards(nil))
}

func TestHasBlockingHazards_AdvisoryIgnored(t *testing.T) {
	p := &ExecutionPlan{Statements: []Statement{{
		Hazards: []hazard.Detected{{Type: hazard.DataLoss, Severity: hazard.SeverityAdvisory}},
	}}}
	require.False(t, p.HasBlockingHazards(nil))
}

func TestHasBlockingHazards_Unmitigated(t *testing.T) {
	p := &ExecutionPlan{Statements: []Statement{{
		Hazards: []hazard.Detected{{Type: hazard.DataLoss, Severity: hazard.SeverityBlocking}},
	}}}
	require.True(t, p.HasBlockingHazards(nil))
}
