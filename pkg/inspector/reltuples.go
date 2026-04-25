package inspector

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Reltuples returns an estimate of live rows (pg_class.reltuples) for a heap relation.
func Reltuples(ctx context.Context, pool *pgxpool.Pool, schema, table string) (float64, error) {
	if pool == nil {
		return 0, fmt.Errorf("nil pool")
	}
	if schema == "" {
		schema = "public"
	}
	var n float64
	err := pool.QueryRow(ctx, `
		SELECT c.reltuples::float8
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = $1 AND c.relname = $2 AND c.relkind IN ('r', 'p')
	`, schema, table).Scan(&n)
	return n, err
}
