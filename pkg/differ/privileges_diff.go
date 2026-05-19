package differ

import (
	"fmt"
	"strings"

	"github.com/nexg/pg-flux/pkg/plan"
	"github.com/nexg/pg-flux/pkg/schema"
)

// diffPrivileges emits GRANT / REVOKE statements when the desired ACL set differs
// from the live ACL set on a given object. Skips objects where desired has no
// Privileges recorded (declarative opt-in: source files without GRANTs leave live
// permissions untouched — accidentally revoking everything is worse than a no-op).
func diffPrivileges(d, l *schema.SchemaState) []change {
	var out []change
	if d == nil || l == nil {
		return out
	}
	emitObj := func(objType, qualName string, desired, live []schema.Privilege) {
		// Honour the declarative opt-in: a fully-empty desired Privileges slice
		// means "don't manage permissions on this object."
		if len(desired) == 0 {
			return
		}
		grants, revokes := schema.DiffPrivileges(desired, live)
		for _, p := range revokes {
			out = append(out, change{
				kind:   plan.ChangeRawSQL,
				rawSQL: renderRevoke(objType, qualName, p),
				tbl:    qualName,
			})
		}
		for _, p := range grants {
			out = append(out, change{
				kind:   plan.ChangeRawSQL,
				rawSQL: renderGrant(objType, qualName, p),
				tbl:    qualName,
			})
		}
	}
	for k, t := range d.Tables {
		if t == nil {
			continue
		}
		live := l.Tables[k]
		var lp []schema.Privilege
		if live != nil {
			lp = live.Privileges
		}
		emitObj("TABLE", ident(t.Schema)+"."+ident(t.Name), t.Privileges, lp)
	}
	for k, v := range d.Views {
		if v == nil {
			continue
		}
		live := l.Views[k]
		var lp []schema.Privilege
		if live != nil {
			lp = live.Privileges
		}
		kw := "TABLE"
		if v.Materialized {
			// PG GRANT syntax has no MATERIALIZED VIEW kind — use TABLE.
			kw = "TABLE"
		}
		emitObj(kw, ident(v.Schema)+"."+ident(v.Name), v.Privileges, lp)
	}
	for k, s := range d.Sequences {
		if s == nil {
			continue
		}
		live := l.Sequences[k]
		var lp []schema.Privilege
		if live != nil {
			lp = live.Privileges
		}
		emitObj("SEQUENCE", ident(s.Schema)+"."+ident(s.Name), s.Privileges, lp)
	}
	for k, f := range d.Functions {
		if f == nil {
			continue
		}
		live := l.Functions[k]
		var lp []schema.Privilege
		if live != nil {
			lp = live.Privileges
		}
		kw := "FUNCTION"
		switch f.Kind {
		case "p":
			kw = "PROCEDURE"
		}
		emitObj(kw, f.Identity, f.Privileges, lp)
	}
	return out
}

// renderGrant builds a GRANT statement for one Privilege on one qualified object.
func renderGrant(objType, qualName string, p schema.Privilege) string {
	grantee := p.Grantee
	if grantee == "" {
		grantee = "PUBLIC"
	} else if grantee != "CURRENT_USER" && grantee != "SESSION_USER" {
		grantee = ident(grantee)
	}
	wgo := ""
	if p.WithGrantOption {
		wgo = " WITH GRANT OPTION"
	}
	return fmt.Sprintf("GRANT %s ON %s %s TO %s%s",
		strings.ToUpper(p.Priv), objType, qualName, grantee, wgo)
}

// renderRevoke builds a REVOKE statement matching renderGrant's form.
func renderRevoke(objType, qualName string, p schema.Privilege) string {
	grantee := p.Grantee
	if grantee == "" {
		grantee = "PUBLIC"
	} else if grantee != "CURRENT_USER" && grantee != "SESSION_USER" {
		grantee = ident(grantee)
	}
	// Revoking GRANT OPTION alone is different from revoking the privilege itself.
	// For simplicity we always revoke the full privilege; the next iteration of the
	// loop will re-grant if it should still exist (without WGO).
	return fmt.Sprintf("REVOKE %s ON %s %s FROM %s",
		strings.ToUpper(p.Priv), objType, qualName, grantee)
}
