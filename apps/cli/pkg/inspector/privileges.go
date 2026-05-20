package inspector

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nexg/pg-flux/pkg/schema"
)

// loadPrivileges annotates every loaded relation / function / sequence with the
// parsed ACL entries from pg_class.relacl / pg_proc.proacl. Designed to run
// AFTER object maps are loaded.
func loadPrivileges(ctx context.Context, pool *pgxpool.Pool, st *schema.SchemaState, schemas []string) error {
	if pool == nil || st == nil {
		return nil
	}
	if err := loadRelPrivileges(ctx, pool, st, schemas); err != nil {
		return fmt.Errorf("load rel privileges: %w", err)
	}
	if err := loadFunctionPrivileges(ctx, pool, st, schemas); err != nil {
		return fmt.Errorf("load function privileges: %w", err)
	}
	return nil
}

func loadRelPrivileges(ctx context.Context, pool *pgxpool.Pool, st *schema.SchemaState, schemas []string) error {
	rows, err := pool.Query(ctx, `
		SELECT n.nspname, c.relname, c.relkind::text,
		       COALESCE(c.relacl::text[], ARRAY[]::text[]) AS acl
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = ANY($1) AND c.relkind IN ('r','p','v','m','S')
	`, schemas)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var nsp, name, relkind string
		var acl []string
		if err := rows.Scan(&nsp, &name, &relkind, &acl); err != nil {
			return err
		}
		privs := schema.ParseACL(acl)
		if len(privs) == 0 {
			continue
		}
		key := schema.TableKey(nsp, name)
		switch relkind {
		case "r", "p":
			if t := st.Tables[key]; t != nil {
				t.Privileges = privs
			}
		case "v", "m":
			if v := st.Views[key]; v != nil {
				v.Privileges = privs
			}
		case "S":
			if s := st.Sequences[key]; s != nil {
				s.Privileges = privs
			}
		}
	}
	return rows.Err()
}

func loadFunctionPrivileges(ctx context.Context, pool *pgxpool.Pool, st *schema.SchemaState, schemas []string) error {
	rows, err := pool.Query(ctx, `
		SELECT n.nspname, p.proname, pg_get_function_identity_arguments(p.oid),
		       COALESCE(p.proacl::text[], ARRAY[]::text[]) AS acl
		FROM pg_proc p
		JOIN pg_namespace n ON n.oid = p.pronamespace
		WHERE n.nspname = ANY($1)
	`, schemas)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var nsp, name, args string
		var acl []string
		if err := rows.Scan(&nsp, &name, &args, &acl); err != nil {
			return err
		}
		privs := schema.ParseACL(acl)
		if len(privs) == 0 {
			continue
		}
		ident := fmt.Sprintf("%s.%s(%s)", nsp, name, args)
		if f := st.Functions[schema.FunctionKey(ident)]; f != nil {
			f.Privileges = privs
		}
	}
	return rows.Err()
}
