package inspector

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nexg/pg-flux/pkg/schema"
)

func loadExtensionMap(ctx context.Context, pool *pgxpool.Pool) (map[string]*schema.Extension, error) {
	rows, err := pool.Query(ctx, `
		SELECT extname,
			extversion,
			'CREATE EXTENSION IF NOT EXISTS ' || quote_ident(extname) || ' CASCADE'
		FROM pg_extension
		WHERE extname NOT IN ('plpgsql')
		ORDER BY extname
	`)
	if err != nil {
		return nil, fmt.Errorf("pg_extension: %w", err)
	}
	defer rows.Close()
	out := make(map[string]*schema.Extension)
	for rows.Next() {
		var name, ver, def string
		if err := rows.Scan(&name, &ver, &def); err != nil {
			return nil, err
		}
		n := strings.ToLower(name)
		k := schema.ExtensionKey(n)
		out[k] = &schema.Extension{Name: n, DefSQL: def, Version: ver}
	}
	return out, rows.Err()
}
