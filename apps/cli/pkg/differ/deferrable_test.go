package differ

import (
	"strings"
	"testing"

	"github.com/nex-gen-tech/pg-flux/pkg/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// When source declares an FK as DEFERRABLE INITIALLY DEFERRED and live has the same
// FK declared as non-deferrable, the fingerprint-based table-constraint diff should
// see the difference and emit DROP+ADD (PG cannot ALTER a constraint's deferrability
// in place without DROP+ADD).
func TestDiffConstraints_deferrableChange(t *testing.T) {
	des := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.orders": {
			Schema: "public", Name: "orders",
			Columns: []*schema.Column{{Name: "user_id", TypeSQL: "bigint"}},
			ForeignKeys: []*schema.TableForeignKey{{
				Name:              "orders_user_fk",
				DefSQL:            "FOREIGN KEY (user_id) REFERENCES users(id) DEFERRABLE INITIALLY DEFERRED",
				Deferrable:        true,
				InitiallyDeferred: true,
			}},
		},
		"public.users": {Schema: "public", Name: "users", Columns: []*schema.Column{{Name: "id", TypeSQL: "bigint"}}},
	}}
	live := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.orders": {
			Schema: "public", Name: "orders",
			Columns: []*schema.Column{{Name: "user_id", TypeSQL: "bigint"}},
			ForeignKeys: []*schema.TableForeignKey{{
				Name:   "orders_user_fk",
				DefSQL: "FOREIGN KEY (user_id) REFERENCES users(id)",
			}},
		},
		"public.users": {Schema: "public", Name: "users", Columns: []*schema.Column{{Name: "id", TypeSQL: "bigint"}}},
	}}
	dr, err := Diff(des, live, Options{})
	require.NoError(t, err)
	var dropped, added bool
	for _, s := range dr.Plan.Statements {
		if strings.Contains(s.DDL, "DROP CONSTRAINT") && strings.Contains(s.DDL, "orders_user_fk") {
			dropped = true
		}
		if strings.Contains(s.DDL, "ADD CONSTRAINT orders_user_fk") && strings.Contains(s.DDL, "DEFERRABLE") {
			added = true
		}
	}
	assert.True(t, dropped, "expected DROP CONSTRAINT for deferrability change")
	assert.True(t, added, "expected ADD CONSTRAINT with DEFERRABLE clause")
}

func TestForeignKeyModel_MatchType(t *testing.T) {
	fk := &schema.TableForeignKey{Name: "f", DefSQL: "FOREIGN KEY (a) REFERENCES b(c) MATCH FULL", MatchType: "FULL"}
	assert.Equal(t, "FULL", fk.MatchType)
}
