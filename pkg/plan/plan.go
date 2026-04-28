package plan

import "github.com/nexg/pg-flux/pkg/hazard"

// ChangeType enumerates diff operations.
type ChangeType string

const (
	ChangeCreateTable    ChangeType = "CREATE_TABLE"
	ChangeDropTable      ChangeType = "DROP_TABLE"
	ChangeRenameTable    ChangeType = "RENAME_TABLE"
	ChangeAddColumn      ChangeType = "ADD_COLUMN"
	ChangeDropColumn     ChangeType = "DROP_COLUMN"
	ChangeRenameColumn   ChangeType = "RENAME_COLUMN"
	ChangeAlterColumn    ChangeType = "ALTER_COLUMN"
	ChangeToggleRLS      ChangeType = "TOGGLE_RLS"
	ChangeCreateIndex    ChangeType = "CREATE_INDEX"
	ChangeDropIndex      ChangeType = "DROP_INDEX"
	ChangeCreateFunction ChangeType = "CREATE_FUNCTION"
	ChangeDropFunction   ChangeType = "DROP_FUNCTION"
	ChangeCreatePolicy   ChangeType = "CREATE_POLICY"
	ChangeDropPolicy     ChangeType = "DROP_POLICY"
	ChangeAddConstraint  ChangeType = "ADD_TABLE_CONSTRAINT"
	ChangeDropConstraint ChangeType = "DROP_TABLE_CONSTRAINT"
	ChangeCreateView     ChangeType = "CREATE_VIEW"
	ChangeDropView       ChangeType = "DROP_VIEW"
	// ChangeDropViewEarly is a DROP VIEW emitted before ALTER COLUMN TYPE to unblock type changes.
	// It renders identically to ChangeDropView but sorts with higher priority (before ALTER_COLUMN_TYPE).
	ChangeDropViewEarly  ChangeType = "DROP_VIEW_EARLY"
	ChangeCreateSequence ChangeType = "CREATE_SEQUENCE"
	ChangeDropSequence   ChangeType = "DROP_SEQUENCE"
	ChangeCreateTrigger  ChangeType = "CREATE_TRIGGER"
	ChangeDropTrigger    ChangeType = "DROP_TRIGGER"
	// ChangeCreateType covers CREATE TYPE and CREATE DOMAIN — sorted early so tables can reference the type.
	ChangeCreateType ChangeType = "CREATE_TYPE"
	// ChangeRawSQL is pass-through DDL (e.g. partition attach) not represented as first-class objects.
	ChangeRawSQL ChangeType = "RAW_DDL"
	ChangeCreateExtension ChangeType = "CREATE_EXTENSION"
	ChangeDropExtension   ChangeType = "DROP_EXTENSION"
	// ChangeUpdateExtension runs ALTER EXTENSION ... UPDATE TO (version pin / upgrade).
	ChangeUpdateExtension ChangeType = "UPDATE_EXTENSION"
)

// Statement is one DDL step in the execution plan.
type Statement struct {
	ID                 int
	DDL                string
	OpType             string
	Object             string
	Column             string // set for column-level operations (e.g. SET NOT NULL)
	IsConcurrent       bool
	LockTimeoutMS      int
	StatementTimeoutMS int
	Hazards            []hazard.Detected
	BlockingHazards    bool
	// ObjectKind refines the object for JSON consumers (e.g. aggregate, window, function).
	ObjectKind string `json:"object_kind,omitempty"`
}

// ExecutionPlan is the ordered list of DDL to run.
type ExecutionPlan struct {
	Statements []Statement
	Hazards    []hazard.Detected
}

// HasBlockingHazards returns true if any statement has unmitigated blocking hazards.
func (p *ExecutionPlan) HasBlockingHazards(allowed map[hazard.Type]bool) bool {
	if p == nil {
		return false
	}
	for _, s := range p.Statements {
		for _, h := range s.Hazards {
			if h.Severity == hazard.SeverityAdvisory {
				continue
			}
			ack := allowed != nil && allowed[h.Type]
			if !ack {
				return true
			}
		}
	}
	return false
}
