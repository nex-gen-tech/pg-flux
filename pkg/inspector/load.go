package inspector

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"

	"github.com/nexg/pg-flux/pkg/pgver"
	"github.com/nexg/pg-flux/pkg/schema"
)

// Options configure catalog inspection.
type Options struct {
	Schemas []string // default public
	// PGVersion is optional: when set, inspector queries that vary by version
	// (e.g. NULLS NOT DISTINCT on PG15+) gate on it. When zero, Inspect detects
	// the version itself via pgver.Detect.
	PGVersion pgver.Version
}

// Inspect loads user tables, indexes, functions, and RLS policies from system catalogs in parallel.
func Inspect(ctx context.Context, pool *pgxpool.Pool, opt Options) (*schema.SchemaState, error) {
	if pool == nil {
		return nil, fmt.Errorf("inspector: nil pool")
	}
	schemas := opt.Schemas
	if len(schemas) == 0 {
		schemas = []string{"public"}
	}
	for i, s := range schemas {
		schemas[i] = strings.ToLower(strings.TrimSpace(s))
	}
	// Detect server version if the caller didn't supply one. Fails loud below MinSupportedMajor.
	pgv := opt.PGVersion
	if pgv == (pgver.Version{}) {
		detected, err := pgver.Detect(ctx, pool)
		if err != nil {
			return nil, err
		}
		pgv = detected
	}
	st := &schema.SchemaState{
		PGVersion:  pgv,
		Tables:     make(map[string]*schema.Table),
		Indexes:    make(map[string]*schema.Index),
		Functions:  make(map[string]*schema.Function),
		Policies:   make(map[string]*schema.Policy),
		Views:      make(map[string]*schema.View),
		Sequences:  make(map[string]*schema.Sequence),
		Triggers:   make(map[string]*schema.Trigger),
		Extensions: make(map[string]*schema.Extension),
	}
	g, gctx := errgroup.WithContext(ctx)
	var mu sync.Mutex
	g.Go(func() error {
		tm, e := loadTablesMap(gctx, pool, schemas)
		if e != nil {
			return e
		}
		mu.Lock()
		for k, t := range tm {
			st.Tables[k] = t
		}
		mu.Unlock()
		return nil
	})
	g.Go(func() error {
		im, e := loadIndexMap(gctx, pool, schemas)
		if e != nil {
			return e
		}
		mu.Lock()
		for k, v := range im {
			st.Indexes[k] = v
		}
		mu.Unlock()
		return nil
	})
	g.Go(func() error {
		fm, e := loadFunctionMap(gctx, pool, schemas)
		if e != nil {
			return e
		}
		mu.Lock()
		for k, v := range fm {
			st.Functions[k] = v
		}
		mu.Unlock()
		return nil
	})
	g.Go(func() error {
		pm, e := loadPolicyMap(gctx, pool, schemas)
		if e != nil {
			return e
		}
		mu.Lock()
		for k, v := range pm {
			st.Policies[k] = v
		}
		mu.Unlock()
		return nil
	})
	if err := g.Wait(); err != nil {
		return nil, err
	}
	if err := mergeTableConstraints(ctx, pool, st, schemas); err != nil {
		return nil, err
	}
	g2, g2ctx := errgroup.WithContext(ctx)
	g2.Go(func() error {
		vm, e := loadViewsMap(g2ctx, pool, schemas)
		if e != nil {
			return e
		}
		for k, v := range vm {
			st.Views[k] = v
		}
		return nil
	})
	g2.Go(func() error {
		sm, e := loadSequenceMap(g2ctx, pool, schemas)
		if e != nil {
			return e
		}
		for k, v := range sm {
			st.Sequences[k] = v
		}
		return nil
	})
	g2.Go(func() error {
		tm, e := loadTriggerMap(g2ctx, pool, schemas)
		if e != nil {
			return e
		}
		for k, v := range tm {
			st.Triggers[k] = v
		}
		return nil
	})
	if err := g2.Wait(); err != nil {
		return nil, err
	}
	em, err := loadExtensionMap(ctx, pool)
	if err != nil {
		return nil, err
	}
	st.Extensions = em
	utm, enumVals, err := loadUserTypeMap(ctx, pool, schemas)
	if err != nil {
		return nil, err
	}
	st.UserTypes = utm
	st.EnumValues = enumVals
	// Load domain definitions with CHECK constraints for ALTER DOMAIN diffing.
	dm, err := loadDomainMap(ctx, pool, schemas)
	if err != nil {
		return nil, err
	}
	st.Domains = dm
	// Load partition children so diffExtraDDL can skip them if they already exist.
	pc, err := loadPartitionChildren(ctx, pool, schemas)
	if err != nil {
		return nil, err
	}
	st.PartitionChildren = pc
	// Annotate every loaded object with its pg_description comment if any.
	if err := loadComments(ctx, pool, st, schemas); err != nil {
		return nil, err
	}
	// Annotate every loaded object with its pg_class.relowner / pg_proc.proowner role.
	if err := loadOwners(ctx, pool, st, schemas); err != nil {
		return nil, err
	}
	// Annotate every loaded relation/function with its ACL (pg_class.relacl / pg_proc.proacl).
	if err := loadPrivileges(ctx, pool, st, schemas); err != nil {
		return nil, err
	}
	return st, nil
}

func loadTablesMap(ctx context.Context, pool *pgxpool.Pool, schemas []string) (map[string]*schema.Table, error) {
	out := make(map[string]*schema.Table)
	rows, err := pool.Query(ctx, `
		SELECT c.oid, n.nspname, c.relname, c.relrowsecurity, c.relforcerowsecurity,
		       c.relkind::text,
		       c.relpersistence::text,
		       COALESCE(c.reloptions, ARRAY[]::text[]) AS reloptions
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relkind IN ('r', 'p') AND n.nspname = ANY($1)
		  AND c.relispartition = false
		ORDER BY n.nspname, c.relname
	`, schemas)
	if err != nil {
		return nil, fmt.Errorf("query tables: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var oid uint32
		var nsp, rel string
		var rls, rlsf bool
		var relkind, persistence string
		var reloptions []string
		if err := rows.Scan(&oid, &nsp, &rel, &rls, &rlsf, &relkind, &persistence, &reloptions); err != nil {
			return nil, err
		}
		key := schema.TableKey(nsp, rel)
		tab := &schema.Table{
			Schema:     nsp,
			Name:       strings.ToLower(rel),
			RLSEnabled: rls,
			RLSForced:  rlsf,
			Unlogged:   persistence == "u",
			ReLOptions: reloptions,
		}
		// For partitioned tables (relkind='p'), load the partition key definition.
		if relkind == "p" {
			var partKey string
			if err := pool.QueryRow(ctx, `SELECT pg_get_partkeydef($1)`, oid).Scan(&partKey); err == nil {
				tab.PartitionBy = partKey
			}
		}
		if err := fillColumns(ctx, pool, oid, tab); err != nil {
			return nil, err
		}
		if err := fillPK(ctx, pool, oid, tab); err != nil {
			return nil, err
		}
		out[key] = tab
	}
	return out, rows.Err()
}

// loadPartitionChildren returns a set of "schema.table" keys for all partition child tables
// in the given schemas. Used by diffExtraDDL to skip emitting CREATE TABLE IF NOT EXISTS
// for partition children that already exist in the live DB.
func loadPartitionChildren(ctx context.Context, pool *pgxpool.Pool, schemas []string) (map[string]bool, error) {
	rows, err := pool.Query(ctx, `
		SELECT n.nspname, c.relname
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relispartition = true AND n.nspname = ANY($1)
	`, schemas)
	if err != nil {
		return nil, fmt.Errorf("query partition children: %w", err)
	}
	defer rows.Close()
	out := make(map[string]bool)
	for rows.Next() {
		var nsp, rel string
		if err := rows.Scan(&nsp, &rel); err != nil {
			return nil, err
		}
		out[schema.TableKey(nsp, strings.ToLower(rel))] = true
	}
	return out, rows.Err()
}

func loadIndexMap(ctx context.Context, pool *pgxpool.Pool, schemas []string) (map[string]*schema.Index, error) {
	rows, err := pool.Query(ctx, `
		SELECT
			n.nspname, ic.relname, pg_get_indexdef(i.indexrelid, 0, true),
			nt.nspname, t.relname, i.indisunique
		FROM pg_index i
		JOIN pg_class ic ON ic.oid = i.indexrelid
		JOIN pg_namespace n ON n.oid = ic.relnamespace
		JOIN pg_class t ON t.oid = i.indrelid
		JOIN pg_namespace nt ON nt.oid = t.relnamespace
		WHERE n.nspname = ANY($1) AND NOT i.indisprimary
		  AND t.relispartition = false
		  AND NOT EXISTS (
			SELECT 1 FROM pg_constraint c
			WHERE c.conrelid = t.oid
				AND c.conname = ic.relname
				AND c.contype IN ('u', 'x')
		)
	`, schemas)
	if err != nil {
		return nil, fmt.Errorf("index query: %w", err)
	}
	defer rows.Close()
	out := make(map[string]*schema.Index)
	for rows.Next() {
		var nsp, iname, def, tSchema, tName string
		var _uniq bool
		if err := rows.Scan(&nsp, &iname, &def, &tSchema, &tName, &_uniq); err != nil {
			return nil, err
		}
		iname = strings.ToLower(iname)
		key := schema.IndexKey(nsp, iname)
		out[key] = &schema.Index{
			Schema:      nsp,
			Name:        iname,
			TableSchema: tSchema,
			Table:       strings.ToLower(tName),
			CreateSQL:   def,
		}
	}
	return out, rows.Err()
}

func loadFunctionMap(ctx context.Context, pool *pgxpool.Pool, schemas []string) (map[string]*schema.Function, error) {
	// Only plain functions (not agg/window) in v1
	rows, err := pool.Query(ctx, `
		SELECT
			n.nspname, p.proname, l.lanname, p.prokind::text, pg_get_functiondef(p.oid),
			COALESCE((
				SELECT string_agg(format_type(x, NULL), ', ' ORDER BY ord)
				FROM unnest(p.proargtypes::oid[]) WITH ORDINALITY AS u(x, ord)
			), '') AS argtypes,
			p.provolatile::text,
			p.prosecdef,
			p.proparallel::text,
			p.proleakproof,
			p.procost,
			p.prorows,
			COALESCE(p.proconfig, ARRAY[]::text[]) AS proconfig
		FROM pg_proc p
		JOIN pg_namespace n ON n.oid = p.pronamespace
		JOIN pg_language l ON l.oid = p.prolang
		WHERE n.nspname = ANY($1) AND p.prokind IN ('f', 'a', 'w', 'p')
		  AND NOT EXISTS (
		    SELECT 1 FROM pg_depend d
		    WHERE d.objid = p.oid AND d.deptype = 'e'
		  )
	`, schemas)
	if err != nil {
		return nil, fmt.Errorf("function query: %w", err)
	}
	defer rows.Close()
	out := make(map[string]*schema.Function)
	for rows.Next() {
		var nsp, name, lang, pkind, def, argtypes string
		var provolatile, proparallel string
		var prosecdef, proleakproof bool
		var procost, prorows float64
		var proconfig []string
		if err := rows.Scan(&nsp, &name, &lang, &pkind, &def, &argtypes,
			&provolatile, &prosecdef, &proparallel, &proleakproof, &procost, &prorows, &proconfig); err != nil {
			return nil, err
		}
		identity := schema.BuildFunctionIdentity(nsp, name, argtypes)
		fk := schema.FunctionKey(identity)
		fn := &schema.Function{
			Schema: nsp, Name: name, Language: strings.ToLower(lang), Kind: pkind,
			DefSQL: def, Identity: identity,
			LeakProof: proleakproof,
			Cost:      procost,
			Rows:      prorows,
			Config:    proconfig,
		}
		switch provolatile {
		case "i":
			fn.Volatility = "IMMUTABLE"
		case "s":
			fn.Volatility = "STABLE"
		case "v":
			fn.Volatility = "VOLATILE"
		}
		if prosecdef {
			fn.Security = "DEFINER"
		} else {
			fn.Security = "INVOKER"
		}
		switch proparallel {
		case "s":
			fn.Parallel = "SAFE"
		case "r":
			fn.Parallel = "RESTRICTED"
		case "u":
			fn.Parallel = "UNSAFE"
		}
		out[fk] = fn
	}
	return out, rows.Err()
}

func loadPolicyMap(ctx context.Context, pool *pgxpool.Pool, schemas []string) (map[string]*schema.Policy, error) {
	rows, err := pool.Query(ctx, `
		SELECT
			tn.nspname,
			t.relname,
			p.polname,
			CASE p.polcmd
				WHEN 'r' THEN 'select'
				WHEN 'a' THEN 'insert'
				WHEN 'w' THEN 'update'
				WHEN 'd' THEN 'delete'
				ELSE '*'
			END,
			p.polpermissive,
			pg_get_expr(p.polqual, p.polrelid) AS us,
			pg_get_expr(p.polwithcheck, p.polrelid) AS wchk,
			COALESCE((
				SELECT string_agg(quote_ident(rol.rolname), ', ' ORDER BY rol.rolname)
				FROM unnest(p.polroles) AS pr(oid)
				JOIN pg_authid rol ON rol.oid = pr.oid
			), '') AS roles
		FROM pg_policy p
		JOIN pg_class t ON t.oid = p.polrelid
		JOIN pg_namespace tn ON tn.oid = t.relnamespace
		WHERE tn.nspname = ANY($1)
	`, schemas)
	if err != nil {
		return nil, fmt.Errorf("policy query: %w", err)
	}
	defer rows.Close()
	out := make(map[string]*schema.Policy)
	for rows.Next() {
		var sn, tname, polname, cmd, roles string
		var us, wchk *string // nullable: may be NULL when expression is not set
		var perm bool
		if err := rows.Scan(&sn, &tname, &polname, &cmd, &perm, &us, &wchk, &roles); err != nil {
			return nil, err
		}
		k := schema.PolicyKey(sn, tname, polname)
		var rlist []string
		if strings.TrimSpace(roles) != "" {
			for _, p := range strings.Split(roles, ",") {
				if t := strings.ToLower(strings.TrimSpace(strings.ReplaceAll(p, `"`, ""))); t != "" {
					rlist = append(rlist, t)
				}
			}
		}
		usingSQL := ""
		if us != nil {
			usingSQL = *us
		}
		withCheck := ""
		if wchk != nil {
			withCheck = *wchk
		}
		out[k] = &schema.Policy{Schema: sn, Table: strings.ToLower(tname), Name: polname, Cmd: cmd, Permissive: perm, UsingSQL: usingSQL, WithCheck: withCheck, Roles: rlist}
	}
	return out, rows.Err()
}
