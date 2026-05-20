package differ

import (
	"strings"
	"testing"

	"github.com/nexg/pg-flux/pkg/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiffComments_addsTableAndColumnComments(t *testing.T) {
	des := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.users": {
			Schema: "public", Name: "users",
			Comment: "Identity record for every account.",
			Columns: []*schema.Column{
				{Name: "id", TypeSQL: "bigint", Comment: "Primary key"},
				{Name: "email", TypeSQL: "text"},
			},
		},
	}}
	live := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.users": {
			Schema: "public", Name: "users",
			Columns: []*schema.Column{
				{Name: "id", TypeSQL: "bigint"},
				{Name: "email", TypeSQL: "text"},
			},
		},
	}}
	dr, err := Diff(des, live, Options{})
	require.NoError(t, err)
	var tblCmt, colCmt bool
	for _, s := range dr.Plan.Statements {
		if strings.Contains(s.DDL, "COMMENT ON TABLE public.users") {
			tblCmt = true
			assert.Contains(t, s.DDL, "Identity record")
		}
		if strings.Contains(s.DDL, "COMMENT ON COLUMN public.users.id") {
			colCmt = true
			assert.Contains(t, s.DDL, "Primary key")
		}
	}
	assert.True(t, tblCmt, "expected COMMENT ON TABLE")
	assert.True(t, colCmt, "expected COMMENT ON COLUMN")
}

func TestDiffComments_clearsWithNULL(t *testing.T) {
	des := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.t": {Schema: "public", Name: "t", Comment: ""},
	}}
	live := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.t": {Schema: "public", Name: "t", Comment: "stale doc"},
	}}
	dr, err := Diff(des, live, Options{})
	require.NoError(t, err)
	var cleared bool
	for _, s := range dr.Plan.Statements {
		if strings.Contains(s.DDL, "COMMENT ON TABLE public.t IS NULL") {
			cleared = true
		}
	}
	assert.True(t, cleared, "expected COMMENT ON TABLE … IS NULL when desired comment is empty")
}

func TestDiffComments_noChange_emitsNothing(t *testing.T) {
	des := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.t": {Schema: "public", Name: "t", Comment: "same"},
	}}
	live := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.t": {Schema: "public", Name: "t", Comment: "same"},
	}}
	dr, err := Diff(des, live, Options{})
	require.NoError(t, err)
	for _, s := range dr.Plan.Statements {
		assert.NotContains(t, s.DDL, "COMMENT ON", "should not emit COMMENT ON when comments match")
	}
}

// commentLiteral edge cases — single quotes are doubled, empty becomes NULL.
func TestCommentLiteral(t *testing.T) {
	assert.Equal(t, "NULL", commentLiteral(""))
	assert.Equal(t, "NULL", commentLiteral("   "))
	assert.Equal(t, "'hello'", commentLiteral("hello"))
	assert.Equal(t, "'it''s'", commentLiteral("it's"))
}
