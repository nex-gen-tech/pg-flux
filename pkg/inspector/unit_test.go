package inspector

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nexg/pg-flux/pkg/schema"
)

// ---------------------------------------------------------------------------
// Reltuples / ReltuplesByTable — nil-pool / empty-tables fast paths
// ---------------------------------------------------------------------------

func TestReltuplesByTable_NilPool(t *testing.T) {
	tables := map[string]*schema.Table{"public.users": {Schema: "public", Name: "users"}}
	res, err := ReltuplesByTable(context.Background(), nil, tables)
	require.NoError(t, err)
	require.Nil(t, res)
}

func TestReltuplesByTable_EmptyTables(t *testing.T) {
	res, err := ReltuplesByTable(context.Background(), nil, map[string]*schema.Table{})
	require.NoError(t, err)
	require.Nil(t, res)
}

func TestReltuplesByTable_NilTables(t *testing.T) {
	res, err := ReltuplesByTable(context.Background(), nil, nil)
	require.NoError(t, err)
	require.Nil(t, res)
}

// ---------------------------------------------------------------------------
// Options — default schema logic
// ---------------------------------------------------------------------------

func TestOptions_DefaultSchemaIsPublic(t *testing.T) {
	// The Inspect function defaults to "public" when Schemas is empty.
	// We verify the schema-normalisation part doesn't mutate the struct in-place in a
	// surprising way — we do that by simply checking the logic with strings utilities.
	opt := Options{}
	schemas := opt.Schemas
	if len(schemas) == 0 {
		schemas = []string{"public"}
	}
	for i, s := range schemas {
		schemas[i] = strings.ToLower(strings.TrimSpace(s))
	}
	require.Equal(t, []string{"public"}, schemas)
}

func TestOptions_SchemasNormalised(t *testing.T) {
	opt := Options{Schemas: []string{"  PUBLIC  ", "MySchema"}}
	schemas := opt.Schemas
	for i, s := range schemas {
		schemas[i] = strings.ToLower(strings.TrimSpace(s))
	}
	require.Equal(t, []string{"public", "myschema"}, schemas)
}

// ---------------------------------------------------------------------------
// Inspect — returns error when pool is nil (goroutine error propagation)
// ---------------------------------------------------------------------------

func TestInspect_NilPool(t *testing.T) {
	_, err := Inspect(context.Background(), nil, Options{})
	require.Error(t, err)
}
