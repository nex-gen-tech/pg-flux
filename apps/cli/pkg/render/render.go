package render

import (
	"encoding/json"
	"io"
	"strings"
	"time"

	"github.com/nexg/pg-flux/pkg/hazard"
	"github.com/nexg/pg-flux/pkg/plan"
)

// PlanJSON is the structured output for `plan --format=json` (PRD FR-09 subset).
// Hazard type strings and operation_type values are extensible: new entries may appear;
// clients should treat unknown values as opaque.
type PlanJSON struct {
	Version            string         `json:"version"`
	GeneratedAt        string         `json:"generated_at"`
	SourceSchemaHash   string         `json:"source_schema_hash"`
	LiveSchemaHash     string         `json:"live_schema_hash"`
	HasBlockingHazards bool           `json:"has_blocking_hazards"`
	Statements         []StatementRow `json:"statements"`
}

// HazardRow is a single hazard annotation (Type is the stable string id, e.g. DATA_LOSS).
type HazardRow struct {
	Type     string `json:"type"`
	Severity string `json:"severity"`
	Message  string `json:"message,omitempty"`
}

// StatementRow maps one execution step.
type StatementRow struct {
	ID                 int         `json:"id"`
	DDL                string      `json:"ddl"`
	OperationType      string      `json:"operation_type"`
	ObjectName         string      `json:"object_name"`
	IsConcurrent       bool        `json:"is_concurrent"`
	LockTimeoutMS      int         `json:"lock_timeout_ms"`
	StatementTimeoutMS int         `json:"statement_timeout_ms"`
	BlockingHazards    bool        `json:"blocking_hazards"`
	ObjectKind         string      `json:"object_kind,omitempty"`
	Hazards            []HazardRow `json:"hazards,omitempty"`
}

// DriftJSON is the output for `drift --format=json`.
type DriftJSON struct {
	IsDrift     bool         `json:"is_drift"`
	Differences []Difference `json:"differences"`
}

// Difference is one detected drift.
type Difference struct {
	ObjectType string `json:"object_type"`
	ObjectName string `json:"object_name"`
	ChangeType string `json:"change_type"`
	Details    string `json:"details"`
}

// PlanToJSON encodes a plan to JSON.
func PlanToJSON(w io.Writer, p *plan.ExecutionPlan, srcHash, liveHash string, allow map[hazard.Type]bool) error {
	if p == nil {
		p = &plan.ExecutionPlan{}
	}
	blocking := p.HasBlockingHazards(allow)
	rows := make([]StatementRow, 0, len(p.Statements))
	for _, s := range p.Statements {
		var hz []HazardRow
		for _, h := range s.Hazards {
			hz = append(hz, HazardRow{Type: string(h.Type), Severity: string(h.Severity), Message: h.Message})
		}
		rows = append(rows, StatementRow{
			ID: s.ID, DDL: s.DDL, OperationType: s.OpType, ObjectName: s.Object,
			IsConcurrent: s.IsConcurrent, LockTimeoutMS: s.LockTimeoutMS,
			StatementTimeoutMS: s.StatementTimeoutMS, BlockingHazards: s.BlockingHazards,
			ObjectKind: s.ObjectKind, Hazards: hz,
		})
	}
	enc := PlanJSON{
		Version: "1.0.0", GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		SourceSchemaHash: srcHash, LiveSchemaHash: liveHash,
		HasBlockingHazards: blocking, Statements: rows,
	}
	b, err := json.MarshalIndent(enc, "", "  ")
	if err != nil {
		return err
	}
	_, err = w.Write(append(b, '\n'))
	return err
}

// DriftToJSON writes drift report.
func DriftToJSON(w io.Writer, d DriftJSON) error {
	b, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	_, err = w.Write(append(b, '\n'))
	return err
}

// ParseAllowHazards converts comma list to set.
func ParseAllowHazards(s string) map[hazard.Type]bool {
	if s == "" {
		return nil
	}
	m := make(map[hazard.Type]bool)
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			m[hazard.Type(p)] = true
		}
	}
	return m
}
