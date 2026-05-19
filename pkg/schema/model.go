// Package schema holds the internal schema model shared by the parser, inspector, and differ.
package schema

import "github.com/nexg/pg-flux/pkg/pgver"

// SchemaState is the merged desired or live view of a database namespace subset.
type SchemaState struct {
	// PGVersion is the server version this state was inspected against (zero for desired
	// state loaded purely from source files). Used by the differ to fail loud when desired
	// uses a feature unsupported on the target server.
	PGVersion pgver.Version
	Tables    map[string]*Table
	Indexes   map[string]*Index
	Functions map[string]*Function
	Policies  map[string]*Policy
	Views     map[string]*View
	Sequences map[string]*Sequence
	Triggers  map[string]*Trigger
	// Extensions: desired + live (from pg_extension / parsed CREATE EXTENSION).
	Extensions map[string]*Extension
	// UserTypes tracks user-defined type names (enums, domains, composite types) keyed
	// by "schema.name". Populated by the inspector from the live DB; used by the differ
	// to skip CREATE TYPE when the type already exists.
	UserTypes map[string]struct{}
	// EnumValues holds the ordered enum labels for each live enum type keyed by "schema.name".
	// Used by the differ to detect newly added enum values and emit ALTER TYPE ... ADD VALUE.
	EnumValues map[string][]string
	// PendingRLS holds RLS enable/force flags for tables that may not yet exist when
	// ALTER TABLE ... ENABLE ROW LEVEL SECURITY is parsed (cross-file ordering issue).
	// Applied as a post-processing step in LoadDesiredState. Not used by the inspector.
	PendingRLS map[string]*RLSFlags
	// PendingAlterPolicy holds ALTER POLICY statements that arrived before their CREATE POLICY
	// (cross-file ordering). Applied as a post-processing step in LoadDesiredState.
	PendingAlterPolicy []*PendingAlterPol
	// PartitionChildren is the set of live partition child table keys ("schema.name").
	// Populated by the inspector so diffExtraDDL can skip CREATE TABLE IF NOT EXISTS
	// for partition children that already exist.
	PartitionChildren map[string]bool
	// Domains holds user-defined domain definitions keyed by "schema.name".
	// Populated by the inspector from the live DB; used by the differ to detect
	// constraint changes and emit ALTER DOMAIN DDL.
	Domains map[string]*Domain
	// ExtraDDL holds pass-through statements (e.g. ALTER TABLE … ATTACH PARTITION) kept in order.
	ExtraDDL []string
	// MiscObjects lists recognized but not fully modeled objects (FDW, event triggers, etc.).
	MiscObjects []*MiscObject
	// DefaultPrivileges captures ALTER DEFAULT PRIVILEGES state (from pg_default_acl
	// or from source-file ALTER DEFAULT PRIVILEGES statements).
	DefaultPrivileges []*DefaultPrivilege
	// EventTriggers (pg_event_trigger). Database-wide DDL/login triggers.
	EventTriggers map[string]*EventTrigger
	// Statistics (pg_statistic_ext) — extended planner statistics.
	Statistics map[string]*Statistics
}

// RLSFlags carries pending RLS enable/force flags for a table.
type RLSFlags struct {
	Enabled    bool
	EnabledSet bool
	Forced     bool
	ForcedSet  bool
}

// PendingAlterPol holds an ALTER POLICY that arrived before its CREATE POLICY was parsed.
type PendingAlterPol struct {
	Key       string // PolicyKey
	UsingSQL  string
	WithCheck string
	Roles     []string
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
	Owner          string
	PrimaryKeyCols []string
	// PartitionBy holds the partition strategy and key (e.g. "RANGE (ts)") for
	// partitioned tables. Empty for regular tables.
	PartitionBy string
	// Unlogged is true when the table is UNLOGGED (pg_class.relpersistence='u').
	// UNLOGGED tables skip WAL — faster writes, lost on crash. Common for caches.
	Unlogged bool
	// ReLOptions are the table's WITH (...) storage parameters (pg_class.reloptions
	// parsed into a sorted "key=value" slice). Examples: "fillfactor=70",
	// "autovacuum_vacuum_scale_factor=0.1".
	ReLOptions []string
	// Privileges captures GRANT/REVOKE state on the table (from pg_class.relacl
	// parsed via ParseACL). Empty means "no privileges recorded" — when the desired
	// schema's source files include no GRANT statements, the differ leaves live
	// permissions untouched to avoid accidental REVOKEs.
	Privileges []Privilege
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
	// Collation is the explicit COLLATE clause attached to the column (lowercased
	// schema-qualified collation name, or just the bare name). Empty when omitted.
	Collation string
	// Storage maps to pg_attribute.attstorage:
	//   "p" / "plain"    — PLAIN: no TOAST, fixed-length types
	//   "e" / "external" — EXTERNAL: stored out-of-line, not compressed
	//   "m" / "main"     — MAIN: stored inline if possible, else TOAST
	//   "x" / "extended" — EXTENDED: TOAST + compression
	// Stored as the long form (lower-cased) for readability.
	Storage string
	// Compression is the column-level TOAST compression method (PG14+):
	// "lz4" or "pglz" (or "" for the cluster default). pg_attribute.attcompression.
	Compression string
	Comment     string
	// Identity describes a GENERATED ... AS IDENTITY column (PG10+).
	// "always" = GENERATED ALWAYS AS IDENTITY (manual inserts blocked unless OVERRIDING SYSTEM VALUE)
	// "by-default" = GENERATED BY DEFAULT AS IDENTITY (sequence used when caller omits column)
	// "" = not an identity column. From pg_attribute.attidentity ('a' / 'd' / '').
	Identity string
	// IdentitySequenceOptions holds the literal "(START 100 INCREMENT 5 ...)" body when the
	// source specifies sequence options on the identity. Empty when not specified.
	IdentitySequenceOptions string
	// GeneratedExpr is non-empty for generated columns (stored or virtual):
	// the expression text (without GENERATED ALWAYS AS / STORED|VIRTUAL wrapper).
	GeneratedExpr string
	// GeneratedKind is "stored" (PG12+), "virtual" (PG18+), or "" when not generated.
	// Maps to pg_attribute.attgenerated ('s' / 'v' / '').
	GeneratedKind string
	// CustomUsing holds the USING expression override for ALTER COLUMN TYPE, provided via
	// a "-- @using <expr>" comment immediately preceding the column definition. When set,
	// this expression replaces the default "col::newtype" USING clause so incompatible
	// casts (e.g. boolean → enum) can be handled with a user-supplied CASE expression.
	CustomUsing string
}

// Clone returns a deep copy of SchemaState.
func (s *SchemaState) Clone() *SchemaState {
	if s == nil {
		return nil
	}
	out := &SchemaState{
		PGVersion: s.PGVersion,
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
	if len(s.UserTypes) > 0 {
		out.UserTypes = make(map[string]struct{}, len(s.UserTypes))
		for k := range s.UserTypes {
			out.UserTypes[k] = struct{}{}
		}
	}
	if len(s.EnumValues) > 0 {
		out.EnumValues = make(map[string][]string, len(s.EnumValues))
		for k, v := range s.EnumValues {
			out.EnumValues[k] = append([]string(nil), v...)
		}
	}
	if len(s.PendingRLS) > 0 {
		out.PendingRLS = make(map[string]*RLSFlags, len(s.PendingRLS))
		for k, v := range s.PendingRLS {
			if v == nil {
				continue
			}
			vc := *v
			out.PendingRLS[k] = &vc
		}
	}
	if len(s.PendingAlterPolicy) > 0 {
		out.PendingAlterPolicy = make([]*PendingAlterPol, len(s.PendingAlterPolicy))
		for i, p := range s.PendingAlterPolicy {
			if p == nil {
				continue
			}
			pc := *p
			pc.Roles = append([]string(nil), p.Roles...)
			out.PendingAlterPolicy[i] = &pc
		}
	}
	if len(s.PartitionChildren) > 0 {
		out.PartitionChildren = make(map[string]bool, len(s.PartitionChildren))
		for k, v := range s.PartitionChildren {
			out.PartitionChildren[k] = v
		}
	}
	if len(s.Domains) > 0 {
		out.Domains = make(map[string]*Domain, len(s.Domains))
		for k, d := range s.Domains {
			if d == nil {
				continue
			}
			dc := *d
			dc.Constraints = append([]DomainConstraint(nil), d.Constraints...)
			out.Domains[k] = &dc
		}
	}
	for k, t := range s.Tables {
		if t == nil {
			continue
		}
		nt := &Table{
			Schema: t.Schema, Name: t.Name, OldName: t.OldName, Deprecated: t.Deprecated,
			RLSEnabled: t.RLSEnabled, RLSForced: t.RLSForced, Comment: t.Comment,
			PartitionBy: t.PartitionBy,
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

// DomainConstraint represents a single CHECK constraint on a domain.
type DomainConstraint struct {
	// Name is the constraint name as recorded in pg_constraint; may be empty for desired state.
	Name string
	// Expr is the normalized CHECK body expression (without outer parentheses).
	Expr string
}

// Domain models a user-defined domain type (CREATE DOMAIN ... AS ...).
type Domain struct {
	Schema      string
	Name        string
	BaseType    string
	Constraints []DomainConstraint
}
