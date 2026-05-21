package src

import (
	"strings"

	pgq "github.com/pganalyze/pg_query_go/v6"

	"github.com/nex-gen-tech/pg-flux/pkg/schema"
)

// captureCreateEnum extracts CREATE TYPE name AS ENUM (...) into structured
// SchemaState.EnumValues so the dump.Verify command (and any other consumer
// that compares desired vs live by enum-key set) can see the declaration.
//
// Before this capture, source-side enums were only kept as raw ExtraDDL, which
// meant `desired.EnumValues` was empty and Verify always flagged every live
// enum type as "undeclared" — the symptom reported in issue #9.
func captureCreateEnum(s *pgq.CreateEnumStmt, st *schema.SchemaState) error {
	if s == nil || st == nil {
		return nil
	}
	sch, name := nameFromNameList(s.GetTypeName())
	if name == "" {
		return nil
	}
	if sch == "" {
		sch = "public"
	}
	var vals []string
	for _, v := range s.GetVals() {
		if v == nil {
			continue
		}
		if str := v.GetString_(); str != nil {
			vals = append(vals, str.GetSval())
		}
	}
	if st.EnumValues == nil {
		st.EnumValues = make(map[string][]string)
	}
	if st.UserTypes == nil {
		st.UserTypes = make(map[string]struct{})
	}
	key := strings.ToLower(sch) + "." + strings.ToLower(name)
	st.EnumValues[key] = vals
	st.UserTypes[key] = struct{}{}
	return nil
}

// captureCreateRange extracts CREATE TYPE name AS RANGE (subtype = ..., ...)
// into SchemaState.RangeTypes. Subtype and other options are captured so the
// differ's owner / drop diff for range types operates symmetrically.
func captureCreateRange(s *pgq.CreateRangeStmt, st *schema.SchemaState) error {
	if s == nil || st == nil {
		return nil
	}
	sch, name := nameFromNameList(s.GetTypeName())
	if name == "" {
		return nil
	}
	if sch == "" {
		sch = "public"
	}
	rt := &schema.RangeType{Schema: strings.ToLower(sch), Name: strings.ToLower(name)}
	for _, p := range s.GetParams() {
		el := p.GetDefElem()
		if el == nil {
			continue
		}
		k := strings.ToLower(el.GetDefname())
		v := defElemValueToString(el.GetArg())
		if k == "subtype" {
			rt.Subtype = v
			continue
		}
		if v != "" {
			rt.Options = append(rt.Options, k+"="+v)
		}
	}
	if st.RangeTypes == nil {
		st.RangeTypes = make(map[string]*schema.RangeType)
	}
	if st.UserTypes == nil {
		st.UserTypes = make(map[string]struct{})
	}
	key := rt.Schema + "." + rt.Name
	st.RangeTypes[key] = rt
	st.UserTypes[key] = struct{}{}
	return nil
}

// captureForeignServer registers CREATE SERVER name [TYPE 'type'] [VERSION 'ver']
// FOREIGN DATA WRAPPER fdw [OPTIONS (...)] into st.ForeignServers so verify can
// compare the set of declared servers to the live pg_foreign_server contents.
func captureForeignServer(s *pgq.CreateForeignServerStmt, st *schema.SchemaState) error {
	if s == nil || st == nil {
		return nil
	}
	name := strings.ToLower(strings.TrimSpace(s.GetServername()))
	if name == "" {
		return nil
	}
	fs := &schema.ForeignServer{
		Name:    name,
		Type:    s.GetServertype(),
		Version: s.GetVersion(),
		Wrapper: strings.ToLower(strings.TrimSpace(s.GetFdwname())),
	}
	for _, o := range s.GetOptions() {
		el := o.GetDefElem()
		if el == nil {
			continue
		}
		v := defElemValueToString(el.GetArg())
		fs.Options = append(fs.Options, el.GetDefname()+"="+v)
	}
	if st.ForeignServers == nil {
		st.ForeignServers = make(map[string]*schema.ForeignServer)
	}
	st.ForeignServers[name] = fs
	return nil
}

// captureForeignTable registers CREATE FOREIGN TABLE [schema.]name (...) SERVER
// srv [OPTIONS (...)] into st.ForeignTables.
func captureForeignTable(s *pgq.CreateForeignTableStmt, st *schema.SchemaState) error {
	if s == nil || st == nil {
		return nil
	}
	bs := s.GetBaseStmt()
	if bs == nil || bs.GetRelation() == nil {
		return nil
	}
	rv := bs.GetRelation()
	sch := strings.ToLower(strings.TrimSpace(rv.GetSchemaname()))
	if sch == "" {
		sch = "public"
	}
	name := strings.ToLower(strings.TrimSpace(rv.GetRelname()))
	if name == "" {
		return nil
	}
	ft := &schema.ForeignTable{
		Schema: sch,
		Name:   name,
		Server: strings.ToLower(strings.TrimSpace(s.GetServername())),
	}
	for _, o := range s.GetOptions() {
		el := o.GetDefElem()
		if el == nil {
			continue
		}
		v := defElemValueToString(el.GetArg())
		ft.Options = append(ft.Options, el.GetDefname()+"="+v)
	}
	if st.ForeignTables == nil {
		st.ForeignTables = make(map[string]*schema.ForeignTable)
	}
	st.ForeignTables[sch+"."+name] = ft
	return nil
}

// capturePublication registers CREATE PUBLICATION name [FOR ALL TABLES | FOR
// TABLE …] [WITH (...)] into st.Publications. Only enough state to support
// "declared vs live" set comparison.
func capturePublication(s *pgq.CreatePublicationStmt, st *schema.SchemaState) error {
	if s == nil || st == nil {
		return nil
	}
	name := strings.ToLower(strings.TrimSpace(s.GetPubname()))
	if name == "" {
		return nil
	}
	pub := &schema.Publication{
		Name:      name,
		AllTables: s.GetForAllTables(),
	}
	for _, o := range s.GetOptions() {
		el := o.GetDefElem()
		if el == nil {
			continue
		}
		if strings.EqualFold(el.GetDefname(), "publish") {
			pub.Publish = defElemValueToString(el.GetArg())
		}
	}
	if st.Publications == nil {
		st.Publications = make(map[string]*schema.Publication)
	}
	st.Publications[name] = pub
	return nil
}
