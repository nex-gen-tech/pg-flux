package differ

import (
	"regexp"
	"strings"

	"github.com/nexg/pg-flux/pkg/plan"
	"github.com/nexg/pg-flux/pkg/schema"
)

// Patterns for ExtraDDL idempotency rewrites.
var (
	// CREATE SCHEMA [IF NOT EXISTS] name  — PG supports IF NOT EXISTS natively.
	reCreateSchema = regexp.MustCompile(`(?i)^(CREATE\s+SCHEMA\s+)(?:IF\s+NOT\s+EXISTS\s+)?(.+)`)
	// ALTER TYPE name ADD VALUE [IF NOT EXISTS] 'val'  — PG 12+ supports IF NOT EXISTS.
	reAlterTypeAddValue = regexp.MustCompile(`(?i)^(ALTER\s+TYPE\s+\S+\s+ADD\s+VALUE\s+)(?:IF\s+NOT\s+EXISTS\s+)?(.+)`)
	// CREATE TYPE / CREATE DOMAIN  — no native IF NOT EXISTS; wrap in a DO block.
	reCreateType   = regexp.MustCompile(`(?i)^CREATE\s+TYPE\s+`)
	reCreateDomain = regexp.MustCompile(`(?i)^CREATE\s+DOMAIN\s+`)
	// CREATE TABLE ... PARTITION OF  — use IF NOT EXISTS for idempotency.
	reCreateTablePartOf = regexp.MustCompile(`(?i)^(CREATE\s+TABLE\s+)(?:IF\s+NOT\s+EXISTS\s+)?(\S+\s+PARTITION\s+OF\s+.*)`)
)

// makeExtraDDLIdempotent rewrites a single ExtraDDL statement to be safe to
// re-apply on a DB that may already contain the object.
//
//   - CREATE SCHEMA → CREATE SCHEMA IF NOT EXISTS
//   - ALTER TYPE … ADD VALUE → ALTER TYPE … ADD VALUE IF NOT EXISTS
//   - CREATE TYPE / CREATE DOMAIN → wrapped in a DO block that ignores duplicate_object
//   - Everything else → returned unchanged (applied as-is; will error if object exists)
func makeExtraDDLIdempotent(sql string) string {
	s := strings.TrimRight(strings.TrimSpace(sql), ";")
	if s == "" {
		return sql
	}
	// CREATE SCHEMA IF NOT EXISTS
	if m := reCreateSchema.FindStringSubmatch(s); len(m) == 3 {
		return m[1] + "IF NOT EXISTS " + m[2]
	}
	// ALTER TYPE ... ADD VALUE IF NOT EXISTS
	if m := reAlterTypeAddValue.FindStringSubmatch(s); len(m) == 3 {
		return m[1] + "IF NOT EXISTS " + m[2]
	}
	// CREATE TABLE ... PARTITION OF → CREATE TABLE IF NOT EXISTS ... PARTITION OF
	if m := reCreateTablePartOf.FindStringSubmatch(s); len(m) == 3 {
		return m[1] + "IF NOT EXISTS " + m[2]
	}
	// CREATE TYPE / CREATE DOMAIN: wrap in DO block for idempotency.
	// Single quotes do NOT need escaping inside dollar-quoted strings.
	if reCreateType.MatchString(s) || reCreateDomain.MatchString(s) {
		return "DO $pgflux$ BEGIN " + s + "; EXCEPTION WHEN duplicate_object THEN NULL; END $pgflux$"
	}
	return sql
}

func ensureExtMaps(s *schema.SchemaState) {
	if s == nil {
		return
	}
	if s.Extensions == nil {
		s.Extensions = make(map[string]*schema.Extension)
	}
}

func diffExtensions(d, l *schema.SchemaState) []change {
	if d == nil || d.Extensions == nil {
		// Desired schema does not declare extensions; never auto-DROP from live.
		return nil
	}
	var out []change
	ensureExtMaps(d)
	ensureExtMaps(l)
	for k, de := range d.Extensions {
		if de == nil {
			continue
		}
		le := l.Extensions[k]
		if le == nil {
			out = append(out, change{kind: plan.ChangeCreateExtension, ext: de})
			continue
		}
		// Version / upgrade path: prefer ALTER EXTENSION ... UPDATE TO over DROP+CREATE.
		if strings.TrimSpace(de.Version) != "" && strings.TrimSpace(le.Version) != "" && de.Version != le.Version {
			out = append(out, change{kind: plan.ChangeUpdateExtension, ext: de, extLiveVer: le.Version})
			continue
		}
		if strings.TrimSpace(de.DefSQL) != "" && strings.TrimSpace(le.DefSQL) != "" {
			if fpGenericSQL(de.DefSQL) != fpGenericSQL(le.DefSQL) {
				out = append(out, change{kind: plan.ChangeDropExtension, dropExt: k, ext: le})
				out = append(out, change{kind: plan.ChangeCreateExtension, ext: de})
			}
		}
	}
	for k, le := range l.Extensions {
		if le == nil {
			continue
		}
		if d.Extensions[k] == nil {
			out = append(out, change{kind: plan.ChangeDropExtension, dropExt: k, ext: le})
		}
	}
	return out
}

// reCreateEnum matches CREATE TYPE [schema.]name AS ENUM (...) and captures the values list.
var reCreateEnum = regexp.MustCompile(`(?is)^CREATE\s+TYPE\s+(?:\w+\.)?(\w+)\s+AS\s+ENUM\s*\(([^)]*)\)`)

// parseEnumValues extracts the ordered list of enum labels from a parenthesised
// comma-separated list of single-quoted strings.
func parseEnumValues(list string) []string {
	var vals []string
	for _, part := range strings.Split(list, ",") {
		s := strings.TrimSpace(part)
		if len(s) >= 2 && s[0] == '\'' && s[len(s)-1] == '\'' {
			vals = append(vals, s[1:len(s)-1])
		}
	}
	return vals
}

var reExtractTypeName = regexp.MustCompile(`(?i)^CREATE\s+(?:TYPE|DOMAIN)\s+(?:(\w+)\.)?(\w+)`)

// reExtractTableName extracts the schema-qualified table name from a CREATE TABLE statement.
var reExtractTableName = regexp.MustCompile(`(?i)^CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?(?:(\w+)\.)?(\w+)`)

func diffExtraDDL(d *schema.SchemaState, live *schema.SchemaState) []change {
	if d == nil || len(d.ExtraDDL) == 0 {
		return nil
	}
	var out []change
	for _, sql := range d.ExtraDDL {
		s := strings.TrimSpace(sql)
		if s == "" {
			continue
		}
		// Skip CREATE TABLE ... PARTITION OF when the partition child already exists.
		if reCreateTablePartOf.MatchString(s) {
			if live != nil && len(live.PartitionChildren) > 0 {
				if m := reExtractTableName.FindStringSubmatch(s); len(m) == 3 {
					ns := m[1]
					if ns == "" {
						ns = "public"
					}
					key := strings.ToLower(ns) + "." + strings.ToLower(m[2])
					if live.PartitionChildren[key] {
						continue // partition child already exists, skip
					}
				}
			}
		}
		idempotent := makeExtraDDLIdempotent(s)
		// Give CREATE TYPE / CREATE DOMAIN the correct op-score so they sort
		// before CREATE TABLE and ALTER COLUMN statements that reference the type.
		if reCreateType.MatchString(s) || reCreateDomain.MatchString(s) {
			// Check if the type already exists in the live DB.
			typeExists := false
			if live != nil && len(live.UserTypes) > 0 {
				if m := reExtractTypeName.FindStringSubmatch(s); len(m) == 3 {
					ns := m[1]
					if ns == "" {
						ns = "public"
					}
					key := strings.ToLower(ns) + "." + strings.ToLower(m[2])
					if _, exists := live.UserTypes[key]; exists {
						typeExists = true
						// For enums: diff values and emit ADD VALUE for new labels.
						if em := reCreateEnum.FindStringSubmatch(s); len(em) == 3 {
							desiredVals := parseEnumValues(em[2])
							liveVals := live.EnumValues[key]
							liveSet := make(map[string]struct{}, len(liveVals))
							for _, v := range liveVals {
								liveSet[v] = struct{}{}
							}
							typeFull := strings.TrimSpace(reExtractTypeName.FindString(s))
						_ = typeFull
						// Always use the fully-qualified name (ns already defaults to "public")
						// so ALTER TYPE ... ADD VALUE works regardless of search_path.
						qualName := ns + "." + m[2]
						for _, v := range desiredVals {
							if _, ok := liveSet[v]; !ok {
								addSQL := "ALTER TYPE " + qualName + " ADD VALUE IF NOT EXISTS '" + v + "'"
								out = append(out, change{kind: plan.ChangeRawSQL, rawSQL: addSQL})
							}
						}
						}
					}
				}
			}
			if !typeExists {
				out = append(out, change{kind: plan.ChangeCreateType, rawSQL: idempotent})
			}
		} else {
			out = append(out, change{kind: plan.ChangeRawSQL, rawSQL: idempotent})
		}
	}
	return out
}

// diffMiscObjects emits MiscObject statements (GRANT, REVOKE, FDW, etc.) as raw DDL.
// Since the inspector does not currently load these from the live DB, they are always
// re-emitted. GRANT/REVOKE are idempotent; other MiscObject kinds are emitted as-is.
func diffMiscObjects(d *schema.SchemaState) []change {
	if d == nil || len(d.MiscObjects) == 0 {
		return nil
	}
	var out []change
	for _, m := range d.MiscObjects {
		if m == nil {
			continue
		}
		s := strings.TrimSpace(m.DefSQL)
		if s == "" {
			continue
		}
		out = append(out, change{kind: plan.ChangeRawSQL, rawSQL: s})
	}
	return out
}
