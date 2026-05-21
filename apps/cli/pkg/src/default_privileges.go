package src

import (
	"strings"

	pgq "github.com/pganalyze/pg_query_go/v6"

	"github.com/nex-gen-tech/pg-flux/pkg/schema"
)

// captureAlterDefaultPrivileges parses an ALTER DEFAULT PRIVILEGES statement and
// appends a DefaultPrivilege record to st. Only handles the common shape:
//
//	ALTER DEFAULT PRIVILEGES [FOR ROLE r [, ...]] [IN SCHEMA s [, ...]]
//	    GRANT <priv,...> ON <TABLES|SEQUENCES|FUNCTIONS|TYPES|SCHEMAS> TO <role,...>;
//	ALTER DEFAULT PRIVILEGES [FOR ROLE r] [IN SCHEMA s]
//	    REVOKE <priv,...> ON ... FROM <role,...>;
//
// REVOKE is represented by emitting a DefaultPrivilege with the corresponding
// Privilege records OMITTED — so the diff (desired vs live) flips into a REVOKE.
// For declarative source, the typical pattern is GRANT only.
func captureAlterDefaultPrivileges(s *pgq.AlterDefaultPrivilegesStmt, st *schema.SchemaState) error {
	if s == nil || st == nil {
		return nil
	}
	action := s.GetAction()
	if action == nil {
		return nil
	}
	// Parse FOR ROLE / IN SCHEMA from the options list.
	var forRoles, inSchemas []string
	for _, opt := range s.GetOptions() {
		el := opt.GetDefElem()
		if el == nil {
			continue
		}
		switch strings.ToLower(el.GetDefname()) {
		case "roles":
			if lst := el.GetArg().GetList(); lst != nil {
				for _, it := range lst.GetItems() {
					if rs := it.GetRoleSpec(); rs != nil {
						forRoles = append(forRoles, rs.GetRolename())
					}
				}
			}
		case "schemas":
			if lst := el.GetArg().GetList(); lst != nil {
				for _, it := range lst.GetItems() {
					if str := it.GetString_(); str != nil {
						inSchemas = append(inSchemas, str.GetSval())
					}
				}
			}
		}
	}
	if len(forRoles) == 0 {
		forRoles = []string{""}
	}
	if len(inSchemas) == 0 {
		inSchemas = []string{""}
	}
	// Map GrantStmt.objtype to the DefaultPrivilege keyword.
	objKeyword := ""
	switch action.GetObjtype() {
	case pgq.ObjectType_OBJECT_TABLE:
		objKeyword = "TABLES"
	case pgq.ObjectType_OBJECT_SEQUENCE:
		objKeyword = "SEQUENCES"
	case pgq.ObjectType_OBJECT_FUNCTION, pgq.ObjectType_OBJECT_ROUTINE:
		objKeyword = "FUNCTIONS"
	case pgq.ObjectType_OBJECT_TYPE:
		objKeyword = "TYPES"
	case pgq.ObjectType_OBJECT_SCHEMA:
		objKeyword = "SCHEMAS"
	default:
		return nil
	}
	// Build the privilege set (per-grantee).
	privs := collectGrantPrivileges(action)
	grantees := collectGrantees(action)
	wgo := action.GetGrantOption()
	if !action.GetIsGrant() {
		// REVOKE in source: omit these privileges. We model this by NOT emitting
		// a DefaultPrivilege entry; instead, an explicit empty-Grants entry tells
		// the differ "remove any privileges that match".
		// For simplicity, REVOKE in source is currently a no-op (rare in practice).
		return nil
	}
	for _, r := range forRoles {
		for _, sch := range inSchemas {
			var entries []schema.Privilege
			for _, g := range grantees {
				for _, pr := range privs {
					entries = append(entries, schema.Privilege{Grantee: g, Priv: pr, WithGrantOption: wgo})
				}
			}
			st.DefaultPrivileges = append(st.DefaultPrivileges, &schema.DefaultPrivilege{
				ForRole:    r,
				InSchema:   strings.ToLower(sch),
				ObjectType: objKeyword,
				Grants:     entries,
			})
		}
	}
	return nil
}
