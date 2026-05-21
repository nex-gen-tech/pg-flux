package differ

import (
	"strings"
	"testing"

	"github.com/nex-gen-tech/pg-flux/pkg/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// When a trigger definition changes, the differ emits DROP_TRIGGER + CREATE_TRIGGER.
// The DAG sort MUST place DROP before CREATE — otherwise PG rejects the CREATE with
// "trigger already exists". Regression guard.
func TestTrigger_dropPrecedesCreateOnRedefine(t *testing.T) {
	des := &schema.SchemaState{
		Tables: map[string]*schema.Table{
			"public.posts": {Schema: "public", Name: "posts", Columns: []*schema.Column{{Name: "id", TypeSQL: "bigint"}}},
		},
		Triggers: map[string]*schema.Trigger{
			"public.posts/t_updated": {
				Schema: "public", Table: "posts", Name: "t_updated",
				DefSQL: "CREATE TRIGGER t_updated BEFORE INSERT OR UPDATE ON public.posts FOR EACH ROW EXECUTE FUNCTION public.f()",
			},
		},
	}
	live := &schema.SchemaState{
		Tables: map[string]*schema.Table{
			"public.posts": {Schema: "public", Name: "posts", Columns: []*schema.Column{{Name: "id", TypeSQL: "bigint"}}},
		},
		Triggers: map[string]*schema.Trigger{
			"public.posts/t_updated": {
				Schema: "public", Table: "posts", Name: "t_updated",
				DefSQL: "CREATE TRIGGER t_updated BEFORE UPDATE ON public.posts FOR EACH ROW EXECUTE FUNCTION public.f()",
			},
		},
	}
	dr, err := Diff(des, live, Options{})
	require.NoError(t, err)
	dropIdx, createIdx := -1, -1
	for i, s := range dr.Plan.Statements {
		if strings.Contains(s.DDL, "DROP TRIGGER IF EXISTS t_updated") {
			dropIdx = i
		}
		if strings.Contains(s.DDL, "CREATE TRIGGER t_updated") {
			createIdx = i
		}
	}
	require.GreaterOrEqual(t, dropIdx, 0, "expected DROP TRIGGER in plan")
	require.GreaterOrEqual(t, createIdx, 0, "expected CREATE TRIGGER in plan")
	assert.Less(t, dropIdx, createIdx, "DROP_TRIGGER must run before CREATE_TRIGGER (replace-by-name)")
}
