package src

import (
	"fmt"
	"strings"

	pgq "github.com/pganalyze/pg_query_go/v6"

	"github.com/nexg/pg-flux/pkg/schema"
)

func ensureMoreMaps(st *schema.SchemaState) {
	if st.Views == nil {
		st.Views = make(map[string]*schema.View)
	}
	if st.Sequences == nil {
		st.Sequences = make(map[string]*schema.Sequence)
	}
	if st.Triggers == nil {
		st.Triggers = make(map[string]*schema.Trigger)
	}
}

func captureView(v *pgq.ViewStmt, raw *pgq.RawStmt, st *schema.SchemaState) error {
	if v == nil || v.GetView() == nil {
		return nil
	}
	sql, err := deparseOne(raw)
	if err != nil {
		return err
	}
	sch := v.GetView().GetSchemaname()
	if sch == "" {
		sch = "public"
	}
	name := strings.ToLower(v.GetView().GetRelname())
	ensureMoreMaps(st)
	k := schema.ViewKey(sch, name)
	if st.Views[k] != nil {
		return fmt.Errorf("duplicate view %q", k)
	}
	st.Views[k] = &schema.View{Schema: sch, Name: name, DefSQL: sql, Materialized: false}
	return nil
}

func captureMatView(c *pgq.CreateTableAsStmt, raw *pgq.RawStmt, st *schema.SchemaState) error {
	if c == nil {
		return nil
	}
	if c.GetObjtype() != pgq.ObjectType_OBJECT_MATVIEW {
		return nil
	}
	into := c.GetInto()
	if into == nil || into.GetRel() == nil {
		return nil
	}
	sch := into.GetRel().GetSchemaname()
	if sch == "" {
		sch = "public"
	}
	name := strings.ToLower(into.GetRel().GetRelname())
	sql, err := deparseOne(raw)
	if err != nil {
		return err
	}
	ensureMoreMaps(st)
	k := schema.ViewKey(sch, name)
	if st.Views[k] != nil {
		return fmt.Errorf("duplicate view %q", k)
	}
	st.Views[k] = &schema.View{Schema: sch, Name: name, DefSQL: sql, Materialized: true}
	return nil
}

func captureSequence(c *pgq.CreateSeqStmt, raw *pgq.RawStmt, st *schema.SchemaState) error {
	if c == nil || c.GetSequence() == nil {
		return nil
	}
	sql, err := deparseOne(raw)
	if err != nil {
		return err
	}
	r := c.GetSequence()
	sch := r.GetSchemaname()
	if sch == "" {
		sch = "public"
	}
	name := strings.ToLower(r.GetRelname())
	ensureMoreMaps(st)
	k := schema.SeqKey(sch, name)
	if st.Sequences[k] != nil {
		return fmt.Errorf("duplicate sequence %q", k)
	}
	st.Sequences[k] = &schema.Sequence{Schema: sch, Name: name, DefSQL: sql}
	return nil
}

func captureTrigger(tg *pgq.CreateTrigStmt, raw *pgq.RawStmt, st *schema.SchemaState) error {
	if tg == nil || tg.GetRelation() == nil {
		return nil
	}
	sql, err := deparseOne(raw)
	if err != nil {
		return err
	}
	r := tg.GetRelation()
	sch := r.GetSchemaname()
	if sch == "" {
		sch = "public"
	}
	tbl := strings.ToLower(r.GetRelname())
	tname := strings.ToLower(tg.GetTrigname())
	ensureMoreMaps(st)
	k := schema.TriggerKey(sch, tbl, tname)
	if st.Triggers[k] != nil {
		return fmt.Errorf("duplicate trigger %q", k)
	}
	st.Triggers[k] = &schema.Trigger{Schema: sch, Table: tbl, Name: tname, DefSQL: sql}
	return nil
}
