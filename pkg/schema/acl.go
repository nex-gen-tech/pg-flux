package schema

import (
	"fmt"
	"sort"
	"strings"
)

// Privilege models a single granted privilege on an object. It corresponds to the
// PostgreSQL aclitem type, parsed via ParseACLItem.
//
// Privilege text uses the SQL keyword form ("SELECT", "INSERT", "USAGE", etc.),
// canonically uppercased. Grantee "" represents PUBLIC.
type Privilege struct {
	Grantee         string
	Grantor         string
	Priv            string
	WithGrantOption bool
}

// privCodeToKeyword maps the single-character ACL code (per PostgreSQL aclitem.h)
// to the keyword form used in GRANT / REVOKE statements.
var privCodeToKeyword = map[byte]string{
	'r': "SELECT",
	'w': "UPDATE",
	'a': "INSERT",
	'd': "DELETE",
	'D': "TRUNCATE",
	'x': "REFERENCES",
	't': "TRIGGER",
	'X': "EXECUTE",
	'U': "USAGE",
	'C': "CREATE",
	'c': "CONNECT",
	'T': "TEMPORARY",
	'm': "MAINTAIN", // PG17+
	's': "SET",      // PG15+ for GUC parameter privilege
	'A': "ALTER SYSTEM",
}

// ParseACLItem parses a single pg_aclitem string of the form "grantee=privs/grantor"
// into a slice of Privilege records (one per privilege code). Returns nil on a
// malformed item rather than erroring — ACL parsing is best-effort.
//
// Examples:
//
//	"app_owner=arwdDxt/postgres"       -> 7 privileges granted to app_owner by postgres
//	"=r/postgres"                       -> SELECT to PUBLIC by postgres
//	"app_owner=arwd*/postgres"          -> a/r/w get WITH GRANT OPTION on d
func ParseACLItem(item string) []Privilege {
	eq := strings.IndexByte(item, '=')
	slash := strings.LastIndexByte(item, '/')
	if eq < 0 || slash < 0 || slash < eq {
		return nil
	}
	grantee := strings.Trim(item[:eq], `"`)
	privs := item[eq+1 : slash]
	grantor := strings.Trim(item[slash+1:], `"`)
	out := make([]Privilege, 0, len(privs))
	for i := 0; i < len(privs); i++ {
		kw, ok := privCodeToKeyword[privs[i]]
		if !ok {
			continue
		}
		wgo := false
		if i+1 < len(privs) && privs[i+1] == '*' {
			wgo = true
			i++
		}
		out = append(out, Privilege{
			Grantee:         grantee,
			Grantor:         grantor,
			Priv:            kw,
			WithGrantOption: wgo,
		})
	}
	return out
}

// ParseACL converts an array of aclitem text values (e.g. from pg_class.relacl)
// into a deterministically-sorted Privilege slice. The order is grantee, priv,
// WithGrantOption — so two semantically equal ACLs compare equal.
func ParseACL(items []string) []Privilege {
	var out []Privilege
	for _, it := range items {
		it = strings.TrimSpace(it)
		if it == "" {
			continue
		}
		out = append(out, ParseACLItem(it)...)
	}
	SortPrivileges(out)
	return out
}

// SortPrivileges sorts privileges in a stable, canonical order for comparison.
func SortPrivileges(p []Privilege) {
	sort.Slice(p, func(i, j int) bool {
		if p[i].Grantee != p[j].Grantee {
			return p[i].Grantee < p[j].Grantee
		}
		if p[i].Priv != p[j].Priv {
			return p[i].Priv < p[j].Priv
		}
		return !p[i].WithGrantOption && p[j].WithGrantOption
	})
}

// PrivilegeKey returns a value suitable for set-based comparison: the grantee +
// privilege keyword + WGO flag. Grantor is intentionally NOT part of the key —
// grant statements re-assign grantor implicitly; we don't drive diffs on that.
func (p Privilege) Key() string {
	wgo := ""
	if p.WithGrantOption {
		wgo = "*"
	}
	return fmt.Sprintf("%s|%s|%s", p.Grantee, p.Priv, wgo)
}

// DiffPrivileges returns (toGrant, toRevoke) — privileges present in desired but
// not in live (need GRANT), and privileges in live but not in desired (need REVOKE).
// Comparison is by Key() so grantor differences are ignored.
func DiffPrivileges(desired, live []Privilege) (toGrant, toRevoke []Privilege) {
	dKeys := make(map[string]Privilege, len(desired))
	for _, p := range desired {
		dKeys[p.Key()] = p
	}
	lKeys := make(map[string]Privilege, len(live))
	for _, p := range live {
		lKeys[p.Key()] = p
	}
	for k, p := range dKeys {
		if _, ok := lKeys[k]; !ok {
			toGrant = append(toGrant, p)
		}
	}
	for k, p := range lKeys {
		if _, ok := dKeys[k]; !ok {
			toRevoke = append(toRevoke, p)
		}
	}
	SortPrivileges(toGrant)
	SortPrivileges(toRevoke)
	return
}
