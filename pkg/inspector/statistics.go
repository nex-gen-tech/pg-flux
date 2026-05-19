package inspector

import (
	"context"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nexg/pg-flux/pkg/schema"
)

// loadStatistics reads pg_statistic_ext and populates SchemaState.Statistics.
// Statistics kinds map: d=dependencies, f=mcv, m=mcv (legacy), n=ndistinct, e=expressions (PG14+).
func loadStatistics(ctx context.Context, pool *pgxpool.Pool, st *schema.SchemaState, schemas []string) error {
	if pool == nil || st == nil {
		return nil
	}
	rows, err := pool.Query(ctx, `
		SELECT
			n.nspname           AS stat_schema,
			s.stxname           AS stat_name,
			tn.nspname          AS tbl_schema,
			r.relname           AS tbl_name,
			s.stxkind::text[]   AS kinds,
			-- Columns from stxkeys (attnums) + expressions text from pg_get_statisticsobjdef_columns
			pg_get_statisticsobjdef_columns(s.oid) AS cols
		FROM pg_statistic_ext s
		JOIN pg_namespace n  ON n.oid = s.stxnamespace
		JOIN pg_class    r  ON r.oid = s.stxrelid
		JOIN pg_namespace tn ON tn.oid = r.relnamespace
		WHERE tn.nspname = ANY($1)
	`, schemas)
	if err != nil {
		return err
	}
	defer rows.Close()
	if st.Statistics == nil {
		st.Statistics = make(map[string]*schema.Statistics)
	}
	for rows.Next() {
		var statSch, statName, tblSch, tblName, cols string
		var kinds []string
		if err := rows.Scan(&statSch, &statName, &tblSch, &tblName, &kinds, &cols); err != nil {
			return err
		}
		stmt := &schema.Statistics{
			Schema:      strings.ToLower(statSch),
			Name:        strings.ToLower(statName),
			TableSchema: strings.ToLower(tblSch),
			TableName:   strings.ToLower(tblName),
		}
		// pg_statistic_ext.stxkind one-char codes (per src/include/catalog/pg_statistic_ext.h):
		//   d → ndistinct, f → functional-dependencies, m → mcv, e → expressions
		kindKeywords := make([]string, 0, len(kinds))
		for _, k := range kinds {
			switch k {
			case "d":
				kindKeywords = append(kindKeywords, "ndistinct")
			case "f":
				kindKeywords = append(kindKeywords, "dependencies")
			case "m":
				kindKeywords = append(kindKeywords, "mcv")
			case "e":
				kindKeywords = append(kindKeywords, "expressions")
			}
		}
		sort.Strings(kindKeywords)
		stmt.Kinds = kindKeywords
		stmt.Columns = splitStatColumns(cols)
		st.Statistics[schema.StatisticsKey(stmt.Schema, stmt.Name)] = stmt
	}
	return rows.Err()
}

// splitStatColumns parses the result of pg_get_statisticsobjdef_columns which returns
// a comma-separated list (with possible parenthesized expressions). We do a top-level
// split honoring parenthesis depth so an expression like "(lower(name))" stays intact.
func splitStatColumns(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	var cur strings.Builder
	depth := 0
	for _, r := range s {
		switch r {
		case '(':
			depth++
			cur.WriteRune(r)
		case ')':
			depth--
			cur.WriteRune(r)
		case ',':
			if depth == 0 {
				v := strings.TrimSpace(cur.String())
				if v != "" {
					out = append(out, v)
				}
				cur.Reset()
			} else {
				cur.WriteRune(r)
			}
		default:
			cur.WriteRune(r)
		}
	}
	if v := strings.TrimSpace(cur.String()); v != "" {
		out = append(out, v)
	}
	return out
}
