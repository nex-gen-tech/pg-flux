package differ

import (
	"fmt"
	"strings"

	"github.com/nexg/pg-flux/pkg/plan"
	"github.com/nexg/pg-flux/pkg/schema"
)

// diffDefaultPrivileges emits ALTER DEFAULT PRIVILEGES statements when the desired
// (role, schema, objtype) entries differ from live's pg_default_acl. Only manages
// entries where the desired state has at least one DefaultPrivilege (declarative
// opt-in — empty source means "leave alone").
//
// Matching: an ALTER DEFAULT PRIVILEGES without a FOR ROLE clause records the
// privilege under the CURRENT_USER role in pg_default_acl. Source files with no
// FOR ROLE clause have ForRole="" — we match those against any live entry whose
// schema+objtype matches, regardless of role.
func diffDefaultPrivileges(d, l *schema.SchemaState) []change {
	var out []change
	if d == nil || len(d.DefaultPrivileges) == 0 {
		return out
	}
	if l == nil {
		l = &schema.SchemaState{}
	}
	// Index by Key for set-diff.
	dIdx := indexDefaults(d.DefaultPrivileges)
	lIdx := indexDefaults(l.DefaultPrivileges)
	// Fall-back match: desired entries with ForRole="" can match live entries that
	// only differ in ForRole.
	matchLive := func(d *schema.DefaultPrivilege) *schema.DefaultPrivilege {
		if e, ok := lIdx[d.Key()]; ok {
			return e
		}
		if d.ForRole != "" {
			return nil
		}
		for _, l := range lIdx {
			if l.InSchema == d.InSchema && l.ObjectType == d.ObjectType {
				return l
			}
		}
		return nil
	}
	for k, dEntry := range dIdx {
		lEntry := matchLive(dEntry)
		var lGrants []schema.Privilege
		if lEntry != nil {
			lGrants = lEntry.Grants
		}
		grants, revokes := schema.DiffPrivileges(dEntry.Grants, lGrants)
		_ = k
		for _, p := range revokes {
			out = append(out, change{
				kind:   plan.ChangeRawSQL,
				rawSQL: renderDefaultPrivStmt("REVOKE", "FROM", dEntry, p),
				tbl:    "default-privileges/" + k,
			})
		}
		for _, p := range grants {
			out = append(out, change{
				kind:   plan.ChangeRawSQL,
				rawSQL: renderDefaultPrivStmt("GRANT", "TO", dEntry, p),
				tbl:    "default-privileges/" + k,
			})
		}
	}
	return out
}

func indexDefaults(ds []*schema.DefaultPrivilege) map[string]*schema.DefaultPrivilege {
	out := make(map[string]*schema.DefaultPrivilege, len(ds))
	for _, d := range ds {
		if d == nil {
			continue
		}
		out[d.Key()] = d
	}
	return out
}

func renderDefaultPrivStmt(verb, dir string, d *schema.DefaultPrivilege, p schema.Privilege) string {
	var b strings.Builder
	b.WriteString("ALTER DEFAULT PRIVILEGES")
	if d.ForRole != "" {
		fmt.Fprintf(&b, " FOR ROLE %s", ident(d.ForRole))
	}
	if d.InSchema != "" {
		fmt.Fprintf(&b, " IN SCHEMA %s", ident(d.InSchema))
	}
	grantee := p.Grantee
	if grantee == "" {
		grantee = "PUBLIC"
	} else if grantee != "CURRENT_USER" && grantee != "SESSION_USER" {
		grantee = ident(grantee)
	}
	wgo := ""
	if verb == "GRANT" && p.WithGrantOption {
		wgo = " WITH GRANT OPTION"
	}
	fmt.Fprintf(&b, " %s %s ON %s %s %s%s",
		verb, strings.ToUpper(p.Priv), d.ObjectType, dir, grantee, wgo)
	return b.String()
}
