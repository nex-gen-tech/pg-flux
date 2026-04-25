package hazard

import "fmt"

// StagedSetNotNullSteps is the four-step, low-lock pattern for NOT NULL on large tables (FR-06 sketch).
// pg-flux does not emit these automatically; they are a reference for operators and future automation.
func StagedSetNotNullSteps(qualifiedTable, conName, col, checkExpr string) []string {
	if conName == "" {
		conName = "not_null_chk"
	}
	return []string{
		"BEGIN",
		"SET lock_timeout = '2s';",
		"SET statement_timeout = '0';",
		fmt.Sprintf("ALTER TABLE %s ADD CONSTRAINT %s CHECK ((%s)) NOT VALID", qualifiedTable, conName, checkExpr),
		fmt.Sprintf("ALTER TABLE %s VALIDATE CONSTRAINT %s", qualifiedTable, conName),
		fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s SET NOT NULL", qualifiedTable, col),
		fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT IF EXISTS %s", qualifiedTable, conName),
		"COMMIT",
	}
}
