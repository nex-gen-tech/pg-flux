package src

import (
	"strings"

	pgq "github.com/pganalyze/pg_query_go/v6"

	"github.com/nex-gen-tech/pg-flux/pkg/schema"
)

// captureCompositeType parses CREATE TYPE name AS (col1 type1, ...) into a structured
// CompositeType record so the differ can compute attribute add/drop/alter diffs.
//
// The corresponding pass-through ExtraDDL is preserved by the caller path (handled
// by the existing captureDeparsedExtraDDL) for first-apply CREATE — this function
// only populates the structured model so the diff direction can recognize the type.
func captureCompositeType(s *pgq.CompositeTypeStmt, st *schema.SchemaState) error {
	if s == nil || st == nil {
		return nil
	}
	rv := s.GetTypevar()
	if rv == nil {
		return nil
	}
	sch := strings.ToLower(rv.GetSchemaname())
	if sch == "" {
		sch = "public"
	}
	name := strings.ToLower(rv.GetRelname())
	if name == "" {
		return nil
	}
	ct := &schema.CompositeType{Schema: sch, Name: name}
	for _, elt := range s.GetColdeflist() {
		cd := elt.GetColumnDef()
		if cd == nil {
			continue
		}
		typ, err := typeNameToSQL(cd.GetTypeName())
		if err != nil {
			return err
		}
		ct.Attributes = append(ct.Attributes, schema.CompositeAttribute{
			Name: strings.ToLower(cd.GetColname()),
			Type: typ,
		})
	}
	if st.CompositeTypes == nil {
		st.CompositeTypes = make(map[string]*schema.CompositeType)
	}
	st.CompositeTypes[sch+"."+name] = ct
	return nil
}
