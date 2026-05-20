package differ

import (
	"fmt"
	"sort"
	"strings"

	"github.com/nexg/pg-flux/pkg/plan"
	"github.com/nexg/pg-flux/pkg/schema"
)

// MassDropError is returned by Diff when the plan would drop more than
// MassDropThresholdPct of live objects (tables + views + sequences combined)
// and AllowMassDrop is not set. It guards against the empty-schema footgun:
// pointing pg-flux at an empty --schema directory would otherwise wipe a
// non-empty live DB.
type MassDropError struct {
	LiveCount    int      // total live objects (tables+views+sequences) at risk
	DropCount    int      // proposed drops in that set
	PercentDrop  float64  // 0..100
	ThresholdPct float64  // configured threshold (default 25)
	Names        []string // up to 10 example "kind schema.name" entries
}

func (e *MassDropError) Error() string {
	preview := strings.Join(e.Names, ", ")
	if len(e.Names) >= 10 {
		preview += ", ..."
	}
	return fmt.Sprintf(
		"refusing to plan: %d of %d live objects (%.0f%%) would be dropped, "+
			"exceeding the %.0f%% mass-drop threshold; "+
			"if this is intentional, re-run with --allow-mass-drop. Examples: %s",
		e.DropCount, e.LiveCount, e.PercentDrop, e.ThresholdPct, preview,
	)
}

// checkMassDrop returns a MassDropError when DROPs cross the configured threshold.
// Called from Diff() after the change list is built but before statements are
// emitted, so the caller sees a clear error and zero side effects.
//
// Counts:
//   - Live objects considered "at risk" = tables + (non-materialized) views + sequences.
//     Materialized views are included as tables in PG terms but we count them in views.
//   - Drops = changes with kind in {DropTable, DropView, DropViewEarly, DropSequence}.
//
// Thresholds:
//   - Wipe (live > 0 AND drop count == live count): always trips, even at low absolute numbers.
//   - Mass (drop count / live count > thresholdPct/100): trips above the percentage.
func checkMassDrop(changes []change, live *schema.SchemaState, allow bool, thresholdPct float64) error {
	if allow {
		return nil
	}
	if live == nil {
		return nil
	}
	if thresholdPct <= 0 {
		thresholdPct = 25
	}
	liveCount := 0
	for _, t := range live.Tables {
		if t != nil {
			liveCount++
		}
	}
	for _, v := range live.Views {
		if v != nil {
			liveCount++
		}
	}
	for _, s := range live.Sequences {
		if s != nil {
			liveCount++
		}
	}
	if liveCount == 0 {
		return nil
	}
	var names []string
	dropCount := 0
	for _, c := range changes {
		switch c.kind {
		case plan.ChangeDropTable:
			dropCount++
			if len(names) < 10 {
				names = append(names, "TABLE "+c.sch+"."+c.tbl)
			}
		case plan.ChangeDropView, plan.ChangeDropViewEarly:
			dropCount++
			if len(names) < 10 && c.v != nil {
				names = append(names, "VIEW "+c.v.Schema+"."+c.v.Name)
			}
		case plan.ChangeDropSequence:
			dropCount++
			if len(names) < 10 && c.dropSeq != "" {
				names = append(names, "SEQUENCE "+c.dropSeq)
			}
		}
	}
	if dropCount == 0 {
		return nil
	}
	sort.Strings(names)
	pct := 100.0 * float64(dropCount) / float64(liveCount)
	// Trip on either total wipe of a non-empty DB or exceeding the threshold.
	if dropCount == liveCount || pct > thresholdPct {
		return &MassDropError{
			LiveCount:    liveCount,
			DropCount:    dropCount,
			PercentDrop:  pct,
			ThresholdPct: thresholdPct,
			Names:        names,
		}
	}
	return nil
}
