package inspector

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nexg/pg-flux/pkg/schema"
)

func fillColumns(ctx context.Context, pool *pgxpool.Pool, tableOID uint32, t *schema.Table) error {
	rows, err := pool.Query(ctx, `
		SELECT
			a.attname,
			pg_catalog.format_type(a.atttypid, a.atttypmod),
			a.attnotnull,
			coalesce(pg_get_expr(ad.adbin, ad.adrelid), '') AS def,
			a.attgenerated::text,
			a.attidentity::text
		FROM pg_attribute a
		LEFT JOIN pg_attrdef ad ON ad.adrelid = a.attrelid AND ad.adnum = a.attnum
		WHERE a.attrelid = $1 AND a.attnum > 0 AND NOT a.attisdropped
		ORDER BY a.attnum
	`, tableOID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var name, typ, def, attgenerated, attidentity string
		var notnull bool
		if err := rows.Scan(&name, &typ, &notnull, &def, &attgenerated, &attidentity); err != nil {
			return err
		}
		c := &schema.Column{
			Name:    strings.ToLower(name),
			TypeSQL: typ,
			NotNull: notnull,
		}
		if attgenerated == "s" {
			// Stored generated column: the catalog stores the expression in pg_attrdef.
			c.GeneratedExpr = strings.TrimSpace(def)
		} else {
			c.DefaultSQL = strings.TrimSpace(def)
		}
		switch attidentity {
		case "a":
			c.Identity = "always"
		case "d":
			c.Identity = "by-default"
		}
		t.Columns = append(t.Columns, c)
	}
	return rows.Err()
}

func fillPK(ctx context.Context, pool *pgxpool.Pool, tableOID uint32, t *schema.Table) error {
	rows, err := pool.Query(ctx, `
		SELECT a.attname
		FROM pg_index i
		JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = ANY(i.indkey)
		WHERE i.indrelid = $1 AND i.indisprimary
		ORDER BY a.attnum
	`, tableOID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return err
		}
		ln := strings.ToLower(name)
		t.PrimaryKeyCols = append(t.PrimaryKeyCols, ln)
		for _, col := range t.Columns {
			if col.Name == ln {
				col.IsPrimaryKey = true
			}
		}
	}
	return rows.Err()
}
