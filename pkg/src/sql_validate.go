package src

import (
	"strings"

	pgq "github.com/pganalyze/pg_query_go/v6"
)

// CheckPostgresSQLParse ensures a single statement parses with pg_query (FR-01 validation hook).
func CheckPostgresSQLParse(sql string) error {
	sql = strings.TrimSpace(strings.ReplaceAll(sql, "\r\n", "\n"))
	if sql == "" {
		return nil
	}
	_, err := pgq.Parse(sql)
	return err
}
