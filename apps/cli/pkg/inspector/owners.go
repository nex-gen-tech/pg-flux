package inspector

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nex-gen-tech/pg-flux/pkg/schema"
)

// loadOwners populates the Owner field on each loaded object via pg_class.relowner
// (for tables/views/MVs/sequences) and pg_proc.proowner (for functions). Owners are
// resolved to role names via pg_roles. Designed to run AFTER object maps are loaded.
func loadOwners(ctx context.Context, pool *pgxpool.Pool, st *schema.SchemaState, schemas []string) error {
	if pool == nil || st == nil {
		return nil
	}
	if err := loadRelOwners(ctx, pool, st, schemas); err != nil {
		return fmt.Errorf("load relation owners: %w", err)
	}
	if err := loadFunctionOwners(ctx, pool, st, schemas); err != nil {
		return fmt.Errorf("load function owners: %w", err)
	}
	return nil
}

func loadRelOwners(ctx context.Context, pool *pgxpool.Pool, st *schema.SchemaState, schemas []string) error {
	rows, err := pool.Query(ctx, `
		SELECT n.nspname, c.relname, c.relkind::text, r.rolname
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		JOIN pg_roles    r ON r.oid = c.relowner
		WHERE n.nspname = ANY($1) AND c.relkind IN ('r','p','v','m','S')
	`, schemas)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var nsp, name, relkind, owner string
		if err := rows.Scan(&nsp, &name, &relkind, &owner); err != nil {
			return err
		}
		key := schema.TableKey(nsp, name)
		switch relkind {
		case "r", "p":
			if t := st.Tables[key]; t != nil {
				t.Owner = owner
			}
		case "v", "m":
			if v := st.Views[key]; v != nil {
				v.Owner = owner
			}
		case "S":
			if s := st.Sequences[key]; s != nil {
				s.Owner = owner
			}
		}
	}
	return rows.Err()
}

func loadFunctionOwners(ctx context.Context, pool *pgxpool.Pool, st *schema.SchemaState, schemas []string) error {
	rows, err := pool.Query(ctx, `
		SELECT n.nspname, p.proname, pg_get_function_identity_arguments(p.oid), r.rolname
		FROM pg_proc p
		JOIN pg_namespace n ON n.oid = p.pronamespace
		JOIN pg_roles    r ON r.oid = p.proowner
		WHERE n.nspname = ANY($1)
	`, schemas)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var nsp, name, args, owner string
		if err := rows.Scan(&nsp, &name, &args, &owner); err != nil {
			return err
		}
		ident := fmt.Sprintf("%s.%s(%s)", nsp, name, args)
		if f := st.Functions[schema.FunctionKey(ident)]; f != nil {
			f.Owner = owner
		}
	}
	return rows.Err()
}
