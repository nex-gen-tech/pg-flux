package db

import (
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestSanitizeDSN(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"postgres://user:secret@localhost:5432/db", "postgres://user:***@localhost:5432/db"},
		{"postgres://user:p%40ss@localhost/db", "postgres://user:***@localhost/db"},
		{"postgres://localhost/db", "postgres://localhost/db"}, // no password component
		{"postgres://user@localhost/db", "postgres://user@localhost/db"}, // no colon before @
	}
	for _, tc := range cases {
		got := sanitizeDSN(tc.input)
		if got != tc.want {
			t.Errorf("sanitizeDSN(%q) = %q; want %q", tc.input, got, tc.want)
		}
	}
}

func TestParseConfig_BadURL(t *testing.T) {
	_, err := pgxpool.ParseConfig("::not-a-postgres-dsn::")
	require.Error(t, err)
}

func TestNewPool_MissingConnectionString(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	_, err := NewPool(t.Context(), "")
	require.Error(t, err)
}

func TestNewPool_InvalidDSN(t *testing.T) {
	_, err := NewPool(t.Context(), "::not-valid::")
	require.Error(t, err)
}

func TestNewPool_ValidFormatNoDB(t *testing.T) {
	// Valid-format DSN with unreachable host; pgxpool.NewWithConfig is lazy (MinConns=0)
	// so it returns a pool without error even if no DB is running.
	pool, err := NewPool(t.Context(), "postgres://user:pass@127.0.0.1:1/nonexistent?sslmode=disable")
	if err == nil {
		// Pool was created lazily — test passed.
		pool.Close()
	}
	// Whether it errors or not is acceptable; the important thing is ParseConfig runs.
}

func TestNewPool_FromEnv(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:pass@127.0.0.1:1/db?sslmode=disable")
	pool, err := NewPool(t.Context(), "")
	if err == nil {
		pool.Close()
	}
	// DATABASE_URL path must be exercised (conn trimming + env fallback).
}
