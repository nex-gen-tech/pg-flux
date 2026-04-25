package src

import (
	"strings"

	pgq "github.com/pganalyze/pg_query_go/v6"
)

// policyRolesFromNodes extracts role names from CREATE/ALTER POLICY ... TO ... role list.
func policyRolesFromNodes(nodes []*pgq.Node) []string {
	if len(nodes) == 0 {
		return nil
	}
	var out []string
	for _, n := range nodes {
		if n == nil {
			continue
		}
		if rs := n.GetRoleSpec(); rs != nil {
			if rname := strings.TrimSpace(rs.GetRolename()); rname != "" {
				out = append(out, strings.ToLower(rname))
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	// public is explicit in PG for some policies
	return out
}
