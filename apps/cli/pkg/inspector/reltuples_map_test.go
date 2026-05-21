package inspector

import (
	"context"
	"testing"

	"github.com/nex-gen-tech/pg-flux/pkg/schema"
	"github.com/stretchr/testify/require"
)

func TestReltuplesByTable_Nil(t *testing.T) {
	m, err := ReltuplesByTable(context.Background(), nil, map[string]*schema.Table{"a.b": {Schema: "a", Name: "b"}})
	require.NoError(t, err)
	require.Nil(t, m)
}

func TestReltuplesByTable_EmptyMap(t *testing.T) {
	m, err := ReltuplesByTable(context.Background(), nil, map[string]*schema.Table{})
	require.NoError(t, err)
	require.Nil(t, m)
}
