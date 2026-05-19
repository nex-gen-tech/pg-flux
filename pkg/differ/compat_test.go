package differ

import (
	"testing"

	"github.com/nexg/pg-flux/pkg/pgver"
	"github.com/nexg/pg-flux/pkg/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Virtual generated columns are PG18+. When desired uses one and live is PG17,
// Diff must fail loud with a clear error naming the feature and version.
func TestCompat_virtualGeneratedRequiresPG18(t *testing.T) {
	des := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.t": {Schema: "public", Name: "t", Columns: []*schema.Column{
			{Name: "id", TypeSQL: "bigint"},
			{Name: "tag", TypeSQL: "text", GeneratedExpr: "upper(name)", GeneratedKind: "virtual"},
		}},
	}}
	live := &schema.SchemaState{
		PGVersion: pgver.Version{Major: 17, Minor: 0},
		Tables:    map[string]*schema.Table{},
	}
	_, err := Diff(des, live, Options{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "VIRTUAL")
	assert.Contains(t, err.Error(), "18+")

	// On PG18 the same diff succeeds.
	live.PGVersion = pgver.Version{Major: 18}
	_, err = Diff(des, live, Options{})
	assert.NoError(t, err)
}

// NULLS NOT DISTINCT is PG15+. On PG14 we fail loud; on PG15 it works.
func TestCompat_nullsNotDistinctRequiresPG15(t *testing.T) {
	des := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.t": {Schema: "public", Name: "t",
			Columns: []*schema.Column{{Name: "email", TypeSQL: "text"}},
			Uniques: []*schema.TableUnique{{Name: "t_email_uq", DefSQL: "UNIQUE NULLS NOT DISTINCT (email)", NullsNotDistinct: true}},
		},
	}}
	live := &schema.SchemaState{
		PGVersion: pgver.Version{Major: 14, Minor: 9},
		Tables:    map[string]*schema.Table{},
	}
	_, err := Diff(des, live, Options{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "NULLS NOT DISTINCT")
}

// NOT ENFORCED is PG18+.
func TestCompat_notEnforcedRequiresPG18(t *testing.T) {
	des := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.t": {Schema: "public", Name: "t",
			Columns: []*schema.Column{{Name: "n", TypeSQL: "integer"}},
			Checks:  []*schema.TableCheck{{Name: "t_n_pos", DefSQL: "CHECK (n >= 0) NOT ENFORCED", NotEnforced: true}},
		},
	}}
	live := &schema.SchemaState{
		PGVersion: pgver.Version{Major: 17},
		Tables:    map[string]*schema.Table{},
	}
	_, err := Diff(des, live, Options{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "NOT ENFORCED")
}
