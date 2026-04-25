package inspector

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nexg/pg-flux/pkg/schema"
)

// mergeTableConstraints appends CHECK / FOREIGN KEY rows from pg_constraint into existing tables.
func mergeTableConstraints(ctx context.Context, pool *pgxpool.Pool, st *schema.SchemaState, schemas []string) error {
	rows, err := pool.Query(ctx, `
		SELECT
			tn.nspname,
			r.relname,
			lower(c.conname),
			c.contype::text,
			pg_get_constraintdef(c.oid, true) AS def
		FROM pg_constraint c
		JOIN pg_class r ON r.oid = c.conrelid
		JOIN pg_namespace tn ON tn.oid = r.relnamespace
		WHERE tn.nspname = ANY($1) AND c.contype IN ('c', 'f', 'u', 'x')
	`, schemas)
	if err != nil {
		return fmt.Errorf("pg_constraint: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var nsp, rel, cname, ctype, def string
		if err := rows.Scan(&nsp, &rel, &cname, &ctype, &def); err != nil {
			return err
		}
		tk := schema.TableKey(nsp, rel)
		t, ok := st.Tables[tk]
		if !ok || t == nil {
			continue
		}
		switch ctype {
		case "c":
			t.Checks = append(t.Checks, &schema.TableCheck{Name: cname, DefSQL: def})
		case "f":
			t.ForeignKeys = append(t.ForeignKeys, &schema.TableForeignKey{Name: cname, DefSQL: def})
		case "u":
			t.Uniques = append(t.Uniques, &schema.TableUnique{Name: cname, DefSQL: def})
		case "x":
			t.Excludes = append(t.Excludes, &schema.TableExclusion{Name: cname, DefSQL: def})
		}
	}
	return rows.Err()
}

func loadViewsMap(ctx context.Context, pool *pgxpool.Pool, schemas []string) (map[string]*schema.View, error) {
	rows, err := pool.Query(ctx, `
		SELECT
			n.nspname,
			c.relname,
			false,
			'CREATE VIEW ' || format('%I.%I', n.nspname, c.relname) || ' AS ' || pg_get_viewdef(c.oid, true)
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relkind = 'v' AND n.nspname = ANY($1)
		UNION ALL
		SELECT
			n.nspname,
			c.relname,
			true,
			'CREATE MATERIALIZED VIEW ' || format('%I.%I', n.nspname, c.relname) || ' AS ' || pg_get_viewdef(c.oid, true)
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relkind = 'm' AND n.nspname = ANY($1)
		ORDER BY 1,2
	`, schemas)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]*schema.View)
	for rows.Next() {
		var nsp, vname, def string
		var mat bool
		if err := rows.Scan(&nsp, &vname, &mat, &def); err != nil {
			return nil, err
		}
		nsp = strings.ToLower(nsp)
		vname = strings.ToLower(vname)
		k := schema.ViewKey(nsp, vname)
		out[k] = &schema.View{Schema: nsp, Name: vname, DefSQL: def, Materialized: mat}
	}
	return out, rows.Err()
}

func loadSequenceMap(ctx context.Context, pool *pgxpool.Pool, schemas []string) (map[string]*schema.Sequence, error) {
	rows, err := pool.Query(ctx, `
		SELECT
			n.nspname,
			c.relname,
			'CREATE SEQUENCE ' || format('%I.%I', n.nspname, c.relname) ||
			' INCREMENT BY ' || s.seqincrement ||
			' MINVALUE ' || s.seqmin ||
			' MAXVALUE ' || s.seqmax ||
			' START WITH ' || s.seqstart ||
			' CACHE ' || s.seqcache ||
			CASE WHEN s.seqcycle THEN ' CYCLE' ELSE ' NO CYCLE' END AS def
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		JOIN pg_sequence s ON s.seqrelid = c.oid
		WHERE c.relkind = 'S' AND n.nspname = ANY($1)
		ORDER BY n.nspname, c.relname
	`, schemas)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]*schema.Sequence)
	for rows.Next() {
		var nsp, sname, def string
		if err := rows.Scan(&nsp, &sname, &def); err != nil {
			return nil, err
		}
		nsp = strings.ToLower(nsp)
		sname = strings.ToLower(sname)
		k := schema.SeqKey(nsp, sname)
		out[k] = &schema.Sequence{Schema: nsp, Name: sname, DefSQL: def}
	}
	return out, rows.Err()
}

func loadTriggerMap(ctx context.Context, pool *pgxpool.Pool, schemas []string) (map[string]*schema.Trigger, error) {
	rows, err := pool.Query(ctx, `
		SELECT
			tn.nspname,
			trel.relname,
			lower(t.tgname),
			pg_get_triggerdef(t.oid, true) AS def
		FROM pg_trigger t
		JOIN pg_class trel ON trel.oid = t.tgrelid
		JOIN pg_namespace tn ON tn.oid = trel.relnamespace
		WHERE tn.nspname = ANY($1) AND NOT t.tgisinternal
	`, schemas)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]*schema.Trigger)
	for rows.Next() {
		var nsp, rel, tg, def string
		if err := rows.Scan(&nsp, &rel, &tg, &def); err != nil {
			return nil, err
		}
		nsp = strings.ToLower(nsp)
		rel = strings.ToLower(rel)
		tg = strings.ToLower(tg)
		k := schema.TriggerKey(nsp, rel, tg)
		out[k] = &schema.Trigger{Schema: nsp, Table: rel, Name: tg, DefSQL: def}
	}
	return out, rows.Err()
}
