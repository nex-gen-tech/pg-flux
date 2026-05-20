package inspector

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReltuples_NilPool(t *testing.T) {
	_, err := Reltuples(context.Background(), nil, "public", "t")
	require.Error(t, err)
}
