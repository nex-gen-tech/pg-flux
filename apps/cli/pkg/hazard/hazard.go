package hazard

// Type matches PRD hazard IDs (subset for v1).
type Type string

const (
	DataLoss                Type = "DATA_LOSS"
	TableLock               Type = "TABLE_LOCK"
	ConstraintScan          Type = "CONSTRAINT_SCAN"
	ColumnTypeChange        Type = "COLUMN_TYPE_CHANGE"
	IndexRebuild            Type = "INDEX_REBUILD"
	FunctionSignatureChange Type = "FUNCTION_SIGNATURE_CHANGE"
	EnumValueDrop           Type = "ENUM_VALUE_DROP"
	NotReplicaSafe          Type = "NOT_REPLICA_SAFE"
	// DeferredConstraintValidation marks ADD CONSTRAINT ... NOT VALID (validation pending).
	DeferredConstraintValidation Type = "DEFERRED_CONSTRAINT_VALIDATION"
	// ValidateConstraintScan marks ALTER ... VALIDATE CONSTRAINT (full table scan).
	ValidateConstraintScan Type = "VALIDATE_CONSTRAINT_SCAN"
	// StagedSetNotNull suggests the four-step CHECK/VALIDATE pattern for large tables.
	StagedSetNotNull Type = "STAGED_SET_NOT_NULL"
	// ColumnReorder indicates the desired column order differs from the live schema.
	// Reordering columns requires table recreation; this is advisory only.
	ColumnReorder Type = "COLUMN_REORDER"
)

// Severity distinguishes blocking vs advisory.
type Severity string

const (
	SeverityBlocking Severity = "blocking"
	SeverityAdvisory Severity = "advisory"
)

// Detected is a hazard attached to a statement.
type Detected struct {
	Type     Type
	Severity Severity
	Message  string
}

// DefaultSeverity maps hazard types in v1.
func DefaultSeverity(t Type) Severity {
	switch t {
	case DataLoss, ColumnTypeChange, ConstraintScan, NotReplicaSafe, FunctionSignatureChange, EnumValueDrop, ValidateConstraintScan:
		return SeverityBlocking
	case TableLock, IndexRebuild, DeferredConstraintValidation, StagedSetNotNull, ColumnReorder:
		return SeverityAdvisory
	default:
		return SeverityBlocking
	}
}
