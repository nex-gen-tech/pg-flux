package differ

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nexg/pg-flux/pkg/schema"
)

func TestDiff_ExtensionVersionEmitsUpdate(t *testing.T) {
	des := &schema.SchemaState{
		Tables: map[string]*schema.Table{},
		Extensions: map[string]*schema.Extension{
			"myext": {Name: "myext", Version: "1.1", DefSQL: "CREATE EXTENSION myext WITH VERSION '1.1'"},
		},
	}
	live := &schema.SchemaState{
		Tables: map[string]*schema.Table{},
		Extensions: map[string]*schema.Extension{
			"myext": {Name: "myext", Version: "1.0", DefSQL: "CREATE EXTENSION IF NOT EXISTS myext"},
		},
	}
	dr, err := Diff(des, live, Options{})
	require.NoError(t, err)
	var found bool
	for _, s := range dr.Plan.Statements {
		if strings.Contains(strings.ToUpper(s.DDL), "ALTER EXTENSION") && strings.Contains(s.DDL, "UPDATE TO") {
			found = true
		}
	}
	require.True(t, found, "expected ALTER EXTENSION ... UPDATE TO in plan")
}
