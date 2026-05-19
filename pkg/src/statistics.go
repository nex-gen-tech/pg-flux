package src

import (
	"sort"
	"strings"

	pgq "github.com/pganalyze/pg_query_go/v6"

	"github.com/nexg/pg-flux/pkg/schema"
)

// captureCreateStatistics parses a CREATE STATISTICS statement into the schema model.
func captureCreateStatistics(s *pgq.CreateStatsStmt, st *schema.SchemaState) error {
	if s == nil || st == nil {
		return nil
	}
	if st.Statistics == nil {
		st.Statistics = make(map[string]*schema.Statistics)
	}
	stat := &schema.Statistics{}
	// stats name (List of String parts)
	defs := s.GetDefnames()
	switch len(defs) {
	case 0:
		// Anonymous statistics: PG auto-generates name. Skip for now.
		return nil
	case 1:
		stat.Schema = "public"
		stat.Name = strings.ToLower(defs[0].GetString_().GetSval())
	case 2:
		stat.Schema = strings.ToLower(defs[0].GetString_().GetSval())
		stat.Name = strings.ToLower(defs[1].GetString_().GetSval())
	}
	// Relation: FROM rel — RangeVar (we only handle single-relation stats; pg supports list but rarely used)
	rels := s.GetRelations()
	if len(rels) > 0 {
		if rv := rels[0].GetRangeVar(); rv != nil {
			stat.TableSchema = strings.ToLower(rv.GetSchemaname())
			if stat.TableSchema == "" {
				stat.TableSchema = "public"
			}
			stat.TableName = strings.ToLower(rv.GetRelname())
		}
	}
	// Kinds
	for _, k := range s.GetStatTypes() {
		if str := k.GetString_(); str != nil {
			stat.Kinds = append(stat.Kinds, strings.ToLower(str.GetSval()))
		}
	}
	sort.Strings(stat.Kinds)
	// Columns / expressions — exprs is a list of StatsElem
	for _, e := range s.GetExprs() {
		// pg_query encodes both column refs and full expressions inside StatsElem;
		// for simplicity we deparse each node back to SQL text.
		if expr, err := deparseExprToSQL(e); err == nil && strings.TrimSpace(expr) != "" {
			stat.Columns = append(stat.Columns, strings.TrimSpace(expr))
		}
	}
	st.Statistics[schema.StatisticsKey(stat.Schema, stat.Name)] = stat
	return nil
}
