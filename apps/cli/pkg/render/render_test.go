package render

import (
	"bytes"
	"testing"

	"github.com/nex-gen-tech/pg-flux/pkg/hazard"
	"github.com/nex-gen-tech/pg-flux/pkg/plan"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAllowHazards(t *testing.T) {
	m := ParseAllowHazards("DATA_LOSS, TABLE_LOCK")
	assert.True(t, m[hazard.DataLoss])
	assert.True(t, m[hazard.TableLock])
}

func TestParseAllowHazards_empty(t *testing.T) {
	require.Nil(t, ParseAllowHazards(""))
}

func TestParseAllowHazards_whitespaceOnly(t *testing.T) {
	// "  " has no non-empty tokens after splitting, so result is an empty map (not nil)
	m := ParseAllowHazards("  ")
	assert.Empty(t, m)
}

func TestDriftToJSON(t *testing.T) {
	var buf bytes.Buffer
	err := DriftToJSON(&buf, DriftJSON{IsDrift: true, Differences: []Difference{
		{ObjectType: "table", ObjectName: "t", ChangeType: "CREATE", Details: "x"},
	}})
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "is_drift")
}

func TestPlanToJSON_EmitsHazards(t *testing.T) {
	p := &plan.ExecutionPlan{Statements: []plan.Statement{{
		ID: 1, OpType: "X", DDL: "SELECT 1", Object: "o",
		Hazards: []hazard.Detected{{
			Type: hazard.DataLoss, Severity: hazard.SeverityBlocking, Message: "m",
		}},
	}}}
	var buf bytes.Buffer
	err := PlanToJSON(&buf, p, "a", "b", nil)
	require.NoError(t, err)
	s := buf.String()
	assert.Contains(t, s, "DATA_LOSS")
	assert.Contains(t, s, `"hazards"`)
}

func TestPlanToJSON_nilPlan(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, PlanToJSON(&buf, nil, "", "", nil))
	assert.Contains(t, buf.String(), "statements")
}

func TestPlanToJSON_blockingHazards(t *testing.T) {
	p := &plan.ExecutionPlan{Statements: []plan.Statement{{
		ID: 1, OpType: "X", DDL: "DROP TABLE t",
		BlockingHazards: true,
		Hazards: []hazard.Detected{{
			Type: hazard.DataLoss, Severity: hazard.SeverityBlocking, Message: "drops data",
		}},
	}}}
	var buf bytes.Buffer
	require.NoError(t, PlanToJSON(&buf, p, "src", "live", nil))
	assert.Contains(t, buf.String(), "has_blocking_hazards")
}
