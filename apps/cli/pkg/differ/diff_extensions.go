package differ

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/nex-gen-tech/pg-flux/pkg/hazard"
	"github.com/nex-gen-tech/pg-flux/pkg/plan"
	"github.com/nex-gen-tech/pg-flux/pkg/schema"
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
	// Only auto-drop live extensions that pg-flux manages AND that have been removed
	// from the desired schema. Extensions installed by a DBA (not in the desired schema
	// at all) are left alone. Since we cannot distinguish "managed and removed" from
	// "never managed", we conservatively skip dropping any live extension that is NOT
	// present in the desired schema. The first loop above handles version upgrades and
	// DefSQL changes (which use DROP+CREATE internally).
	for k, le := range l.Extensions {
		if le == nil {
			continue
		}
		if d.Extensions[k] != nil {
			// Present in desired schema — handled by the first loop above.
			continue
		}
		// Not in desired schema — could be a DBA-managed extension or one recently
		// removed. Leave it alone; pg-flux does not implicitly drop extensions.
		_ = k
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

var reExtractTypeName = regexp.MustCompile(`(?i)CREATE\s+(?:TYPE|DOMAIN)\s+(?:(\w+)\.)?(\w+)`)

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
						// Always use the fully-qualified name (ns already defaults to "public")
						// so ALTER TYPE ... ADD VALUE works regardless of search_path.
						qualName := ns + "." + m[2]
						desiredSet := make(map[string]struct{}, len(desiredVals))
						for _, v := range desiredVals {
							desiredSet[v] = struct{}{}
						}
						// Detect renames before logging removed-as-data-loss. A rename appears
						// as a desired value not in live AND a live value not in desired at
						// the SAME position. PG12+ supports ALTER TYPE foo RENAME VALUE
						// 'old' TO 'new', which is the correct emit for this pattern.
						renamed := detectEnumRenames(desiredVals, liveVals, liveSet, desiredSet)
						for _, r := range renamed {
							out = append(out, change{
								kind:   plan.ChangeRawSQL,
								rawSQL: fmt.Sprintf("ALTER TYPE %s RENAME VALUE '%s' TO '%s'", qualName, r.From, r.To),
							})
							// Update tracking sets so removed-detection below doesn't treat
							// the renamed-old as a still-pending removal.
							delete(liveSet, r.From)
							liveSet[r.To] = struct{}{}
						}
						// Now report any remaining live-only values as blocking — PG offers
						// no DROP VALUE, so the type must be recreated.
						for _, lv := range liveVals {
							if _, ok := desiredSet[lv]; ok {
								continue
							}
							if _, stillLive := liveSet[lv]; !stillLive {
								continue // already accounted for by a rename
							}
							out = append(out, change{
								kind: plan.ChangeRawSQL,
								extraHazards: []hazard.Detected{{
									Type:     hazard.DataLoss,
									Severity: hazard.SeverityBlocking,
									Message:  fmt.Sprintf("enum type %s: value '%s' exists in live DB but not in desired schema — PostgreSQL does not support ALTER TYPE DROP VALUE; type must be manually recreated to remove it", qualName, lv),
								}},
							})
						}
						// Add new values in desired order, using BEFORE/AFTER to preserve position.
						for i, v := range desiredVals {
							if _, ok := liveSet[v]; ok {
								continue // already exists (possibly after rename)
							}
							addSQL := "ALTER TYPE " + qualName + " ADD VALUE IF NOT EXISTS '" + v + "'"
							// Position the new value relative to neighbours in the desired list.
							if i < len(desiredVals)-1 {
								next := desiredVals[i+1]
								if _, exists := liveSet[next]; exists {
									addSQL += " BEFORE '" + next + "'"
								}
							} else if i > 0 {
								prev := desiredVals[i-1]
								if _, exists := liveSet[prev]; exists {
									addSQL += " AFTER '" + prev + "'"
								}
							}
							out = append(out, change{kind: plan.ChangeRawSQL, rawSQL: addSQL})
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

// diffDomains compares desired and live domain definitions and emits ALTER DOMAIN changes.
// Currently handles CHECK constraint additions and removals.
// Called from Diff() before diffExtraDDL (so CREATE DOMAIN for new domains still goes through ExtraDDL).
func diffDomains(desired, live *schema.SchemaState) []change {
	if desired == nil || len(desired.Domains) == 0 {
		return nil
	}
	if live == nil || len(live.Domains) == 0 {
		return nil
	}
	var out []change
	for key, dd := range desired.Domains {
		ld, exists := live.Domains[key]
		if !exists {
			continue // domain doesn't exist in live; CREATE DOMAIN handled via ExtraDDL
		}
		qualName := dd.Schema + "." + dd.Name

		// Build normalized expression sets.
		liveByExpr := make(map[string]string) // normalized expr → constraint name
		for _, lc := range ld.Constraints {
			norm := normExprForCompare(lc.Expr)
			liveByExpr[norm] = lc.Name
		}
		desiredExprs := make(map[string]schema.DomainConstraint)
		for _, dc := range dd.Constraints {
			norm := normExprForCompare(dc.Expr)
			desiredExprs[norm] = dc
		}

		// DROP constraints that exist in live but not in desired.
		for norm, lname := range liveByExpr {
			if _, ok := desiredExprs[norm]; !ok {
				ddl := fmt.Sprintf("ALTER DOMAIN %s DROP CONSTRAINT IF EXISTS %s", qualName, lname)
				out = append(out, change{kind: plan.ChangeRawSQL, rawSQL: ddl})
			}
		}
		// ADD constraints that exist in desired but not in live.
		// Track generated names to avoid collisions within the same diff pass.
		generatedNames := make(map[string]bool)
		for _, lc := range ld.Constraints {
			generatedNames[strings.ToLower(lc.Name)] = true
		}
		for norm, dc := range desiredExprs {
			if _, ok := liveByExpr[norm]; !ok {
				conName := dc.Name
				if conName == "" {
					// Auto-generate a unique name not already in live or previously generated.
					base := dd.Name + "_check"
					candidate := base
					for i := 1; generatedNames[strings.ToLower(candidate)]; i++ {
						candidate = fmt.Sprintf("%s%d", base, i)
					}
					conName = candidate
				}
				generatedNames[strings.ToLower(conName)] = true
				ddl := fmt.Sprintf("ALTER DOMAIN %s ADD CONSTRAINT %s CHECK (%s)", qualName, conName, dc.Expr)
				out = append(out, change{kind: plan.ChangeRawSQL, rawSQL: ddl})
			}
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
