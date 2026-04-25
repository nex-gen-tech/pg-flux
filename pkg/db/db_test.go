package db

import (
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestParseConfig_BadURL(t *testing.T) {
	_, err := pgxpool.ParseConfig("::not-a-postgres-dsn::")
	require.Error(t, err)
}

func TestNewPool_MissingConnectionString(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	_, err := NewPool(t.Context(), "")
	require.Error(t, err)
}
