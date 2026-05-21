package differ

import (
	"fmt"
	"strings"

	"github.com/nex-gen-tech/pg-flux/pkg/hazard"
	"github.com/nex-gen-tech/pg-flux/pkg/plan"
	"github.com/nex-gen-tech/pg-flux/pkg/schema"
)

// diffEnums performs structured enum-type diffing using SchemaState.Enums.
//
// Four cases are handled:
//
//  1. Enum in desired only (new): emit CREATE TYPE ... AS ENUM (...) wrapped in a
//     DO $pgflux$ BEGIN ... EXCEPTION WHEN duplicate_object THEN NULL; END $pgflux$;
//     block for idempotency. Sorted as ChangeCreateType so it lands before any
//     CREATE TABLE that references the type.
//
//  2. Enum in both; desired has values not in live: emit
//     ALTER TYPE ... ADD VALUE IF NOT EXISTS '...' for each new label. Positions
//     are preserved with BEFORE/AFTER when a neighbour already exists.
//
//  3. Enum in both; live has values not in desired: emit a DATA_LOSS blocking
//     hazard. PostgreSQL does not support ALTER TYPE DROP VALUE — the type must
//     be manually recreated. No DDL is emitted so the migration stays safe.
//
//  4. Enum in live only (dropped from desired): emit a DATA_LOSS blocking hazard
//     advisory with a DROP TYPE IF EXISTS statement. The administrator must verify
//     no column references the type before applying.
func diffEnums(d, l *schema.SchemaState) []change {
	var out []change
	if d == nil {
		d = &schema.SchemaState{}
	}
	if l == nil {
		l = &schema.SchemaState{}
	}

	// Case 1 & 2 & 3: iterate over desired enums.
	for k, de := range d.Enums {
		if de == nil {
			continue
		}
		le := l.Enums[k]
		if le == nil {
			// Case 1: enum exists in desired but not in live — CREATE.
			out = append(out, change{
				kind:   plan.ChangeCreateType,
				rawSQL: buildCreateEnumSQL(de),
			})
			continue
		}
		// Case 2 & 3: enum exists in both — compare values.
		out = append(out, diffEnumValues(de, le)...)
	}

	// Case 4: enum in live but not in desired — DROP hazard.
	for k, le := range l.Enums {
		if le == nil {
			continue
		}
		if d.Enums[k] != nil {
			continue // already handled above
		}
		qual := ident(le.Schema) + "." + ident(le.Name)
		out = append(out, change{
			kind:   plan.ChangeRawSQL,
			rawSQL: fmt.Sprintf("DROP TYPE IF EXISTS %s CASCADE", qual),
			extraHazards: []hazard.Detected{{
				Type:     hazard.DataLoss,
				Severity: hazard.SeverityBlocking,
				Message: fmt.Sprintf(
					"enum type %s.%s exists in live DB but is not declared in source — dropping it will cascade to all columns and functions that reference the type",
					le.Schema, le.Name,
				),
			}},
		})
	}
	return out
}

// buildCreateEnumSQL returns an idempotent CREATE TYPE ... AS ENUM statement
// wrapped in a DO block to avoid "type already exists" errors on re-apply.
func buildCreateEnumSQL(e *schema.EnumType) string {
	if e == nil {
		return ""
	}
	qual := ident(e.Schema) + "." + ident(e.Name)
	var sb strings.Builder
	fmt.Fprintf(&sb, "DO $pgflux$ BEGIN CREATE TYPE %s AS ENUM (", qual)
	for i, v := range e.Values {
		if i > 0 {
			sb.WriteString(", ")
		}
		fmt.Fprintf(&sb, "'%s'", strings.ReplaceAll(v, "'", "''"))
	}
	sb.WriteString("); EXCEPTION WHEN duplicate_object THEN NULL; END $pgflux$;")
	return sb.String()
}

// diffEnumValues compares desired vs live enum value lists and returns the changes
// needed to bring live in line with desired. It handles renames (position-stable
// label changes), value additions (ADD VALUE IF NOT EXISTS), and value removals
// (DATA_LOSS hazard — PG has no DROP VALUE).
func diffEnumValues(d, l *schema.EnumType) []change {
	if d == nil || l == nil {
		return nil
	}
	liveSet := make(map[string]struct{}, len(l.Values))
	for _, v := range l.Values {
		liveSet[v] = struct{}{}
	}
	desiredSet := make(map[string]struct{}, len(d.Values))
	for _, v := range d.Values {
		desiredSet[v] = struct{}{}
	}

	qual := d.Schema + "." + d.Name
	var out []change

	// Detect renames before reporting removals as data-loss. A rename is a
	// position-stable label swap: live[i] not in desired AND desired[i] not in live.
	renamed := detectEnumRenames(d.Values, l.Values, liveSet, desiredSet)
	for _, r := range renamed {
		out = append(out, change{
			kind:   plan.ChangeRawSQL,
			rawSQL: fmt.Sprintf("ALTER TYPE %s RENAME VALUE '%s' TO '%s'", qual, r.From, r.To),
		})
		// Update the live tracking so subsequent removal/addition checks ignore
		// the renamed pair.
		delete(liveSet, r.From)
		liveSet[r.To] = struct{}{}
	}

	// Data-loss: live has values not in desired (and not renamed away).
	for _, lv := range l.Values {
		if _, ok := desiredSet[lv]; ok {
			continue
		}
		if _, stillLive := liveSet[lv]; !stillLive {
			continue // accounted for by a rename
		}
		out = append(out, change{
			kind: plan.ChangeRawSQL,
			extraHazards: []hazard.Detected{{
				Type:     hazard.DataLoss,
				Severity: hazard.SeverityBlocking,
				Message: fmt.Sprintf(
					"enum type %s: value '%s' exists in live DB but not in desired schema — PostgreSQL does not support ALTER TYPE DROP VALUE; the type must be manually recreated to remove the value",
					qual, lv,
				),
			}},
		})
	}

	// Add new values in desired order with positional BEFORE/AFTER hints.
	for i, v := range d.Values {
		if _, ok := liveSet[v]; ok {
			continue // already exists (possibly after rename)
		}
		addSQL := "ALTER TYPE " + qual + " ADD VALUE IF NOT EXISTS '" + strings.ReplaceAll(v, "'", "''") + "'"
		if i < len(d.Values)-1 {
			next := d.Values[i+1]
			if _, exists := liveSet[next]; exists {
				addSQL += " BEFORE '" + strings.ReplaceAll(next, "'", "''") + "'"
			}
		} else if i > 0 {
			prev := d.Values[i-1]
			if _, exists := liveSet[prev]; exists {
				addSQL += " AFTER '" + strings.ReplaceAll(prev, "'", "''") + "'"
			}
		}
		out = append(out, change{kind: plan.ChangeRawSQL, rawSQL: addSQL})
	}
	return out
}
