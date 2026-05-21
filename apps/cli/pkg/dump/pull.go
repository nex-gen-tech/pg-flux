package dump

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nex-gen-tech/pg-flux/pkg/pgver"
	"github.com/nex-gen-tech/pg-flux/pkg/schema"
)

// PullOptions controls Pull.
type PullOptions struct {
	// DryRun returns the would-be SQL in PullResult.SQL without touching disk.
	DryRun bool
	// OutputDir for the quarantine file; defaults to "./schema/_pulled".
	OutputDir string
}

// PullResult summarises what Pull found / wrote.
type PullResult struct {
	ObjectCount int
	Filename    string // empty on dry-run
	SQL         string // populated on dry-run
}

// Pull renders the set difference (live \ desired) into a single quarantine
// .sql file under OutputDir. It NEVER edits existing source files — the
// quarantine pattern means users can review, move, and edit the captured
// objects manually before they enter the regular schema source set.
//
// If DryRun is true, no file is written; the SQL is returned for the caller
// to print. Otherwise a file named <timestamp>_pulled.sql is created.
func Pull(desired, live *schema.SchemaState, opts PullOptions) (*PullResult, error) {
	if opts.OutputDir == "" {
		opts.OutputDir = filepath.Join("schema", "_pulled")
	}
	report := Verify(desired, live)
	if report.Count() == 0 {
		return &PullResult{}, nil
	}

	// Build a subset of live containing only the undeclared objects, then
	// reuse the regular renderers to produce SQL.
	subset := subsetForPull(live, report)
	objs := renderAll(subset, pgver.Version{Major: 999})
	sortObjects(objs)

	var b strings.Builder
	b.WriteString("-- pg-flux pull (quarantine)\n")
	fmt.Fprintf(&b, "-- captured at %s\n", time.Now().UTC().Format(time.RFC3339))
	b.WriteString("-- review each object below; if you want pg-flux to manage it,\n")
	b.WriteString("-- move the relevant block into the appropriate source file under schema/.\n\n")
	var prevKind string
	for _, o := range objs {
		if o.Kind != prevKind {
			fmt.Fprintf(&b, "-- ---- %s ----\n\n", strings.ToUpper(o.Kind))
			prevKind = o.Kind
		}
		b.WriteString(o.SQL)
		b.WriteString("\n")
	}

	if opts.DryRun {
		return &PullResult{ObjectCount: len(objs), SQL: b.String()}, nil
	}
	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", opts.OutputDir, err)
	}
	ts := time.Now().UTC().Format("20060102_150405")
	fname := filepath.Join(opts.OutputDir, ts+"_pulled.sql")
	if err := os.WriteFile(fname, []byte(b.String()), 0o644); err != nil {
		return nil, fmt.Errorf("write %s: %w", fname, err)
	}
	return &PullResult{ObjectCount: len(objs), Filename: fname}, nil
}

// subsetForPull builds a SchemaState containing only the keys listed in the
// VerifyReport. Each kind is copied by reference from live; pg-flux is
// read-only with respect to the live state.
func subsetForPull(live *schema.SchemaState, r *VerifyReport) *schema.SchemaState {
	out := &schema.SchemaState{}
	keep := func(set []string) map[string]bool {
		m := make(map[string]bool, len(set))
		for _, k := range set {
			m[k] = true
		}
		return m
	}
	if k := keep(r.Tables); len(k) > 0 {
		out.Tables = map[string]*schema.Table{}
		for n := range k {
			if v := live.Tables[n]; v != nil {
				out.Tables[n] = v
			}
		}
	}
	if k := keep(r.Views); len(k) > 0 {
		out.Views = map[string]*schema.View{}
		for n := range k {
			if v := live.Views[n]; v != nil {
				out.Views[n] = v
			}
		}
	}
	if k := keep(r.Sequences); len(k) > 0 {
		out.Sequences = map[string]*schema.Sequence{}
		for n := range k {
			if v := live.Sequences[n]; v != nil {
				out.Sequences[n] = v
			}
		}
	}
	if k := keep(r.Indexes); len(k) > 0 {
		out.Indexes = map[string]*schema.Index{}
		for n := range k {
			if v := live.Indexes[n]; v != nil {
				out.Indexes[n] = v
			}
		}
	}
	if k := keep(r.Functions); len(k) > 0 {
		out.Functions = map[string]*schema.Function{}
		for n := range k {
			if v := live.Functions[n]; v != nil {
				out.Functions[n] = v
			}
		}
	}
	if k := keep(r.Triggers); len(k) > 0 {
		out.Triggers = map[string]*schema.Trigger{}
		for n := range k {
			if v := live.Triggers[n]; v != nil {
				out.Triggers[n] = v
			}
		}
	}
	if k := keep(r.Policies); len(k) > 0 {
		out.Policies = map[string]*schema.Policy{}
		for n := range k {
			if v := live.Policies[n]; v != nil {
				out.Policies[n] = v
			}
		}
	}
	if k := keep(r.Enums); len(k) > 0 {
		out.EnumValues = map[string][]string{}
		for n := range k {
			if v, ok := live.EnumValues[n]; ok {
				out.EnumValues[n] = v
			}
		}
	}
	if k := keep(r.Domains); len(k) > 0 {
		out.Domains = map[string]*schema.Domain{}
		for n := range k {
			if v := live.Domains[n]; v != nil {
				out.Domains[n] = v
			}
		}
	}
	if k := keep(r.CompositeTypes); len(k) > 0 {
		out.CompositeTypes = map[string]*schema.CompositeType{}
		for n := range k {
			if v := live.CompositeTypes[n]; v != nil {
				out.CompositeTypes[n] = v
			}
		}
	}
	if k := keep(r.RangeTypes); len(k) > 0 {
		out.RangeTypes = map[string]*schema.RangeType{}
		for n := range k {
			if v := live.RangeTypes[n]; v != nil {
				out.RangeTypes[n] = v
			}
		}
	}
	if k := keep(r.ForeignTables); len(k) > 0 {
		out.ForeignTables = map[string]*schema.ForeignTable{}
		for n := range k {
			if v := live.ForeignTables[n]; v != nil {
				out.ForeignTables[n] = v
			}
		}
	}
	if k := keep(r.ForeignServers); len(k) > 0 {
		out.ForeignServers = map[string]*schema.ForeignServer{}
		for n := range k {
			if v := live.ForeignServers[n]; v != nil {
				out.ForeignServers[n] = v
			}
		}
	}
	if k := keep(r.EventTriggers); len(k) > 0 {
		out.EventTriggers = map[string]*schema.EventTrigger{}
		for n := range k {
			if v := live.EventTriggers[n]; v != nil {
				out.EventTriggers[n] = v
			}
		}
	}
	if k := keep(r.Statistics); len(k) > 0 {
		out.Statistics = map[string]*schema.Statistics{}
		for n := range k {
			if v := live.Statistics[n]; v != nil {
				out.Statistics[n] = v
			}
		}
	}
	if k := keep(r.Extensions); len(k) > 0 {
		out.Extensions = map[string]*schema.Extension{}
		for n := range k {
			if v := live.Extensions[n]; v != nil {
				out.Extensions[n] = v
			}
		}
	}
	if k := keep(r.Publications); len(k) > 0 {
		out.Publications = map[string]*schema.Publication{}
		for n := range k {
			if v := live.Publications[n]; v != nil {
				out.Publications[n] = v
			}
		}
	}
	return out
}
