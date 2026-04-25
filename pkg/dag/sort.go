package dag

import "github.com/nexg/pg-flux/pkg/plan"

// priority orders kinds for creation-style DDL.
var createOrder = map[string]int{
	"CREATE_EXTENSION":          0,
	"UPDATE_EXTENSION":          1,
	"DROP_EXTENSION":            99,
	"RENAME_TABLE":              0,
	"RENAME_TABLE ":             0,
	"RENAME_COLUMN":             1,
	"CREATE_TYPE":               1,
	"CREATE_DOMAIN":             1,
	"CREATE_TABLE":              2,
	"ADD_COLUMN":                3,
	"CREATE_FUNCTION":           7,
	"TOGGLE_RLS":                8,
	"TOGGLE_RLS_FORCE":          8,
	"TOGGLE_RLS_NOFORCE":        8,
	"VALIDATE_TABLE_CONSTRAINT": 6,
	"CREATE_AGGREGATE":          7,
	"CREATE_WINDOW_FUNCTION":    7,
	"CREATE_POLICY":             9,
	"CREATE_INDEX":              20,
	"ALTER_DEFAULT":             4,
	"ALTER_COLUMN_TYPE":         4,
	"SET_NOT_NULL":              5,
	"DROP_NOT_NULL":             5,
	"DROP_POLICY":               15,
	"DROP_INDEX":                18,
	"DROP_FUNCTION":             19,
	"DROP_TABLE":                100,
	"RAW_DDL":                   200,
	"ADD_COLUMN!":               3,
	"ADD_TABLE_CONSTRAINT":      6,
	"CREATE_SEQUENCE":           5,
	"CREATE_VIEW":               35,
	"CREATE_TRIGGER":            14,
	"CREATE_MATERIALIZED_VIEW":  35,
	"DROP_VIEW":                 16,
	"DROP_SEQUENCE":             17,
	"DROP_TRIGGER":              15,
	"DROP_TABLE_CONSTRAINT":     4,
}

// TopoSort orders statements using a dependency graph (objects referenced vs defined) with
// OpType score as secondary ordering; returns ErrDependencyCycle if the graph is cyclic.
func TopoSort(in []plan.Statement) ([]plan.Statement, error) {
	return TopologicalSortStatements(in)
}

func score(op string) int {
	if n, ok := createOrder[op]; ok {
		return n
	}
	return 50
}

// OpTypeScore returns the DAG sort priority (lower = earlier) for a plan operation type.
// Exported for the differ so change lists can be ordered deterministically before buildStatements.
func OpTypeScore(op string) int { return score(op) }

// checkCycle is retained for tests that import the name; real detection uses ErrDependencyCycle from depgraph.
func checkCycle(stmts []plan.Statement) error {
	_, err := TopologicalSortStatements(stmts)
	return err
}
