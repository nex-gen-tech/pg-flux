package inspector

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nexg/pg-flux/pkg/schema"
)

// loadExoticObjects populates inspectable but rarely-used object kinds:
//   - composite types (pg_type typtype='c' joined with pg_class+pg_attribute)
//   - range types (pg_range)
//   - foreign servers (pg_foreign_server) + foreign tables (pg_foreign_table)
//   - publications (pg_publication) + subscriptions (pg_subscription, when readable)
//
// All entries are populated even when the schema filter excludes them — these
// objects are mostly database-wide. Foreign tables are schema-scoped.
func loadExoticObjects(ctx context.Context, pool *pgxpool.Pool, st *schema.SchemaState, schemas []string) error {
	if pool == nil || st == nil {
		return nil
	}
	if err := loadCompositeTypes(ctx, pool, st, schemas); err != nil {
		return fmt.Errorf("load composite types: %w", err)
	}
	if err := loadRangeTypes(ctx, pool, st, schemas); err != nil {
		return fmt.Errorf("load range types: %w", err)
	}
	if err := loadForeignServers(ctx, pool, st); err != nil {
		return fmt.Errorf("load foreign servers: %w", err)
	}
	if err := loadForeignTables(ctx, pool, st, schemas); err != nil {
		return fmt.Errorf("load foreign tables: %w", err)
	}
	if err := loadPublications(ctx, pool, st); err != nil {
		return fmt.Errorf("load publications: %w", err)
	}
	// pg_subscription requires superuser; tolerate permission errors silently.
	_ = loadSubscriptions(ctx, pool, st)
	return nil
}

func loadCompositeTypes(ctx context.Context, pool *pgxpool.Pool, st *schema.SchemaState, schemas []string) error {
	rows, err := pool.Query(ctx, `
		SELECT n.nspname, t.typname,
		       (SELECT array_agg(format('%I %s', a.attname, format_type(a.atttypid, a.atttypmod)) ORDER BY a.attnum)
		        FROM pg_attribute a
		        WHERE a.attrelid = t.typrelid AND a.attnum > 0 AND NOT a.attisdropped) AS attrs,
		       pg_get_userbyid(t.typowner) AS owner
		FROM pg_type t
		JOIN pg_namespace n ON n.oid = t.typnamespace
		WHERE t.typtype = 'c' AND t.typrelid IS NOT NULL AND t.typrelid <> 0
		  AND NOT EXISTS (SELECT 1 FROM pg_class c WHERE c.oid = t.typrelid AND c.relkind <> 'c')
		  AND n.nspname = ANY($1)
	`, schemas)
	if err != nil {
		return err
	}
	defer rows.Close()
	if st.CompositeTypes == nil {
		st.CompositeTypes = make(map[string]*schema.CompositeType)
	}
	for rows.Next() {
		var nsp, name, owner string
		var attrs []string
		if err := rows.Scan(&nsp, &name, &attrs, &owner); err != nil {
			return err
		}
		ct := &schema.CompositeType{Schema: strings.ToLower(nsp), Name: strings.ToLower(name), Owner: owner}
		for _, a := range attrs {
			// "name type" — split at first space.
			i := strings.IndexByte(a, ' ')
			if i <= 0 {
				continue
			}
			ct.Attributes = append(ct.Attributes, schema.CompositeAttribute{
				Name: strings.ToLower(strings.Trim(a[:i], `"`)),
				Type: strings.TrimSpace(a[i+1:]),
			})
		}
		st.CompositeTypes[ct.Schema+"."+ct.Name] = ct
	}
	return rows.Err()
}

func loadRangeTypes(ctx context.Context, pool *pgxpool.Pool, st *schema.SchemaState, schemas []string) error {
	rows, err := pool.Query(ctx, `
		SELECT n.nspname, t.typname,
		       format_type(r.rngsubtype, NULL) AS subtype,
		       pg_get_userbyid(t.typowner)     AS owner
		FROM pg_range r
		JOIN pg_type t ON t.oid = r.rngtypid
		JOIN pg_namespace n ON n.oid = t.typnamespace
		WHERE n.nspname = ANY($1)
	`, schemas)
	if err != nil {
		return err
	}
	defer rows.Close()
	if st.RangeTypes == nil {
		st.RangeTypes = make(map[string]*schema.RangeType)
	}
	for rows.Next() {
		var nsp, name, subtype, owner string
		if err := rows.Scan(&nsp, &name, &subtype, &owner); err != nil {
			return err
		}
		rt := &schema.RangeType{
			Schema:  strings.ToLower(nsp),
			Name:    strings.ToLower(name),
			Subtype: subtype,
			Owner:   owner,
		}
		st.RangeTypes[rt.Schema+"."+rt.Name] = rt
	}
	return rows.Err()
}

func loadForeignServers(ctx context.Context, pool *pgxpool.Pool, st *schema.SchemaState) error {
	rows, err := pool.Query(ctx, `
		SELECT s.srvname, COALESCE(s.srvtype, ''), COALESCE(s.srvversion, ''),
		       w.fdwname,
		       COALESCE(s.srvoptions::text[], ARRAY[]::text[]) AS opts,
		       pg_get_userbyid(s.srvowner) AS owner
		FROM pg_foreign_server s
		JOIN pg_foreign_data_wrapper w ON w.oid = s.srvfdw
	`)
	if err != nil {
		return err
	}
	defer rows.Close()
	if st.ForeignServers == nil {
		st.ForeignServers = make(map[string]*schema.ForeignServer)
	}
	for rows.Next() {
		var name, srvType, srvVer, fdw, owner string
		var opts []string
		if err := rows.Scan(&name, &srvType, &srvVer, &fdw, &opts, &owner); err != nil {
			return err
		}
		st.ForeignServers[strings.ToLower(name)] = &schema.ForeignServer{
			Name: strings.ToLower(name), Type: srvType, Version: srvVer,
			Wrapper: strings.ToLower(fdw), Options: opts, Owner: owner,
		}
	}
	return rows.Err()
}

func loadForeignTables(ctx context.Context, pool *pgxpool.Pool, st *schema.SchemaState, schemas []string) error {
	rows, err := pool.Query(ctx, `
		SELECT n.nspname, c.relname,
		       (SELECT srvname FROM pg_foreign_server WHERE oid = ft.ftserver) AS server,
		       COALESCE(ft.ftoptions::text[], ARRAY[]::text[]) AS opts,
		       pg_get_userbyid(c.relowner) AS owner
		FROM pg_foreign_table ft
		JOIN pg_class    c ON c.oid = ft.ftrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = ANY($1)
	`, schemas)
	if err != nil {
		return err
	}
	defer rows.Close()
	if st.ForeignTables == nil {
		st.ForeignTables = make(map[string]*schema.ForeignTable)
	}
	for rows.Next() {
		var nsp, name, server, owner string
		var opts []string
		if err := rows.Scan(&nsp, &name, &server, &opts, &owner); err != nil {
			return err
		}
		st.ForeignTables[strings.ToLower(nsp)+"."+strings.ToLower(name)] = &schema.ForeignTable{
			Schema: strings.ToLower(nsp), Name: strings.ToLower(name),
			Server: strings.ToLower(server), Options: opts, Owner: owner,
		}
	}
	return rows.Err()
}

func loadPublications(ctx context.Context, pool *pgxpool.Pool, st *schema.SchemaState) error {
	rows, err := pool.Query(ctx, `
		SELECT p.pubname, p.puballtables,
		       (CASE WHEN p.pubinsert THEN 'insert,' ELSE '' END ||
		        CASE WHEN p.pubupdate THEN 'update,' ELSE '' END ||
		        CASE WHEN p.pubdelete THEN 'delete,' ELSE '' END ||
		        CASE WHEN p.pubtruncate THEN 'truncate,' ELSE '' END) AS publish
		FROM pg_publication p
	`)
	if err != nil {
		return err
	}
	defer rows.Close()
	if st.Publications == nil {
		st.Publications = make(map[string]*schema.Publication)
	}
	for rows.Next() {
		var name, publish string
		var allTables bool
		if err := rows.Scan(&name, &allTables, &publish); err != nil {
			return err
		}
		st.Publications[strings.ToLower(name)] = &schema.Publication{
			Name: strings.ToLower(name), AllTables: allTables,
			Publish: strings.TrimRight(publish, ","),
		}
	}
	return rows.Err()
}

func loadSubscriptions(ctx context.Context, pool *pgxpool.Pool, st *schema.SchemaState) error {
	rows, err := pool.Query(ctx, `
		SELECT s.subname, s.subenabled,
		       array_to_string(s.subpublications, ',') AS pubs
		FROM pg_subscription s
	`)
	if err != nil {
		return err
	}
	defer rows.Close()
	if st.Subscriptions == nil {
		st.Subscriptions = make(map[string]*schema.Subscription)
	}
	for rows.Next() {
		var name, pubs string
		var enabled bool
		if err := rows.Scan(&name, &enabled, &pubs); err != nil {
			return err
		}
		var pubList []string
		if pubs != "" {
			pubList = strings.Split(pubs, ",")
		}
		st.Subscriptions[strings.ToLower(name)] = &schema.Subscription{
			Name: strings.ToLower(name), Enabled: enabled, Publications: pubList,
		}
	}
	return rows.Err()
}
