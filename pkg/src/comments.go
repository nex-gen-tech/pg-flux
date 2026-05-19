package src

import (
	"strings"

	pgq "github.com/pganalyze/pg_query_go/v6"

	"github.com/nexg/pg-flux/pkg/schema"
)

// captureComment processes a COMMENT ON ... IS '...' statement parsed from a source
// file and sets the Comment field on the matching object in st. Unknown / unsupported
// object kinds are silently ignored (pass-through model is intentional — we don't want
// to error on edge cases like COMMENT ON DATABASE in shared infra files).
func captureComment(c *pgq.CommentStmt, st *schema.SchemaState) error {
	if c == nil || st == nil {
		return nil
	}
	desc := strings.TrimSpace(c.GetComment())
	obj := c.GetObject()
	if obj == nil {
		return nil
	}
	switch c.GetObjtype() {
	case pgq.ObjectType_OBJECT_TABLE, pgq.ObjectType_OBJECT_MATVIEW:
		sch, name := splitQualifiedName(obj)
		key := schema.TableKey(sch, name)
		if t := st.Tables[key]; t != nil {
			t.Comment = desc
		} else if v := st.Views[key]; v != nil {
			v.Comment = desc
		}
	case pgq.ObjectType_OBJECT_VIEW:
		sch, name := splitQualifiedName(obj)
		if v := st.Views[schema.ViewKey(sch, name)]; v != nil {
			v.Comment = desc
		}
	case pgq.ObjectType_OBJECT_SEQUENCE:
		sch, name := splitQualifiedName(obj)
		if s := st.Sequences[schema.SeqKey(sch, name)]; s != nil {
			s.Comment = desc
		}
	case pgq.ObjectType_OBJECT_INDEX:
		sch, name := splitQualifiedName(obj)
		if ix := st.Indexes[schema.IndexKey(sch, name)]; ix != nil {
			ix.Comment = desc
		}
	case pgq.ObjectType_OBJECT_COLUMN:
		// Column reference: qualified as table.column or schema.table.column.
		// pg_query encodes this as a List of String nodes.
		parts := stringListParts(obj)
		if len(parts) < 2 {
			return nil
		}
		var sch, tbl, col string
		switch len(parts) {
		case 2:
			sch, tbl, col = "public", parts[0], parts[1]
		default:
			sch, tbl, col = parts[len(parts)-3], parts[len(parts)-2], parts[len(parts)-1]
		}
		t := st.Tables[schema.TableKey(sch, tbl)]
		if t == nil {
			return nil
		}
		for _, c := range t.Columns {
			if c != nil && c.Name == col {
				c.Comment = desc
				return nil
			}
		}
	case pgq.ObjectType_OBJECT_FUNCTION, pgq.ObjectType_OBJECT_PROCEDURE:
		// COMMENT ON FUNCTION uses an ObjectWithArgs node — out of scope for the
		// quick path. Functions resolved by identity are rare enough in source SQL
		// (most teams comment via docstring inside body) that we leave this for the
		// inspector to populate from pg_description when round-tripping.
		return nil
	case pgq.ObjectType_OBJECT_TRIGGER:
		parts := stringListParts(obj)
		if len(parts) < 2 {
			return nil
		}
		var sch, tbl, name string
		switch len(parts) {
		case 2:
			sch, tbl, name = "public", parts[0], parts[1]
		default:
			sch, tbl, name = parts[len(parts)-3], parts[len(parts)-2], parts[len(parts)-1]
		}
		if tr := st.Triggers[schema.TriggerKey(sch, tbl, name)]; tr != nil {
			tr.Comment = desc
		}
	case pgq.ObjectType_OBJECT_POLICY:
		parts := stringListParts(obj)
		if len(parts) < 2 {
			return nil
		}
		var sch, tbl, name string
		switch len(parts) {
		case 2:
			sch, tbl, name = "public", parts[0], parts[1]
		default:
			sch, tbl, name = parts[len(parts)-3], parts[len(parts)-2], parts[len(parts)-1]
		}
		if pl := st.Policies[schema.PolicyKey(sch, tbl, name)]; pl != nil {
			pl.Comment = desc
		}
	}
	return nil
}

// splitQualifiedName interprets an ObjectType-tagged pg_query Node as a 1- or
// 2-part schema-qualified name (e.g. "public", "users" or just "users"). Returns
// lowercase schema + name; defaults schema to "public" when only the leaf name was given.
func splitQualifiedName(n *pgq.Node) (string, string) {
	parts := stringListParts(n)
	switch len(parts) {
	case 1:
		return "public", parts[0]
	case 2:
		return parts[0], parts[1]
	default:
		return "public", ""
	}
}

// stringListParts unwraps a pg_query List-of-String node into a slice of
// lowercase identifier components.
func stringListParts(n *pgq.Node) []string {
	if n == nil {
		return nil
	}
	if s := n.GetString_(); s != nil {
		return []string{strings.ToLower(s.GetSval())}
	}
	lst := n.GetList()
	if lst == nil {
		return nil
	}
	out := make([]string, 0, len(lst.GetItems()))
	for _, it := range lst.GetItems() {
		if str := it.GetString_(); str != nil {
			out = append(out, strings.ToLower(str.GetSval()))
		}
	}
	return out
}
