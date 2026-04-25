package differ

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nexg/pg-flux/pkg/hazard"
	"github.com/nexg/pg-flux/pkg/plan"
	"github.com/nexg/pg-flux/pkg/schema"
)

func TestEnrichReltupleStagedSetNotNull(t *testing.T) {
	stmts := []plan.Statement{{
		OpType:  "SET_NOT_NULL",
		DDL:     `ALTER TABLE public.t ALTER COLUMN c SET NOT NULL`,
		Object:  "public.t",
		Hazards: []hazard.Detected{{Type: hazard.ConstraintScan, Severity: hazard.SeverityBlocking}},
	}}
	enrichHazardsFromOptions(&stmts, Options{
		Reltuples:                   map[string]float64{"public.t": 2e6},
		SetNotNullReltupleThreshold: 1e6,
	})
	var found bool
	for _, h := range stmts[0].Hazards {
		if h.Type == hazard.StagedSetNotNull {
			found = true
		}
	}
	require.True(t, found)
}

func TestDiffExtensionsNilDesiredNoDrop(t *testing.T) {
	d := &schema.SchemaState{Tables: map[string]*schema.Table{}}
	l := &schema.SchemaState{Tables: map[string]*schema.Table{}, Extensions: map[string]*schema.Extension{
		"pgcrypto": {Name: "pgcrypto", DefSQL: "CREATE EXTENSION pgcrypto"},
	}}
	out := diffExtensions(d, l)
	require.Empty(t, out)
}
