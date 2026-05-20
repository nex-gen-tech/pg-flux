package hazard

import (
	"strings"
)

// EnrichFromDDL adds advisory/blocking hazards inferred from raw DDL text (NOT VALID, VALIDATE).
func EnrichFromDDL(ddl string) []Detected {
	if ddl == "" {
		return nil
	}
	low := strings.ToLower(ddl)
	var out []Detected
	// ADD CONSTRAINT ... NOT VALID
	if strings.Contains(low, " not valid") {
		out = append(out, Detected{
			Type:     DeferredConstraintValidation,
			Severity: SeverityAdvisory,
			Message:  "Constraint added NOT VALID; run VALIDATE CONSTRAINT when ready (may scan table)",
		})
	}
	// ALTER TABLE ... VALIDATE CONSTRAINT
	if strings.Contains(low, "validate constraint") {
		out = append(out, Detected{
			Type:     ValidateConstraintScan,
			Severity: SeverityBlocking,
			Message:  "VALIDATE CONSTRAINT typically performs a full table scan",
		})
	}
	return out
}
