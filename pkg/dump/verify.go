package dump

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/nexg/pg-flux/pkg/schema"
)

// VerifyReport lists per-kind sets of object identifiers present in live but
// missing from the desired/source state. The dump command uses this to enforce
// "everything in live must be declared in source" as a CI gate.
type VerifyReport struct {
	Tables           []string
	Views            []string
	Sequences        []string
	Indexes          []string
	Functions        []string
	Triggers         []string
	Policies         []string
	Enums            []string
	Domains          []string
	CompositeTypes   []string
	RangeTypes       []string
	ForeignTables    []string
	ForeignServers   []string
	EventTriggers    []string
	Statistics       []string
	Extensions       []string
}

// Count returns the total number of undeclared objects across all kinds.
func (r *VerifyReport) Count() int {
	return len(r.Tables) + len(r.Views) + len(r.Sequences) + len(r.Indexes) +
		len(r.Functions) + len(r.Triggers) + len(r.Policies) + len(r.Enums) +
		len(r.Domains) + len(r.CompositeTypes) + len(r.RangeTypes) +
		len(r.ForeignTables) + len(r.ForeignServers) + len(r.EventTriggers) +
		len(r.Statistics) + len(r.Extensions)
}

// WriteText renders the report as a human-readable grouped list.
func (r *VerifyReport) WriteText(w io.Writer) {
	groups := []struct {
		Name  string
		Items []string
	}{
		{"Extensions", r.Extensions},
		{"Enums", r.Enums},
		{"Domains", r.Domains},
		{"Composite types", r.CompositeTypes},
		{"Range types", r.RangeTypes},
		{"Sequences", r.Sequences},
		{"Tables", r.Tables},
		{"Indexes", r.Indexes},
		{"Views", r.Views},
		{"Functions", r.Functions},
		{"Triggers", r.Triggers},
		{"Policies", r.Policies},
		{"Event triggers", r.EventTriggers},
		{"Statistics", r.Statistics},
		{"Foreign servers", r.ForeignServers},
		{"Foreign tables", r.ForeignTables},
	}
	total := r.Count()
	if total == 0 {
		fmt.Fprintln(w, "verify: clean — every live object is declared in source.")
		return
	}
	fmt.Fprintf(w, "verify: %d undeclared live object(s):\n", total)
	for _, g := range groups {
		if len(g.Items) == 0 {
			continue
		}
		fmt.Fprintf(w, "\n  %s (%d):\n", g.Name, len(g.Items))
		for _, it := range g.Items {
			fmt.Fprintf(w, "    - %s\n", it)
		}
	}
	fmt.Fprintln(w, "\nRun `pg-flux pull` to capture these into schema/_pulled/<ts>.sql for review.")
}

// Verify computes the live\desired set difference across every supported kind.
// It does not require the live DB to contain *only* declared objects (extra
// live objects are reported, never deleted) — pg-flux is declarative-with-respect-to-source,
// not authoritative-over-live.
func Verify(desired, live *schema.SchemaState) *VerifyReport {
	r := &VerifyReport{}
	if live == nil {
		return r
	}
	if desired == nil {
		desired = &schema.SchemaState{}
	}
	missingStrings := func(d, l map[string]struct{}) []string {
		var out []string
		for k := range l {
			if _, ok := d[k]; !ok {
				out = append(out, k)
			}
		}
		sort.Strings(out)
		return out
	}
	missing := func(d, l interface{ /* generic map[string]T */ }) []string {
		// Use reflection-free explicit checks per call site below; this helper
		// is unused but kept as a placeholder to remind future devs of the pattern.
		_ = d
		_ = l
		return nil
	}
	_ = missing

	// Tables / views / sequences / indexes etc. use map[string]*T — write per-kind
	// helpers inline; generics would simplify but cost a dependency.
	for k, v := range live.Tables {
		if v == nil {
			continue
		}
		if _, ok := desired.Tables[k]; !ok {
			r.Tables = append(r.Tables, k)
		}
	}
	for k, v := range live.Views {
		if v == nil {
			continue
		}
		if _, ok := desired.Views[k]; !ok {
			r.Views = append(r.Views, k)
		}
	}
	for k, v := range live.Sequences {
		if v == nil {
			continue
		}
		if _, ok := desired.Sequences[k]; !ok {
			// Skip identity sequences — they're implicit, not user-declared.
			if isIdentitySequence(live, v) {
				continue
			}
			r.Sequences = append(r.Sequences, k)
		}
	}
	for k, v := range live.Indexes {
		if v == nil {
			continue
		}
		if _, ok := desired.Indexes[k]; !ok {
			r.Indexes = append(r.Indexes, k)
		}
	}
	for k, v := range live.Functions {
		if v == nil {
			continue
		}
		if _, ok := desired.Functions[k]; !ok {
			r.Functions = append(r.Functions, k)
		}
	}
	for k, v := range live.Triggers {
		if v == nil {
			continue
		}
		if _, ok := desired.Triggers[k]; !ok {
			r.Triggers = append(r.Triggers, k)
		}
	}
	for k, v := range live.Policies {
		if v == nil {
			continue
		}
		if _, ok := desired.Policies[k]; !ok {
			r.Policies = append(r.Policies, k)
		}
	}
	for k := range live.EnumValues {
		if _, ok := desired.EnumValues[k]; !ok {
			r.Enums = append(r.Enums, k)
		}
	}
	for k, v := range live.Domains {
		if v == nil {
			continue
		}
		if _, ok := desired.Domains[k]; !ok {
			r.Domains = append(r.Domains, k)
		}
	}
	for k, v := range live.CompositeTypes {
		if v == nil {
			continue
		}
		if _, ok := desired.CompositeTypes[k]; !ok {
			r.CompositeTypes = append(r.CompositeTypes, k)
		}
	}
	for k, v := range live.RangeTypes {
		if v == nil {
			continue
		}
		if _, ok := desired.RangeTypes[k]; !ok {
			r.RangeTypes = append(r.RangeTypes, k)
		}
	}
	for k, v := range live.ForeignTables {
		if v == nil {
			continue
		}
		if _, ok := desired.ForeignTables[k]; !ok {
			r.ForeignTables = append(r.ForeignTables, k)
		}
	}
	for k, v := range live.ForeignServers {
		if v == nil {
			continue
		}
		if _, ok := desired.ForeignServers[k]; !ok {
			r.ForeignServers = append(r.ForeignServers, k)
		}
	}
	for k, v := range live.EventTriggers {
		if v == nil {
			continue
		}
		if _, ok := desired.EventTriggers[k]; !ok {
			r.EventTriggers = append(r.EventTriggers, k)
		}
	}
	for k, v := range live.Statistics {
		if v == nil {
			continue
		}
		if _, ok := desired.Statistics[k]; !ok {
			r.Statistics = append(r.Statistics, k)
		}
	}
	for k, v := range live.Extensions {
		if v == nil {
			continue
		}
		if _, ok := desired.Extensions[k]; !ok {
			r.Extensions = append(r.Extensions, k)
		}
	}
	sortAll(r)
	// Drop _ = missingStrings import — keep the helper available for tests/future use.
	_ = missingStrings
	return r
}

func sortAll(r *VerifyReport) {
	for _, sl := range [][]string{
		r.Tables, r.Views, r.Sequences, r.Indexes, r.Functions, r.Triggers,
		r.Policies, r.Enums, r.Domains, r.CompositeTypes, r.RangeTypes,
		r.ForeignTables, r.ForeignServers, r.EventTriggers, r.Statistics, r.Extensions,
	} {
		sort.Strings(sl)
	}
	_ = strings.Compare // keep strings import slot stable
}
