package src

import (
	"strings"

	pgq "github.com/pganalyze/pg_query_go/v6"

	"github.com/nexg/pg-flux/pkg/schema"
)

// isStructuredGrantTarget reports whether the GrantStmt object kind is one we
// track on the schema model (Privileges field). Other kinds are passed through
// as MiscObjects so the source GRANT survives but isn't diffed.
func isStructuredGrantTarget(t pgq.ObjectType) bool {
	switch t {
	case pgq.ObjectType_OBJECT_TABLE,
		pgq.ObjectType_OBJECT_VIEW,
		pgq.ObjectType_OBJECT_MATVIEW,
		pgq.ObjectType_OBJECT_SEQUENCE,
		pgq.ObjectType_OBJECT_FUNCTION,
		pgq.ObjectType_OBJECT_PROCEDURE,
		pgq.ObjectType_OBJECT_ROUTINE:
		return true
	}
	return false
}

// captureGrantStmt walks a GRANT or REVOKE node and attaches the resulting
// Privilege records to the matching object in st. Falls back to MiscObject
// pass-through for object kinds pg-flux doesn't yet track (DATABASE, LANGUAGE,
// FOREIGN DATA WRAPPER, TABLESPACE, etc.).
//
// Supported object kinds: TABLE, VIEW, MATERIALIZED VIEW, SEQUENCE, FUNCTION,
// PROCEDURE, ROUTINE, SCHEMA. ALL TABLES/SEQUENCES/FUNCTIONS IN SCHEMA is
// expanded to per-object grants (each matching object gets the privilege).
//
// is_grant=false means this is a REVOKE; the privileges are subtracted from
// the live state rather than added (the differ later compares desired vs live
// sets, so REVOKE statements in source files have the same effect as omitting
// the privilege from desired).
func captureGrantStmt(g *pgq.GrantStmt, st *schema.SchemaState) error {
	if g == nil || st == nil {
		return nil
	}
	privNames := collectGrantPrivileges(g)
	grantees := collectGrantees(g)
	wgo := g.GetGrantOption()
	objType := g.GetObjtype()
	objs := g.GetObjects()
	for _, target := range objs {
		applyGrantToTarget(st, objType, target, g.GetTargtype(), privNames, grantees, wgo, g.GetIsGrant())
	}
	return nil
}

// collectGrantPrivileges flattens a GrantStmt.privileges list into uppercase keywords.
// An empty privilege list means "ALL [PRIVILEGES]" — expand to the per-object-type set.
func collectGrantPrivileges(g *pgq.GrantStmt) []string {
	priv := g.GetPrivileges()
	if len(priv) == 0 {
		return allPrivilegesForObjType(g.GetObjtype())
	}
	out := make([]string, 0, len(priv))
	for _, p := range priv {
		ap := p.GetAccessPriv()
		if ap == nil {
			continue
		}
		name := strings.ToUpper(ap.GetPrivName())
		if name == "" {
			// ALL PRIVILEGES encoded as priv_name="" with no cols
			out = append(out, allPrivilegesForObjType(g.GetObjtype())...)
			continue
		}
		out = append(out, name)
	}
	return out
}

func allPrivilegesForObjType(t pgq.ObjectType) []string {
	switch t {
	case pgq.ObjectType_OBJECT_TABLE, pgq.ObjectType_OBJECT_VIEW, pgq.ObjectType_OBJECT_MATVIEW:
		return []string{"SELECT", "INSERT", "UPDATE", "DELETE", "TRUNCATE", "REFERENCES", "TRIGGER"}
	case pgq.ObjectType_OBJECT_SEQUENCE:
		return []string{"USAGE", "SELECT", "UPDATE"}
	case pgq.ObjectType_OBJECT_FUNCTION, pgq.ObjectType_OBJECT_PROCEDURE, pgq.ObjectType_OBJECT_ROUTINE:
		return []string{"EXECUTE"}
	case pgq.ObjectType_OBJECT_SCHEMA:
		return []string{"USAGE", "CREATE"}
	}
	return nil
}

// collectGrantees returns the grantee role names. "" represents PUBLIC.
func collectGrantees(g *pgq.GrantStmt) []string {
	out := make([]string, 0, len(g.GetGrantees()))
	for _, r := range g.GetGrantees() {
		role := r.GetRoleSpec()
		if role == nil {
			continue
		}
		switch role.GetRoletype() {
		case pgq.RoleSpecType_ROLESPEC_PUBLIC:
			out = append(out, "") // PUBLIC
		case pgq.RoleSpecType_ROLESPEC_CSTRING:
			out = append(out, role.GetRolename())
		case pgq.RoleSpecType_ROLESPEC_CURRENT_ROLE, pgq.RoleSpecType_ROLESPEC_CURRENT_USER:
			out = append(out, "CURRENT_USER")
		case pgq.RoleSpecType_ROLESPEC_SESSION_USER:
			out = append(out, "SESSION_USER")
		}
	}
	return out
}

// applyGrantToTarget locates the target object in st and merges/removes the
// privilege set onto its Privileges field.
func applyGrantToTarget(
	st *schema.SchemaState,
	objType pgq.ObjectType,
	target *pgq.Node,
	targtype pgq.GrantTargetType,
	privs []string,
	grantees []string,
	wgo bool,
	isGrant bool,
) {
	// ALL X IN SCHEMA case
	if targtype == pgq.GrantTargetType_ACL_TARGET_ALL_IN_SCHEMA {
		schemaName := strings.ToLower(target.GetString_().GetSval())
		switch objType {
		case pgq.ObjectType_OBJECT_TABLE:
			for _, t := range st.Tables {
				if t != nil && strings.EqualFold(t.Schema, schemaName) {
					t.Privileges = mergePrivileges(t.Privileges, privs, grantees, wgo, isGrant)
				}
			}
		case pgq.ObjectType_OBJECT_SEQUENCE:
			for _, s := range st.Sequences {
				if s != nil && strings.EqualFold(s.Schema, schemaName) {
					s.Privileges = mergePrivileges(s.Privileges, privs, grantees, wgo, isGrant)
				}
			}
		case pgq.ObjectType_OBJECT_FUNCTION, pgq.ObjectType_OBJECT_PROCEDURE, pgq.ObjectType_OBJECT_ROUTINE:
			for _, f := range st.Functions {
				if f != nil && strings.EqualFold(f.Schema, schemaName) {
					f.Privileges = mergePrivileges(f.Privileges, privs, grantees, wgo, isGrant)
				}
			}
		}
		return
	}
	// Object-by-object
	switch objType {
	case pgq.ObjectType_OBJECT_TABLE, pgq.ObjectType_OBJECT_VIEW, pgq.ObjectType_OBJECT_MATVIEW:
		sch, name := rangeVarParts(target)
		key := schema.TableKey(sch, name)
		if t := st.Tables[key]; t != nil {
			t.Privileges = mergePrivileges(t.Privileges, privs, grantees, wgo, isGrant)
			return
		}
		if v := st.Views[key]; v != nil {
			v.Privileges = mergePrivileges(v.Privileges, privs, grantees, wgo, isGrant)
		}
	case pgq.ObjectType_OBJECT_SEQUENCE:
		sch, name := rangeVarParts(target)
		if s := st.Sequences[schema.SeqKey(sch, name)]; s != nil {
			s.Privileges = mergePrivileges(s.Privileges, privs, grantees, wgo, isGrant)
		}
	case pgq.ObjectType_OBJECT_FUNCTION, pgq.ObjectType_OBJECT_PROCEDURE, pgq.ObjectType_OBJECT_ROUTINE:
		// ObjectWithArgs: schema.name(arg1, arg2)
		owa := target.GetObjectWithArgs()
		if owa == nil {
			return
		}
		sch, name := objectNameParts(owa.GetObjname())
		args := objectArgsToString(owa.GetObjargs())
		ident := sch + "." + name + "(" + args + ")"
		if f := st.Functions[schema.FunctionKey(ident)]; f != nil {
			f.Privileges = mergePrivileges(f.Privileges, privs, grantees, wgo, isGrant)
		}
	}
}

// rangeVarParts extracts (schema, name) from a RangeVar node. Defaults schema to "public".
func rangeVarParts(n *pgq.Node) (string, string) {
	rv := n.GetRangeVar()
	if rv == nil {
		return "public", ""
	}
	sch := rv.GetSchemaname()
	if sch == "" {
		sch = "public"
	}
	return strings.ToLower(sch), strings.ToLower(rv.GetRelname())
}

// objectNameParts unwraps a List of String nodes ("schema","name" or "name") into a
// schema + leaf pair.
func objectNameParts(parts []*pgq.Node) (string, string) {
	switch len(parts) {
	case 1:
		return "public", strings.ToLower(parts[0].GetString_().GetSval())
	case 2:
		return strings.ToLower(parts[0].GetString_().GetSval()),
			strings.ToLower(parts[1].GetString_().GetSval())
	}
	return "public", ""
}

// objectArgsToString deparses function argument-type nodes back to a comma-separated
// type-name string compatible with pg_get_function_identity_arguments().
func objectArgsToString(args []*pgq.Node) string {
	if len(args) == 0 {
		return ""
	}
	out := make([]string, 0, len(args))
	for _, a := range args {
		fp := a.GetFunctionParameter()
		if fp != nil {
			ts, _ := typeNameToSQL(fp.GetArgType())
			out = append(out, ts)
			continue
		}
		// Plain TypeName node
		ts, err := typeNameToSQL(a.GetTypeName())
		if err == nil && ts != "" {
			out = append(out, ts)
		}
	}
	return strings.Join(out, ", ")
}

// mergePrivileges adds (isGrant=true) or removes (isGrant=false) the given priv set
// for the given grantees against the existing privilege list. Returns the new list.
func mergePrivileges(existing []schema.Privilege, privs, grantees []string, wgo, isGrant bool) []schema.Privilege {
	keyOf := func(p schema.Privilege) string { return p.Key() }
	have := make(map[string]int, len(existing))
	for i, p := range existing {
		have[keyOf(p)] = i
	}
	for _, g := range grantees {
		for _, pr := range privs {
			np := schema.Privilege{Grantee: g, Priv: pr, WithGrantOption: wgo}
			k := np.Key()
			if isGrant {
				if _, ok := have[k]; !ok {
					existing = append(existing, np)
					have[k] = len(existing) - 1
				}
			} else {
				if idx, ok := have[k]; ok {
					existing = append(existing[:idx], existing[idx+1:]...)
					// Re-index map after slice mutation
					have = make(map[string]int, len(existing))
					for i, p := range existing {
						have[keyOf(p)] = i
					}
				}
			}
		}
	}
	schema.SortPrivileges(existing)
	return existing
}
