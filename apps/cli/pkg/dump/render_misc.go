package dump

import (
	"fmt"
	"sort"
	"strings"

	"github.com/nex-gen-tech/pg-flux/pkg/differ"
	"github.com/nex-gen-tech/pg-flux/pkg/schema"
)

func renderExtensions(s *schema.SchemaState) []object {
	if s == nil || len(s.Extensions) == 0 {
		return nil
	}
	keys := make([]string, 0, len(s.Extensions))
	for k := range s.Extensions {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out []object
	for _, k := range keys {
		e := s.Extensions[k]
		if e == nil {
			continue
		}
		var b strings.Builder
		if e.DefSQL != "" {
			body := strings.TrimRight(e.DefSQL, "; \n")
			b.WriteString(body)
			b.WriteString(";\n")
		} else {
			// Construct minimally from fields.
			fmt.Fprintf(&b, "CREATE EXTENSION IF NOT EXISTS %s", differ.Ident(e.Name))
			if e.Version != "" {
				fmt.Fprintf(&b, " VERSION '%s'", e.Version)
			}
			b.WriteString(";\n")
		}
		out = append(out, object{
			Kind: "extensions", Schema: "_global", Name: e.Name,
			SortKey: sortExtensions, SQL: b.String(),
		})
	}
	return out
}

func renderPolicies(s *schema.SchemaState) []object {
	if s == nil || len(s.Policies) == 0 {
		return nil
	}
	keys := make([]string, 0, len(s.Policies))
	for k := range s.Policies {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out []object
	for _, k := range keys {
		p := s.Policies[k]
		if p == nil {
			continue
		}
		// Prefer p.DefSQL when populated (source-loaded state); the inspector
		// path constructs from fields because pg_policy doesn't expose a deparse.
		var b strings.Builder
		if p.DefSQL != "" {
			body := strings.TrimRight(p.DefSQL, "; \n")
			b.WriteString(body)
		} else {
			b.WriteString(renderPolicyFromFields(p))
		}
		b.WriteString(";\n")
		out = append(out, object{
			Kind: "policies", Schema: p.Schema, Name: p.Table + "." + p.Name,
			SortKey: sortPolicies, SQL: b.String(),
		})
	}
	return out
}

// renderPolicyFromFields constructs a CREATE POLICY statement from the
// structured fields. Inspector callers populate (Schema, Table, Name, Cmd,
// Permissive, UsingSQL, WithCheck, Roles) but not DefSQL — so the dump path
// has to build the source form. Order of clauses matches PG documentation.
func renderPolicyFromFields(p *schema.Policy) string {
	var b strings.Builder
	fmt.Fprintf(&b, "CREATE POLICY %s ON %s.%s",
		differ.Ident(p.Name), differ.Ident(p.Schema), differ.Ident(p.Table))
	// Permissive is the default; only emit RESTRICTIVE when applicable.
	if !p.Permissive {
		b.WriteString(" AS RESTRICTIVE")
	}
	if p.Cmd != "" && strings.ToUpper(p.Cmd) != "ALL" {
		fmt.Fprintf(&b, " FOR %s", strings.ToUpper(p.Cmd))
	}
	if len(p.Roles) > 0 {
		quoted := make([]string, len(p.Roles))
		for i, r := range p.Roles {
			if strings.EqualFold(r, "public") {
				quoted[i] = "PUBLIC"
			} else {
				quoted[i] = differ.Ident(r)
			}
		}
		fmt.Fprintf(&b, " TO %s", strings.Join(quoted, ", "))
	}
	if p.UsingSQL != "" {
		fmt.Fprintf(&b, " USING (%s)", p.UsingSQL)
	}
	if p.WithCheck != "" {
		fmt.Fprintf(&b, " WITH CHECK (%s)", p.WithCheck)
	}
	return b.String()
}

func renderEventTriggers(s *schema.SchemaState) []object {
	if s == nil || len(s.EventTriggers) == 0 {
		return nil
	}
	keys := make([]string, 0, len(s.EventTriggers))
	for k := range s.EventTriggers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out []object
	for _, k := range keys {
		et := s.EventTriggers[k]
		if et == nil {
			continue
		}
		var b strings.Builder
		fmt.Fprintf(&b, "CREATE EVENT TRIGGER %s ON %s", differ.Ident(et.Name), et.Event)
		if len(et.Tags) > 0 {
			quoted := make([]string, len(et.Tags))
			for i, t := range et.Tags {
				quoted[i] = quote(t)
			}
			fmt.Fprintf(&b, "\n  WHEN tag IN (%s)", strings.Join(quoted, ", "))
		}
		fmt.Fprintf(&b, "\n  EXECUTE FUNCTION %s;\n", et.Function)
		switch et.Enabled {
		case "D":
			fmt.Fprintf(&b, "ALTER EVENT TRIGGER %s DISABLE;\n", differ.Ident(et.Name))
		case "R":
			fmt.Fprintf(&b, "ALTER EVENT TRIGGER %s ENABLE REPLICA;\n", differ.Ident(et.Name))
		case "A":
			fmt.Fprintf(&b, "ALTER EVENT TRIGGER %s ENABLE ALWAYS;\n", differ.Ident(et.Name))
		}
		if et.Owner != "" {
			fmt.Fprintf(&b, "ALTER EVENT TRIGGER %s OWNER TO %s;\n",
				differ.Ident(et.Name), differ.Ident(et.Owner))
		}
		out = append(out, object{
			Kind: "event_triggers", Schema: "_global", Name: et.Name,
			SortKey: sortEventTriggers, SQL: b.String(),
		})
	}
	return out
}

func renderStatistics(s *schema.SchemaState) []object {
	if s == nil || len(s.Statistics) == 0 {
		return nil
	}
	keys := make([]string, 0, len(s.Statistics))
	for k := range s.Statistics {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out []object
	for _, k := range keys {
		st := s.Statistics[k]
		if st == nil {
			continue
		}
		var b strings.Builder
		fmt.Fprintf(&b, "CREATE STATISTICS %s.%s",
			differ.Ident(st.Schema), differ.Ident(st.Name))
		if len(st.Kinds) > 0 {
			fmt.Fprintf(&b, " (%s)", strings.Join(st.Kinds, ", "))
		}
		fmt.Fprintf(&b, " ON %s FROM %s.%s;\n",
			strings.Join(st.Columns, ", "),
			differ.Ident(st.TableSchema), differ.Ident(st.TableName))
		if st.Comment != "" {
			fmt.Fprintf(&b, "COMMENT ON STATISTICS %s.%s IS %s;\n",
				differ.Ident(st.Schema), differ.Ident(st.Name), quote(st.Comment))
		}
		if st.Owner != "" {
			fmt.Fprintf(&b, "ALTER STATISTICS %s.%s OWNER TO %s;\n",
				differ.Ident(st.Schema), differ.Ident(st.Name), differ.Ident(st.Owner))
		}
		out = append(out, object{
			Kind: "statistics", Schema: st.Schema, Name: st.Name,
			SortKey: sortStatistics, SQL: b.String(),
		})
	}
	return out
}

func renderForeignServers(s *schema.SchemaState) []object {
	if s == nil || len(s.ForeignServers) == 0 {
		return nil
	}
	keys := make([]string, 0, len(s.ForeignServers))
	for k := range s.ForeignServers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out []object
	for _, k := range keys {
		fs := s.ForeignServers[k]
		if fs == nil {
			continue
		}
		var b strings.Builder
		fmt.Fprintf(&b, "CREATE SERVER %s", differ.Ident(fs.Name))
		if fs.Type != "" {
			fmt.Fprintf(&b, " TYPE %s", quote(fs.Type))
		}
		if fs.Version != "" {
			fmt.Fprintf(&b, " VERSION %s", quote(fs.Version))
		}
		fmt.Fprintf(&b, " FOREIGN DATA WRAPPER %s", differ.Ident(fs.Wrapper))
		if len(fs.Options) > 0 {
			fmt.Fprintf(&b, " OPTIONS (%s)", strings.Join(fs.Options, ", "))
		}
		b.WriteString(";\n")
		if fs.Owner != "" {
			fmt.Fprintf(&b, "ALTER SERVER %s OWNER TO %s;\n",
				differ.Ident(fs.Name), differ.Ident(fs.Owner))
		}
		out = append(out, object{
			Kind: "foreign_servers", Schema: "_global", Name: fs.Name,
			SortKey: sortForeignServers, SQL: b.String(),
		})
	}
	return out
}

func renderForeignTables(s *schema.SchemaState) []object {
	if s == nil || len(s.ForeignTables) == 0 {
		return nil
	}
	keys := make([]string, 0, len(s.ForeignTables))
	for k := range s.ForeignTables {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out []object
	for _, k := range keys {
		ft := s.ForeignTables[k]
		if ft == nil {
			continue
		}
		var b strings.Builder
		fmt.Fprintf(&b, "CREATE FOREIGN TABLE %s.%s (\n",
			differ.Ident(ft.Schema), differ.Ident(ft.Name))
		for i, c := range ft.Columns {
			if c == nil {
				continue
			}
			if i > 0 {
				b.WriteString(",\n")
			}
			fmt.Fprintf(&b, "  %s %s", differ.Ident(c.Name), c.TypeSQL)
			if c.NotNull {
				b.WriteString(" NOT NULL")
			}
		}
		fmt.Fprintf(&b, "\n) SERVER %s", differ.Ident(ft.Server))
		if len(ft.Options) > 0 {
			fmt.Fprintf(&b, " OPTIONS (%s)", strings.Join(ft.Options, ", "))
		}
		b.WriteString(";\n")
		if ft.Owner != "" {
			fmt.Fprintf(&b, "ALTER FOREIGN TABLE %s.%s OWNER TO %s;\n",
				differ.Ident(ft.Schema), differ.Ident(ft.Name), differ.Ident(ft.Owner))
		}
		out = append(out, object{
			Kind: "foreign_tables", Schema: ft.Schema, Name: ft.Name,
			SortKey: sortForeignTables, SQL: b.String(),
		})
	}
	return out
}

func renderDefaultPrivileges(s *schema.SchemaState) []object {
	if s == nil || len(s.DefaultPrivileges) == 0 {
		return nil
	}
	// Deterministic order by (ForRole, InSchema, ObjectType).
	dps := append([]*schema.DefaultPrivilege(nil), s.DefaultPrivileges...)
	sort.Slice(dps, func(i, j int) bool {
		if dps[i] == nil || dps[j] == nil {
			return dps[i] != nil
		}
		if dps[i].ForRole != dps[j].ForRole {
			return dps[i].ForRole < dps[j].ForRole
		}
		if dps[i].InSchema != dps[j].InSchema {
			return dps[i].InSchema < dps[j].InSchema
		}
		return dps[i].ObjectType < dps[j].ObjectType
	})
	var out []object
	for _, dp := range dps {
		if dp == nil || len(dp.Grants) == 0 {
			continue
		}
		var b strings.Builder
		type granteeKey struct {
			Grantee string
			WGO     bool
		}
		byGrantee := map[granteeKey][]string{}
		for _, p := range dp.Grants {
			gk := granteeKey{Grantee: p.Grantee, WGO: p.WithGrantOption}
			byGrantee[gk] = append(byGrantee[gk], strings.ToUpper(p.Priv))
		}
		gkeys := make([]granteeKey, 0, len(byGrantee))
		for gk := range byGrantee {
			gkeys = append(gkeys, gk)
		}
		sort.Slice(gkeys, func(i, j int) bool {
			return gkeys[i].Grantee < gkeys[j].Grantee
		})
		for _, gk := range gkeys {
			privs := byGrantee[gk]
			sort.Strings(privs)
			b.WriteString("ALTER DEFAULT PRIVILEGES")
			if dp.ForRole != "" {
				fmt.Fprintf(&b, " FOR ROLE %s", differ.Ident(dp.ForRole))
			}
			if dp.InSchema != "" {
				fmt.Fprintf(&b, " IN SCHEMA %s", differ.Ident(dp.InSchema))
			}
			grantee := gk.Grantee
			if grantee == "" {
				grantee = "PUBLIC"
			} else {
				grantee = differ.Ident(grantee)
			}
			wgo := ""
			if gk.WGO {
				wgo = " WITH GRANT OPTION"
			}
			fmt.Fprintf(&b, " GRANT %s ON %s TO %s%s;\n",
				strings.Join(privs, ", "), dp.ObjectType, grantee, wgo)
		}
		name := dp.ForRole + "_" + dp.InSchema + "_" + dp.ObjectType
		out = append(out, object{
			Kind: "default_privileges", Schema: "_global", Name: name,
			SortKey: sortDefaultPrivilege, SQL: b.String(),
		})
	}
	return out
}
