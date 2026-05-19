package differ

import (
	"strings"
	"testing"

	"github.com/nexg/pg-flux/pkg/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiffColumnAttrs_storageChange(t *testing.T) {
	des := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.t": {Schema: "public", Name: "t", Columns: []*schema.Column{
			{Name: "body", TypeSQL: "text", Storage: "EXTERNAL"},
		}},
	}}
	live := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.t": {Schema: "public", Name: "t", Columns: []*schema.Column{
			{Name: "body", TypeSQL: "text", Storage: "EXTENDED"},
		}},
	}}
	dr, err := Diff(des, live, Options{})
	require.NoError(t, err)
	var saw bool
	for _, s := range dr.Plan.Statements {
		if strings.Contains(s.DDL, "SET STORAGE EXTERNAL") {
			saw = true
		}
	}
	assert.True(t, saw)
}

func TestDiffColumnAttrs_compressionChange(t *testing.T) {
	des := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.t": {Schema: "public", Name: "t", Columns: []*schema.Column{
			{Name: "body", TypeSQL: "text", Compression: "lz4"},
		}},
	}}
	live := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.t": {Schema: "public", Name: "t", Columns: []*schema.Column{
			{Name: "body", TypeSQL: "text", Compression: "pglz"},
		}},
	}}
	// Need PG14+ for lz4
	live.PGVersion.Major = 14
	dr, err := Diff(des, live, Options{})
	require.NoError(t, err)
	var saw bool
	for _, s := range dr.Plan.Statements {
		if strings.Contains(s.DDL, "SET COMPRESSION lz4") {
			saw = true
		}
	}
	assert.True(t, saw)
}

func TestDiffColumnAttrs_collateChange(t *testing.T) {
	des := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.t": {Schema: "public", Name: "t", Columns: []*schema.Column{
			{Name: "name", TypeSQL: "text", Collation: "en_US"},
		}},
	}}
	live := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.t": {Schema: "public", Name: "t", Columns: []*schema.Column{
			{Name: "name", TypeSQL: "text"},
		}},
	}}
	dr, err := Diff(des, live, Options{})
	require.NoError(t, err)
	var saw bool
	for _, s := range dr.Plan.Statements {
		if strings.Contains(s.DDL, "SET DATA TYPE text COLLATE") {
			saw = true
		}
	}
	assert.True(t, saw)
}
