package src

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckPostgresSQLParse_PG18EdgeCases(t *testing.T) {
	// uuidv7() and temporal types — must parse under pg_query v6.
	require.NoError(t, CheckPostgresSQLParse(
		`CREATE TABLE u (id uuid PRIMARY KEY DEFAULT uuidv7());`,
	))
	require.NoError(t, CheckPostgresSQLParse(
		`CREATE TABLE t (x int, p tsrange, PRIMARY KEY (x));`,
	))
	// When pg_query adds full ENFORCED/NOT ENFORCED support, add a case here.
}
