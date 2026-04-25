package src

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckPlPgSqlSource(t *testing.T) {
	require.NoError(t, CheckPlPgSqlSource(""))
	require.NoError(t, CheckPlPgSqlSource("\n  "))
	require.NoError(t, CheckPlPgSqlSource(
		`CREATE OR REPLACE FUNCTION f_pgflux() RETURNS int AS $$ BEGIN RETURN 1; END; $$ LANGUAGE plpgsql;`,
	))
	require.Error(t, CheckPlPgSqlSource("CREATE OR REPLACE FUNCTION bad() AS $$ { $$ LANGUAGE plpgsql;"))
}
