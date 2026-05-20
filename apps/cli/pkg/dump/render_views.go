package dump

import (
	"fmt"
	"sort"
	"strings"

	"github.com/nexg/pg-flux/pkg/differ"
	"github.com/nexg/pg-flux/pkg/schema"
)

// renderViews emits CREATE [MATERIALIZED] VIEW + tails. Materialized view
// indexes are surfaced as separate index objects (via renderIndexes).
func renderViews(s *schema.SchemaState) []object {
	if s == nil || len(s.Views) == 0 {
		return nil
	}
	keys := make([]string, 0, len(s.Views))
	for k := range s.Views {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out []object
	for _, k := range keys {
		v := s.Views[k]
		if v == nil {
			continue
		}
		var b strings.Builder
		// DefSQL from the inspector is pg_get_viewdef wrapped in CREATE OR
		// REPLACE VIEW or CREATE MATERIALIZED VIEW. It's already source-shaped.
		body := strings.TrimRight(v.DefSQL, "; \n")
		b.WriteString(body)
		b.WriteString(";\n")
		if v.Comment != "" {
			kw := "VIEW"
			if v.Materialized {
				kw = "MATERIALIZED VIEW"
			}
			fmt.Fprintf(&b, "COMMENT ON %s %s.%s IS %s;\n",
				kw, differ.Ident(v.Schema), differ.Ident(v.Name), quote(v.Comment))
		}
		if v.Owner != "" {
			kw := "VIEW"
			if v.Materialized {
				kw = "MATERIALIZED VIEW"
			}
			fmt.Fprintf(&b, "ALTER %s %s.%s OWNER TO %s;\n",
				kw, differ.Ident(v.Schema), differ.Ident(v.Name), differ.Ident(v.Owner))
		}
		b.WriteString(renderPrivileges("TABLE",
			fmt.Sprintf("%s.%s", differ.Ident(v.Schema), differ.Ident(v.Name)),
			v.Owner, v.Privileges))
		out = append(out, object{
			Kind: "views", Schema: v.Schema, Name: v.Name,
			SortKey: sortViews, SQL: b.String(),
		})
	}
	return out
}

func renderSequences(s *schema.SchemaState) []object {
	if s == nil || len(s.Sequences) == 0 {
		return nil
	}
	keys := make([]string, 0, len(s.Sequences))
	for k := range s.Sequences {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out []object
	for _, k := range keys {
		sq := s.Sequences[k]
		if sq == nil {
			continue
		}
		// Skip identity-owned sequences: they're created implicitly with the
		// owning column's GENERATED ... AS IDENTITY clause and re-creating them
		// here would produce a duplicate definition. We detect the implicit
		// case by checking if OwnedBy points to an identity column.
		if isIdentitySequence(s, sq) {
			continue
		}
		var b strings.Builder
		body := strings.TrimRight(sq.DefSQL, "; \n")
		b.WriteString(body)
		b.WriteString(";\n")
		if sq.OwnedBy != "" {
			fmt.Fprintf(&b, "ALTER SEQUENCE %s.%s OWNED BY %s;\n",
				differ.Ident(sq.Schema), differ.Ident(sq.Name), sq.OwnedBy)
		}
		if sq.Comment != "" {
			fmt.Fprintf(&b, "COMMENT ON SEQUENCE %s.%s IS %s;\n",
				differ.Ident(sq.Schema), differ.Ident(sq.Name), quote(sq.Comment))
		}
		if sq.Owner != "" {
			fmt.Fprintf(&b, "ALTER SEQUENCE %s.%s OWNER TO %s;\n",
				differ.Ident(sq.Schema), differ.Ident(sq.Name), differ.Ident(sq.Owner))
		}
		b.WriteString(renderPrivileges("SEQUENCE",
			fmt.Sprintf("%s.%s", differ.Ident(sq.Schema), differ.Ident(sq.Name)),
			sq.Owner, sq.Privileges))
		out = append(out, object{
			Kind: "sequences", Schema: sq.Schema, Name: sq.Name,
			SortKey: sortSequences, SQL: b.String(),
		})
	}
	return out
}

// isIdentitySequence returns true when sq is an implicitly-created sequence
// backing either a GENERATED ... AS IDENTITY column or a SERIAL/BIGSERIAL
// column. Re-creating these via CREATE SEQUENCE in source would produce a
// duplicate at apply time.
//
// Detection: sq.OwnedBy points to a column whose Identity is non-empty
// (modern IDENTITY) OR whose DefaultSQL references this sequence via
// nextval('...'::regclass) (legacy serial). We match by sequence name in the
// default expression rather than full schema-qualified equality because PG
// renders un-qualified for in-schema sequences.
func isIdentitySequence(s *schema.SchemaState, sq *schema.Sequence) bool {
	if sq == nil || sq.OwnedBy == "" {
		return false
	}
	// OwnedBy format: "schema.table.column"
	parts := strings.Split(sq.OwnedBy, ".")
	if len(parts) != 3 {
		return false
	}
	tk := schema.TableKey(parts[0], parts[1])
	t := s.Tables[tk]
	if t == nil {
		return false
	}
	colName := strings.ToLower(parts[2])
	for _, c := range t.Columns {
		if c == nil || c.Name != colName {
			continue
		}
		if c.Identity != "" {
			return true
		}
		// SERIAL/BIGSERIAL: column DEFAULT contains nextval('<seq>'::regclass).
		// Match by the sequence's bare name appearing inside the default text.
		if c.DefaultSQL != "" && strings.Contains(c.DefaultSQL, sq.Name) &&
			strings.Contains(strings.ToLower(c.DefaultSQL), "nextval") {
			return true
		}
	}
	return false
}

func renderFunctions(s *schema.SchemaState) []object {
	if s == nil || len(s.Functions) == 0 {
		return nil
	}
	keys := make([]string, 0, len(s.Functions))
	for k := range s.Functions {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out []object
	for _, k := range keys {
		f := s.Functions[k]
		if f == nil {
			continue
		}
		var b strings.Builder
		body := strings.TrimRight(f.DefSQL, "; \n")
		b.WriteString(body)
		b.WriteString(";\n")
		if f.Comment != "" {
			kw := functionKindKeyword(f.Kind)
			fmt.Fprintf(&b, "COMMENT ON %s %s IS %s;\n", kw, f.Identity, quote(f.Comment))
		}
		if f.Owner != "" {
			kw := functionKindKeyword(f.Kind)
			fmt.Fprintf(&b, "ALTER %s %s OWNER TO %s;\n", kw, f.Identity, differ.Ident(f.Owner))
		}
		b.WriteString(renderPrivileges(functionKindKeyword(f.Kind), f.Identity, f.Owner, f.Privileges))
		out = append(out, object{
			Kind: "functions", Schema: f.Schema, Name: f.Name,
			SortKey: sortFunctions, SQL: b.String(),
		})
	}
	return out
}

func functionKindKeyword(kind string) string {
	switch kind {
	case "a":
		return "AGGREGATE"
	case "p":
		return "PROCEDURE"
	case "w":
		return "FUNCTION" // window functions use FUNCTION keyword for ALTER/COMMENT
	default:
		return "FUNCTION"
	}
}

func renderTriggers(s *schema.SchemaState) []object {
	if s == nil || len(s.Triggers) == 0 {
		return nil
	}
	keys := make([]string, 0, len(s.Triggers))
	for k := range s.Triggers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out []object
	for _, k := range keys {
		tr := s.Triggers[k]
		if tr == nil {
			continue
		}
		var b strings.Builder
		body := strings.TrimRight(tr.DefSQL, "; \n")
		b.WriteString(body)
		b.WriteString(";\n")
		// Non-default enable state needs a follow-up ALTER TABLE.
		switch tr.Enabled {
		case "D":
			fmt.Fprintf(&b, "ALTER TABLE %s.%s DISABLE TRIGGER %s;\n",
				differ.Ident(tr.Schema), differ.Ident(tr.Table), differ.Ident(tr.Name))
		case "R":
			fmt.Fprintf(&b, "ALTER TABLE %s.%s ENABLE REPLICA TRIGGER %s;\n",
				differ.Ident(tr.Schema), differ.Ident(tr.Table), differ.Ident(tr.Name))
		case "A":
			fmt.Fprintf(&b, "ALTER TABLE %s.%s ENABLE ALWAYS TRIGGER %s;\n",
				differ.Ident(tr.Schema), differ.Ident(tr.Table), differ.Ident(tr.Name))
		}
		if tr.Comment != "" {
			fmt.Fprintf(&b, "COMMENT ON TRIGGER %s ON %s.%s IS %s;\n",
				differ.Ident(tr.Name), differ.Ident(tr.Schema), differ.Ident(tr.Table), quote(tr.Comment))
		}
		out = append(out, object{
			Kind: "triggers", Schema: tr.Schema, Name: tr.Table + "." + tr.Name,
			SortKey: sortTriggers, SQL: b.String(),
		})
	}
	return out
}

func renderIndexes(s *schema.SchemaState) []object {
	if s == nil || len(s.Indexes) == 0 {
		return nil
	}
	keys := make([]string, 0, len(s.Indexes))
	for k := range s.Indexes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out []object
	for _, k := range keys {
		ix := s.Indexes[k]
		if ix == nil {
			continue
		}
		// pg_get_indexdef gives us a complete CREATE INDEX statement.
		var b strings.Builder
		body := strings.TrimRight(ix.CreateSQL, "; \n")
		b.WriteString(body)
		b.WriteString(";\n")
		if ix.Comment != "" {
			fmt.Fprintf(&b, "COMMENT ON INDEX %s.%s IS %s;\n",
				differ.Ident(ix.Schema), differ.Ident(ix.Name), quote(ix.Comment))
		}
		out = append(out, object{
			Kind: "indexes", Schema: ix.Schema, Name: ix.Name,
			SortKey: sortIndexes, SQL: b.String(),
		})
	}
	return out
}
