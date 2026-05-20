package inspector

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nexg/pg-flux/pkg/schema"
)

// loadDefaultPrivileges reads pg_default_acl (per role × schema × object-type) and
// populates SchemaState.DefaultPrivileges. Filtered by schemas: we report only
// (role, schema) pairs where the schema is in the inspection set, plus the
// cluster-wide entries (schema is NULL → reported as "").
func loadDefaultPrivileges(ctx context.Context, pool *pgxpool.Pool, st *schema.SchemaState, schemas []string) error {
	if pool == nil || st == nil {
		return nil
	}
	rows, err := pool.Query(ctx, `
		SELECT
			COALESCE(r.rolname, '') AS for_role,
			COALESCE(n.nspname, '') AS in_schema,
			d.defaclobjtype::text  AS objtype,
			COALESCE(d.defaclacl::text[], ARRAY[]::text[]) AS acl
		FROM pg_default_acl d
		LEFT JOIN pg_roles r     ON r.oid = d.defaclrole
		LEFT JOIN pg_namespace n ON n.oid = d.defaclnamespace
		WHERE n.nspname IS NULL OR n.nspname = ANY($1)
	`, schemas)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var forRole, inSchema, objtype string
		var acl []string
		if err := rows.Scan(&forRole, &inSchema, &objtype, &acl); err != nil {
			return err
		}
		kw, ok := schema.DefaclObjTypeCodeToKeyword[objtype]
		if !ok {
			continue
		}
		st.DefaultPrivileges = append(st.DefaultPrivileges, &schema.DefaultPrivilege{
			ForRole:    forRole,
			InSchema:   inSchema,
			ObjectType: kw,
			Grants:     schema.ParseACL(acl),
		})
	}
	return rows.Err()
}
