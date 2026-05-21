package dump

import (
	"fmt"
	"sort"
	"strings"

	"github.com/nex-gen-tech/pg-flux/pkg/differ"
	"github.com/nex-gen-tech/pg-flux/pkg/pgver"
	"github.com/nex-gen-tech/pg-flux/pkg/schema"
)

// sort keys: types/sequences before tables, tables before indexes/views, etc.
// Higher number = later in dependency order.
const (
	sortExtensions       = 10
	sortEnums            = 20
	sortDomains          = 22
	sortCompositeTypes   = 24
	sortRangeTypes       = 26
	sortSequences        = 28
	sortTables           = 30
	sortIndexes          = 32
	sortViews            = 40
	sortFunctions        = 50
	sortTriggers         = 60
	sortPolicies         = 65
	sortEventTriggers    = 70
	sortStatistics       = 75
	sortForeignServers   = 80
	sortForeignTables    = 82
	sortDefaultPrivilege = 90
)

// renderTables emits CREATE TABLE plus all object-level tails (comments, owner,
// RLS toggles, grants). Indexes are rendered separately so the resulting source
// files mirror the typical declarative layout.
func renderTables(s *schema.SchemaState, pgv pgver.Version) []object {
	if s == nil || len(s.Tables) == 0 {
		return nil
	}
	keys := make([]string, 0, len(s.Tables))
	for k := range s.Tables {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var out []object
	for _, k := range keys {
		t := s.Tables[k]
		if t == nil {
			continue
		}
		// Pin pgv at the source-friendly latest so inline STORAGE / COMPRESSION /
		// generated-virtual variants are emitted; pg-flux requires PG14+ anyway.
		create, postAlter := differ.CreateTableSQL(t, pgver.Version{Major: 999})

		var b strings.Builder
		b.WriteString(create)
		b.WriteString("\n")
		for _, a := range postAlter {
			b.WriteString(a)
			b.WriteString(";\n")
		}

		// RLS toggle: emit only when the table opts in. RLSEnabled / RLSForced are
		// not part of CREATE TABLE syntax.
		if t.RLSEnabled {
			fmt.Fprintf(&b, "ALTER TABLE %s.%s ENABLE ROW LEVEL SECURITY;\n",
				differ.Ident(t.Schema), differ.Ident(t.Name))
		}
		if t.RLSForced {
			fmt.Fprintf(&b, "ALTER TABLE %s.%s FORCE ROW LEVEL SECURITY;\n",
				differ.Ident(t.Schema), differ.Ident(t.Name))
		}

		// COMMENT ON TABLE / COLUMN.
		if t.Comment != "" {
			fmt.Fprintf(&b, "COMMENT ON TABLE %s.%s IS %s;\n",
				differ.Ident(t.Schema), differ.Ident(t.Name), quote(t.Comment))
		}
		for _, c := range t.Columns {
			if c == nil || c.Comment == "" {
				continue
			}
			fmt.Fprintf(&b, "COMMENT ON COLUMN %s.%s.%s IS %s;\n",
				differ.Ident(t.Schema), differ.Ident(t.Name), differ.Ident(c.Name), quote(c.Comment))
		}
		// Constraint comments live in pg_description for the constraint OID; the
		// model doesn't currently carry them so they're omitted (parity with
		// inspector behavior).

		// ALTER TABLE ... OWNER TO.
		if t.Owner != "" {
			fmt.Fprintf(&b, "ALTER TABLE %s.%s OWNER TO %s;\n",
				differ.Ident(t.Schema), differ.Ident(t.Name), differ.Ident(t.Owner))
		}

		// GRANTs.
		b.WriteString(renderPrivileges("TABLE",
			fmt.Sprintf("%s.%s", differ.Ident(t.Schema), differ.Ident(t.Name)),
			t.Owner, t.Privileges))

		out = append(out, object{
			Kind: "tables", Schema: t.Schema, Name: t.Name,
			SortKey: sortTables, SQL: b.String(),
		})
	}
	return out
}

// renderPrivileges emits GRANT statements for the desired ACL set. Owner privileges
// are skipped — they're implicit. Multiple privileges on the same grantee are
// folded into one GRANT for compactness.
func renderPrivileges(objType, qualName, owner string, privs []schema.Privilege) string {
	if len(privs) == 0 {
		return ""
	}
	// Group by (grantee, WithGrantOption) so the dump emits one line per grantee.
	type granteeKey struct {
		Grantee string
		WGO     bool
	}
	byGrantee := map[granteeKey][]string{}
	for _, p := range privs {
		if p.Grantee != "" && owner != "" && strings.EqualFold(p.Grantee, owner) {
			continue // owner privileges are implicit
		}
		k := granteeKey{Grantee: p.Grantee, WGO: p.WithGrantOption}
		byGrantee[k] = append(byGrantee[k], strings.ToUpper(p.Priv))
	}
	if len(byGrantee) == 0 {
		return ""
	}
	keys := make([]granteeKey, 0, len(byGrantee))
	for k := range byGrantee {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Grantee != keys[j].Grantee {
			return keys[i].Grantee < keys[j].Grantee
		}
		return !keys[i].WGO && keys[j].WGO
	})
	var b strings.Builder
	for _, k := range keys {
		privList := byGrantee[k]
		sort.Strings(privList)
		grantee := k.Grantee
		switch grantee {
		case "":
			grantee = "PUBLIC"
		case "CURRENT_USER", "SESSION_USER":
			// keywords; do not quote
		default:
			grantee = differ.Ident(grantee)
		}
		wgo := ""
		if k.WGO {
			wgo = " WITH GRANT OPTION"
		}
		fmt.Fprintf(&b, "GRANT %s ON %s %s TO %s%s;\n",
			strings.Join(privList, ", "), objType, qualName, grantee, wgo)
	}
	return b.String()
}

// quote escapes a string literal for use in a SQL statement; doubles embedded
// single quotes.
func quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
