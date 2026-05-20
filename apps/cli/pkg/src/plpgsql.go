package src

import (
	"strings"

	pgq "github.com/pganalyze/pg_query_go/v6"
)

// CheckPlPgSqlSource checks that a PL/pgSQL object parses with pg_query's Pl/pgSQL front-end
// (typically a full `CREATE [OR REPLACE] FUNCTION ... LANGUAGE plpgsql` or similar, not a bare
// `BEGIN ... END;` fragment — see pg_query tests).
// Empty or whitespace-only input is valid.
func CheckPlPgSqlSource(source string) error {
	if strings.TrimSpace(source) == "" {
		return nil
	}
	_, err := pgq.ParsePlPgSqlToJSON(source)
	return err
}
