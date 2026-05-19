package schema

// DefaultPrivilege models a row in pg_default_acl — the privileges PostgreSQL will
// automatically grant on newly-created objects of a given kind, for a given
// (role, schema) combination.
//
// PostgreSQL stores one record per (defaclrole, defaclnamespace, defaclobjtype)
// with defaclacl as an aclitem[] giving the default grants.
type DefaultPrivilege struct {
	// ForRole is the role whose newly-created objects will receive these defaults.
	// "" means cluster-wide (any role).
	ForRole string
	// InSchema is the schema. "" means cluster-wide (any schema).
	InSchema string
	// ObjectType is one of: TABLES, SEQUENCES, FUNCTIONS, TYPES, SCHEMAS.
	// (Maps from pg_default_acl.defaclobjtype: r/S/f/T/n.)
	ObjectType string
	// Grants is the parsed Privilege list (ParseACL on defaclacl).
	Grants []Privilege
}

// DefaultPrivilegeKey returns a comparable identity for ALTER DEFAULT PRIVILEGES,
// keyed by (role, schema, objtype) — the pg_default_acl primary lookup.
func (d DefaultPrivilege) Key() string {
	return d.ForRole + "|" + d.InSchema + "|" + d.ObjectType
}

// DefaclObjTypeCodeToKeyword maps pg_default_acl.defaclobjtype values to the
// keyword form used in ALTER DEFAULT PRIVILEGES statements.
var DefaclObjTypeCodeToKeyword = map[string]string{
	"r": "TABLES",
	"S": "SEQUENCES",
	"f": "FUNCTIONS",
	"T": "TYPES",
	"n": "SCHEMAS",
}
