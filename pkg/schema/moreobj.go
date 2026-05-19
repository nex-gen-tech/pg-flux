package schema

import "strings"

// TableCheck is a table-level CHECK (from CREATE TABLE or pg_constraint contype c).
type TableCheck struct {
	Name   string
	DefSQL string // "CHECK (expr)" style fragment comparable to pg_get_constraintdef
}

// TableForeignKey is a table-level foreign key (contype f in pg_constraint).
type TableForeignKey struct {
	Name   string
	DefSQL string // "FOREIGN KEY ..." fragment comparable to pg_get_constraintdef
}

// TableUnique is a named UNIQUE table constraint (contype u; may include NULLS NOT DISTINCT).
type TableUnique struct {
	Name   string
	DefSQL string
}

// TableExclusion is a named EXCLUDE constraint (contype x).
type TableExclusion struct {
	Name   string
	DefSQL string
}

// View is a regular or materialized view.
type View struct {
	Schema       string
	Name         string
	DefSQL       string
	Materialized bool
	Comment      string
}

// Sequence is a free-standing sequence.
type Sequence struct {
	Schema, Name, DefSQL string
	Comment              string
}

// Trigger is a non-internal trigger.
type Trigger struct {
	Schema, Table, Name, DefSQL string
	Comment                     string
}

// ConstraintKey names a constraint within a table: schema.relation/constraint
func ConstraintKey(sch, tbl, conName string) string {
	if sch == "" {
		sch = "public"
	}
	return TableKey(sch, tbl) + "/" + strings.ToLower(conName)
}

// ViewKey is schema.relation.
func ViewKey(sch, name string) string {
	if sch == "" {
		sch = "public"
	}
	return TableKey(sch, name)
}

// SeqKey is schema.relation.
func SeqKey(sch, name string) string { return ViewKey(sch, name) }

// TriggerKey is schema.relation/trigger.
func TriggerKey(sch, tbl, tg string) string {
	if sch == "" {
		sch = "public"
	}
	return TableKey(sch, tbl) + "/" + strings.ToLower(tg)
}
