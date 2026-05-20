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
			a.attidentity::text,
			a.attstorage::text,
			COALESCE(a.attcompression::text, '') AS attcompression,
			COALESCE((SELECT collname FROM pg_collation co WHERE co.oid = a.attcollation), '') AS attcollname
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
		var name, typ, def, attgenerated, attidentity, attstorage, attcompression, attcollname string
		var notnull bool
		if err := rows.Scan(&name, &typ, &notnull, &def, &attgenerated, &attidentity, &attstorage, &attcompression, &attcollname); err != nil {
			return err
		}
		c := &schema.Column{
			Name:    strings.ToLower(name),
			TypeSQL: typ,
			NotNull: notnull,
		}
		switch attstorage {
		case "p":
			c.Storage = "PLAIN"
		case "e":
			c.Storage = "EXTERNAL"
		case "m":
			c.Storage = "MAIN"
		case "x":
			c.Storage = "EXTENDED"
		}
		switch attcompression {
		case "l":
			c.Compression = "lz4"
		case "p":
			c.Compression = "pglz"
		}
		// Only record collation when it differs from "default" (which means use the
		// column type's default collation). Preserve case — "C" and "c" are distinct.
		if attcollname != "" && attcollname != "default" {
			c.Collation = attcollname
		}
		switch attgenerated {
		case "s":
			c.GeneratedExpr = strings.TrimSpace(def)
			c.GeneratedKind = "stored"
		case "v":
			c.GeneratedExpr = strings.TrimSpace(def)
			c.GeneratedKind = "virtual"
		default:
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
