// Package schema holds the internal schema model shared by the parser, inspector, and differ.
package schema

// SchemaState is the merged desired or live view of a database namespace subset.
type SchemaState struct {
	Tables    map[string]*Table
	Indexes   map[string]*Index
	Functions map[string]*Function
	Policies  map[string]*Policy
	Views     map[string]*View
	Sequences map[string]*Sequence
	Triggers  map[string]*Trigger
	// Extensions: desired + live (from pg_extension / parsed CREATE EXTENSION).
	Extensions map[string]*Extension
	// ExtraDDL holds pass-through statements (e.g. ALTER TABLE … ATTACH PARTITION) kept in order.
	ExtraDDL []string
	// MiscObjects lists recognized but not fully modeled objects (FDW, event triggers, etc.).
	MiscObjects []*MiscObject
}

// TableKey returns qualified name: "schema.relation" (unquoted, lowercased for lookup unless quoted).
func TableKey(schema, name string) string {
	if schema == "" {
		schema = "public"
	}
	return schema + "." + name
}

// Table models a user table from CREATE TABLE or system catalogs.
type Table struct {
	Schema         string
	Name           string
	OldName        string // from @renamed (desired): previous live name
	Deprecated     bool
	Columns        []*Column
	RLSEnabled     bool
	RLSForced      bool
	Comment        string
	PrimaryKeyCols []string
	// Table-level CHECK / UNIQUE / EXCLUDE / FK
	Checks      []*TableCheck
	Uniques     []*TableUnique
	Excludes    []*TableExclusion
	ForeignKeys []*TableForeignKey
}

// Column models a table column.
type Column struct {
	Name         string
	RenameFrom   string // from @renamed from=X (logical old name in live DB)
	Deprecated   bool
	TypeSQL      string
	NotNull      bool
	DefaultSQL   string
	IsPrimaryKey bool
	Collation    string
}

// Clone returns a deep copy of SchemaState.
func (s *SchemaState) Clone() *SchemaState {
	if s == nil {
		return nil
	}
	out := &SchemaState{
		Tables:    make(map[string]*Table, len(s.Tables)),
		Indexes:   make(map[string]*Index, len(s.Indexes)),
		Functions: make(map[string]*Function, len(s.Functions)),
		Policies:  make(map[string]*Policy, len(s.Policies)),
		Views:     make(map[string]*View, len(s.Views)),
		Sequences: make(map[string]*Sequence, len(s.Sequences)),
		Triggers:  make(map[string]*Trigger, len(s.Triggers)),
	}
	if len(s.ExtraDDL) > 0 {
		out.ExtraDDL = append([]string(nil), s.ExtraDDL...)
	}
	if len(s.MiscObjects) > 0 {
		out.MiscObjects = make([]*MiscObject, len(s.MiscObjects))
		for i, m := range s.MiscObjects {
			if m == nil {
				continue
			}
			mc := *m
			out.MiscObjects[i] = &mc
		}
	}
	if s.Extensions != nil {
		out.Extensions = make(map[string]*Extension, len(s.Extensions))
		for k, e := range s.Extensions {
			if e == nil {
				continue
			}
			ce := *e
			out.Extensions[k] = &ce
		}
	}
	for k, t := range s.Tables {
		if t == nil {
			continue
		}
		nt := &Table{
			Schema: t.Schema, Name: t.Name, OldName: t.OldName, Deprecated: t.Deprecated,
			RLSEnabled: t.RLSEnabled, RLSForced: t.RLSForced, Comment: t.Comment,
		}
		nt.PrimaryKeyCols = append([]string(nil), t.PrimaryKeyCols...)
		for _, x := range t.Checks {
			if x == nil {
				continue
			}
			xc := *x
			nt.Checks = append(nt.Checks, &xc)
		}
		for _, x := range t.Uniques {
			if x == nil {
				continue
			}
			xf := *x
			nt.Uniques = append(nt.Uniques, &xf)
		}
		for _, x := range t.Excludes {
			if x == nil {
				continue
			}
			xf := *x
			nt.Excludes = append(nt.Excludes, &xf)
		}
		for _, x := range t.ForeignKeys {
			if x == nil {
				continue
			}
			xf := *x
			nt.ForeignKeys = append(nt.ForeignKeys, &xf)
		}
		for _, c := range t.Columns {
			if c == nil {
				continue
			}
			nc := *c
			nt.Columns = append(nt.Columns, &nc)
		}
		out.Tables[k] = nt
	}
	for k, ix := range s.Indexes {
		if ix == nil {
			continue
		}
		c := *ix
		out.Indexes[k] = &c
	}
	for k, f := range s.Functions {
		if f == nil {
			continue
		}
		c := *f
		out.Functions[k] = &c
	}
	for k, p := range s.Policies {
		if p == nil {
			continue
		}
		c := *p
		c.Roles = append([]string(nil), p.Roles...)
		out.Policies[k] = &c
	}
	for k, v := range s.Views {
		if v == nil {
			continue
		}
		c := *v
		out.Views[k] = &c
	}
	for k, v := range s.Sequences {
		if v == nil {
			continue
		}
		c := *v
		out.Sequences[k] = &c
	}
	for k, t := range s.Triggers {
		if t == nil {
			continue
		}
		c := *t
		out.Triggers[k] = &c
	}
	return out
}

// ColumnByName returns a column or nil.
func (t *Table) ColumnByName(name string) *Column {
	if t == nil {
		return nil
	}
	for _, c := range t.Columns {
		if c != nil && c.Name == name {
			return c
		}
	}
	return nil
}

// ColumnNames returns all column names in order.
func (t *Table) ColumnNames() []string {
	if t == nil {
		return nil
	}
	var out []string
	for _, c := range t.Columns {
		if c != nil {
			out = append(out, c.Name)
		}
	}
	return out
}
