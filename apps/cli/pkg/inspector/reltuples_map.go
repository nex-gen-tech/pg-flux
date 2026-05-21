package inspector

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nex-gen-tech/pg-flux/pkg/schema"
)

// ReltuplesByTable returns pg_class.reltuples for each heap in tables (keys "schema.name").
func ReltuplesByTable(ctx context.Context, pool *pgxpool.Pool, tables map[string]*schema.Table) (map[string]float64, error) {
	if pool == nil || len(tables) == 0 {
		return nil, nil
	}
	out := make(map[string]float64, len(tables))
	for k, t := range tables {
		if t == nil {
			continue
		}
		sch := t.Schema
		if sch == "" {
			sch = "public"
		}
		n, err := Reltuples(ctx, pool, sch, t.Name)
		if err != nil {
			return nil, err
		}
		out[strings.ToLower(k)] = n
	}
	return out, nil
}
