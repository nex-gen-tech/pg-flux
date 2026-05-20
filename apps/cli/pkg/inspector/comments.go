package inspector

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nexg/pg-flux/pkg/schema"
)

// loadComments reads pg_description and populates Comment fields on each object kind
// already present in st. Designed to run AFTER all object maps have been loaded — it
// doesn't create new objects, only annotates existing ones.
func loadComments(ctx context.Context, pool *pgxpool.Pool, st *schema.SchemaState, schemas []string) error {
	if pool == nil || st == nil {
		return nil
	}
	if err := loadTableAndViewComments(ctx, pool, st, schemas); err != nil {
		return fmt.Errorf("load table/view comments: %w", err)
	}
	if err := loadColumnComments(ctx, pool, st, schemas); err != nil {
		return fmt.Errorf("load column comments: %w", err)
	}
	if err := loadFunctionComments(ctx, pool, st, schemas); err != nil {
		return fmt.Errorf("load function comments: %w", err)
	}
	if err := loadIndexComments(ctx, pool, st, schemas); err != nil {
		return fmt.Errorf("load index comments: %w", err)
	}
	if err := loadSequenceComments(ctx, pool, st, schemas); err != nil {
		return fmt.Errorf("load sequence comments: %w", err)
	}
	if err := loadTriggerComments(ctx, pool, st, schemas); err != nil {
		return fmt.Errorf("load trigger comments: %w", err)
	}
	if err := loadPolicyComments(ctx, pool, st, schemas); err != nil {
		return fmt.Errorf("load policy comments: %w", err)
	}
	return nil
}

func loadTableAndViewComments(ctx context.Context, pool *pgxpool.Pool, st *schema.SchemaState, schemas []string) error {
	rows, err := pool.Query(ctx, `
		SELECT n.nspname, c.relname, c.relkind::text, d.description
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		JOIN pg_description d ON d.objoid = c.oid AND d.classoid = 'pg_class'::regclass AND d.objsubid = 0
		WHERE n.nspname = ANY($1)
	`, schemas)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var nsp, name, relkind, desc string
		if err := rows.Scan(&nsp, &name, &relkind, &desc); err != nil {
			return err
		}
		key := schema.TableKey(nsp, name)
		switch relkind {
		case "r", "p":
			if t := st.Tables[key]; t != nil {
				t.Comment = desc
			}
		case "v", "m":
			if v := st.Views[key]; v != nil {
				v.Comment = desc
			}
		case "S":
			if s := st.Sequences[key]; s != nil {
				s.Comment = desc
			}
		}
	}
	return rows.Err()
}

func loadColumnComments(ctx context.Context, pool *pgxpool.Pool, st *schema.SchemaState, schemas []string) error {
	rows, err := pool.Query(ctx, `
		SELECT n.nspname, c.relname, a.attname, d.description
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		JOIN pg_attribute a ON a.attrelid = c.oid AND a.attnum > 0 AND NOT a.attisdropped
		JOIN pg_description d ON d.objoid = c.oid AND d.classoid = 'pg_class'::regclass AND d.objsubid = a.attnum
		WHERE n.nspname = ANY($1) AND c.relkind IN ('r','p','v','m','f')
	`, schemas)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var nsp, rel, col, desc string
		if err := rows.Scan(&nsp, &rel, &col, &desc); err != nil {
			return err
		}
		t := st.Tables[schema.TableKey(nsp, rel)]
		if t == nil {
			continue
		}
		for _, c := range t.Columns {
			if c != nil && c.Name == col {
				c.Comment = desc
				break
			}
		}
	}
	return rows.Err()
}

func loadFunctionComments(ctx context.Context, pool *pgxpool.Pool, st *schema.SchemaState, schemas []string) error {
	rows, err := pool.Query(ctx, `
		SELECT n.nspname, p.proname, pg_get_function_identity_arguments(p.oid), d.description
		FROM pg_proc p
		JOIN pg_namespace n ON n.oid = p.pronamespace
		JOIN pg_description d ON d.objoid = p.oid AND d.classoid = 'pg_proc'::regclass
		WHERE n.nspname = ANY($1)
	`, schemas)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var nsp, name, args, desc string
		if err := rows.Scan(&nsp, &name, &args, &desc); err != nil {
			return err
		}
		ident := fmt.Sprintf("%s.%s(%s)", nsp, name, args)
		if f := st.Functions[schema.FunctionKey(ident)]; f != nil {
			f.Comment = desc
		}
	}
	return rows.Err()
}

func loadIndexComments(ctx context.Context, pool *pgxpool.Pool, st *schema.SchemaState, schemas []string) error {
	rows, err := pool.Query(ctx, `
		SELECT n.nspname, c.relname, d.description
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		JOIN pg_description d ON d.objoid = c.oid AND d.classoid = 'pg_class'::regclass AND d.objsubid = 0
		WHERE c.relkind = 'i' AND n.nspname = ANY($1)
	`, schemas)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var nsp, name, desc string
		if err := rows.Scan(&nsp, &name, &desc); err != nil {
			return err
		}
		if ix := st.Indexes[schema.IndexKey(nsp, name)]; ix != nil {
			ix.Comment = desc
		}
	}
	return rows.Err()
}

func loadSequenceComments(ctx context.Context, pool *pgxpool.Pool, st *schema.SchemaState, schemas []string) error {
	// Sequences are pg_class.relkind='S'; loadTableAndViewComments already populates them.
	_ = pool
	_ = st
	_ = schemas
	return nil
}

func loadTriggerComments(ctx context.Context, pool *pgxpool.Pool, st *schema.SchemaState, schemas []string) error {
	rows, err := pool.Query(ctx, `
		SELECT n.nspname, c.relname, t.tgname, d.description
		FROM pg_trigger t
		JOIN pg_class c ON c.oid = t.tgrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		JOIN pg_description d ON d.objoid = t.oid AND d.classoid = 'pg_trigger'::regclass
		WHERE NOT t.tgisinternal AND n.nspname = ANY($1)
	`, schemas)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var nsp, rel, name, desc string
		if err := rows.Scan(&nsp, &rel, &name, &desc); err != nil {
			return err
		}
		if tr := st.Triggers[schema.TriggerKey(nsp, rel, name)]; tr != nil {
			tr.Comment = desc
		}
	}
	return rows.Err()
}

func loadPolicyComments(ctx context.Context, pool *pgxpool.Pool, st *schema.SchemaState, schemas []string) error {
	rows, err := pool.Query(ctx, `
		SELECT n.nspname, c.relname, p.polname, d.description
		FROM pg_policy p
		JOIN pg_class c ON c.oid = p.polrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		JOIN pg_description d ON d.objoid = p.oid AND d.classoid = 'pg_policy'::regclass
		WHERE n.nspname = ANY($1)
	`, schemas)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var nsp, rel, name, desc string
		if err := rows.Scan(&nsp, &rel, &name, &desc); err != nil {
			return err
		}
		if pl := st.Policies[schema.PolicyKey(nsp, rel, name)]; pl != nil {
			pl.Comment = desc
		}
	}
	return rows.Err()
}
