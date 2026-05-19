package inspector

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nexg/pg-flux/pkg/schema"
)

// loadUserTypeMap returns a set of user-defined type keys ("schema.name") for enum,
// domain, and composite types visible in the given schemas.
func loadUserTypeMap(ctx context.Context, pool *pgxpool.Pool, schemas []string) (map[string]struct{}, map[string][]string, error) {
	typeSet := make(map[string]struct{})
	enumVals := make(map[string][]string)
	rows, err := pool.Query(ctx, `
		SELECT n.nspname, t.typname, t.typtype::text
		FROM pg_type t
		JOIN pg_namespace n ON n.oid = t.typnamespace
		WHERE n.nspname = ANY($1)
		  AND t.typtype IN ('e', 'd', 'c')   -- enum, domain, composite
		  AND t.typelem = 0                   -- exclude array types (_typename)
	`, schemas)
	if err != nil {
		return nil, nil, fmt.Errorf("load user types: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var ns, name, typtype string
		if err := rows.Scan(&ns, &name, &typtype); err != nil {
			return nil, nil, err
		}
		key := strings.ToLower(ns) + "." + strings.ToLower(name)
		typeSet[key] = struct{}{}
		if typtype == "e" {
			enumVals[key] = nil // mark as enum; values filled below
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	// Load enum labels in sort order.
	if len(enumVals) > 0 {
		erows, err := pool.Query(ctx, `
			SELECT n.nspname, t.typname, e.enumlabel
			FROM pg_enum e
			JOIN pg_type t ON t.oid = e.enumtypid
			JOIN pg_namespace n ON n.oid = t.typnamespace
			WHERE n.nspname = ANY($1)
			ORDER BY t.typname, e.enumsortorder
		`, schemas)
		if err != nil {
			return nil, nil, fmt.Errorf("load enum values: %w", err)
		}
		defer erows.Close()
		for erows.Next() {
			var ns, name, label string
			if err := erows.Scan(&ns, &name, &label); err != nil {
				return nil, nil, err
			}
			key := strings.ToLower(ns) + "." + strings.ToLower(name)
			enumVals[key] = append(enumVals[key], label)
		}
		if err := erows.Err(); err != nil {
			return nil, nil, err
		}
	}
	return typeSet, enumVals, nil
}

// mergeTableConstraints appends CHECK / FOREIGN KEY rows from pg_constraint into existing tables.
func mergeTableConstraints(ctx context.Context, pool *pgxpool.Pool, st *schema.SchemaState, schemas []string) error {
	rows, err := pool.Query(ctx, `
		SELECT
			tn.nspname,
			r.relname,
			lower(c.conname),
			c.contype::text,
			pg_get_constraintdef(c.oid, true) AS def,
			c.condeferrable,
			c.condeferred,
			c.confmatchtype::text
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
		var nsp, rel, cname, ctype, def, matchType string
		var deferrable, deferred bool
		if err := rows.Scan(&nsp, &rel, &cname, &ctype, &def, &deferrable, &deferred, &matchType); err != nil {
			return err
		}
		tk := schema.TableKey(nsp, rel)
		t, ok := st.Tables[tk]
		if !ok || t == nil {
			continue
		}
		switch ctype {
		case "c":
			t.Checks = append(t.Checks, &schema.TableCheck{Name: cname, DefSQL: def, Deferrable: deferrable, InitiallyDeferred: deferred})
		case "f":
			fkMatch := ""
			switch matchType {
			case "f":
				fkMatch = "FULL"
			case "p":
				fkMatch = "PARTIAL"
				// "s" / "" / " " all map to default SIMPLE — leave empty.
			}
			t.ForeignKeys = append(t.ForeignKeys, &schema.TableForeignKey{Name: cname, DefSQL: def, Deferrable: deferrable, InitiallyDeferred: deferred, MatchType: fkMatch})
		case "u":
			t.Uniques = append(t.Uniques, &schema.TableUnique{Name: cname, DefSQL: def, Deferrable: deferrable, InitiallyDeferred: deferred})
		case "x":
			t.Excludes = append(t.Excludes, &schema.TableExclusion{Name: cname, DefSQL: def, Deferrable: deferrable, InitiallyDeferred: deferred})
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
			' AS ' || format_type(s.seqtypid, NULL) ||
			' INCREMENT BY ' || s.seqincrement ||
			' MINVALUE ' || s.seqmin ||
			' MAXVALUE ' || s.seqmax ||
			' START WITH ' || s.seqstart ||
			' CACHE ' || s.seqcache ||
			CASE WHEN s.seqcycle THEN ' CYCLE' ELSE ' NO CYCLE' END AS def,
			format_type(s.seqtypid, NULL) AS as_type,
			COALESCE((
				SELECT format('%I.%I.%I', on_n.nspname, on_c.relname, a.attname)
				FROM pg_depend d
				JOIN pg_class on_c     ON on_c.oid = d.refobjid
				JOIN pg_namespace on_n ON on_n.oid = on_c.relnamespace
				JOIN pg_attribute a    ON a.attrelid = d.refobjid AND a.attnum = d.refobjsubid
				WHERE d.objid = c.oid AND d.deptype = 'a' AND d.classid = 'pg_class'::regclass
				LIMIT 1
			), '') AS owned_by
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		JOIN pg_sequence s ON s.seqrelid = c.oid
		WHERE c.relkind = 'S' AND n.nspname = ANY($1)
		-- Exclude sequences that are owned by a column (implicit serial/bigserial sequences).
		AND NOT EXISTS (
			SELECT 1 FROM pg_depend d
			JOIN pg_attribute a ON a.attrelid = d.refobjid AND a.attnum = d.refobjsubid
			WHERE d.objid = c.oid AND d.deptype = 'a' AND d.classid = 'pg_class'::regclass
		)
		ORDER BY n.nspname, c.relname
	`, schemas)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]*schema.Sequence)
	for rows.Next() {
		var nsp, sname, def, asType, ownedBy string
		if err := rows.Scan(&nsp, &sname, &def, &asType, &ownedBy); err != nil {
			return nil, err
		}
		nsp = strings.ToLower(nsp)
		sname = strings.ToLower(sname)
		k := schema.SeqKey(nsp, sname)
		out[k] = &schema.Sequence{Schema: nsp, Name: sname, DefSQL: def, AsType: asType, OwnedBy: ownedBy}
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

// loadDomainMap loads user-defined domains and their CHECK constraints from the catalog.
// Returns a map keyed by "schema.name".
func loadDomainMap(ctx context.Context, pool *pgxpool.Pool, schemas []string) (map[string]*schema.Domain, error) {
	rows, err := pool.Query(ctx, `
		SELECT
			n.nspname,
			t.typname,
			pg_catalog.format_type(t.typbasetype, t.typtypmod) AS base_type,
			COALESCE(c.conname, '') AS conname,
			COALESCE(pg_get_constraintdef(c.oid, true), '') AS condef
		FROM pg_catalog.pg_type t
		JOIN pg_catalog.pg_namespace n ON n.oid = t.typnamespace
		LEFT JOIN pg_catalog.pg_constraint c
			ON c.contypid = t.oid AND c.contype = 'c'
		WHERE t.typtype = 'd'
		  AND n.nspname = ANY($1)
		ORDER BY n.nspname, t.typname, c.conname
	`, schemas)
	if err != nil {
		return nil, fmt.Errorf("load domains: %w", err)
	}
	defer rows.Close()

	out := make(map[string]*schema.Domain)
	for rows.Next() {
		var ns, name, baseType, conname, condef string
		if err := rows.Scan(&ns, &name, &baseType, &conname, &condef); err != nil {
			return nil, err
		}
		key := strings.ToLower(ns) + "." + strings.ToLower(name)
		dom, ok := out[key]
		if !ok {
			dom = &schema.Domain{Schema: ns, Name: name, BaseType: baseType}
			out[key] = dom
		}
		// pg_get_constraintdef returns "CHECK (...)" — strip the "CHECK (" and ")" wrapper.
		if condef != "" {
			expr := stripCheckWrapper(condef)
			dom.Constraints = append(dom.Constraints, schema.DomainConstraint{Name: conname, Expr: expr})
		}
	}
	return out, rows.Err()
}

// stripCheckWrapper removes the "CHECK (" prefix and trailing ")" from a constraint definition.
func stripCheckWrapper(s string) string {
	upper := strings.ToUpper(strings.TrimSpace(s))
	if strings.HasPrefix(upper, "CHECK (") && strings.HasSuffix(s, ")") {
		inner := s[len("CHECK (") : len(s)-1]
		return strings.TrimSpace(inner)
	}
	return s
}
