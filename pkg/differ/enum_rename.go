package differ

// enumRename pairs an old → new value for an ALTER TYPE foo RENAME VALUE.
type enumRename struct {
	From, To string
}

// detectEnumRenames returns the inferred renames between live and desired enum
// label lists. A rename is detected when, at the SAME ordinal position, the
// live label is missing from desiredSet AND the desired label is missing from
// liveSet. Other shapes (insertion, deletion, reorder) are not treated as
// renames — the safer fallbacks (ADD VALUE + DROP-not-supported advisory) still
// run for unmatched values.
//
// Heuristic correctness: a rename preserves position, while pure adds/removes
// shift downstream positions. Walking both lists in lock-step with a single
// rename emits a valid PG12+ ALTER TYPE foo RENAME VALUE statement; multiple
// renames at non-overlapping positions also work. We bail out the moment a
// length mismatch interrupts position alignment to avoid mis-inferring on
// add-and-rename patterns.
func detectEnumRenames(desired, live []string, liveSet, desiredSet map[string]struct{}) []enumRename {
	if len(desired) == 0 || len(live) == 0 {
		return nil
	}
	// We require equal length to walk in lock-step. If lengths differ, the
	// add/remove paths handle the rest.
	if len(desired) != len(live) {
		return nil
	}
	var out []enumRename
	for i := range desired {
		if desired[i] == live[i] {
			continue
		}
		// Position mismatch — both sides must have a "lone" value to count as rename.
		_, liveValStillDesired := desiredSet[live[i]]
		_, desiredValStillLive := liveSet[desired[i]]
		if liveValStillDesired || desiredValStillLive {
			// The values appear elsewhere in the other list — this is a reorder,
			// not a rename. Don't infer.
			continue
		}
		out = append(out, enumRename{From: live[i], To: desired[i]})
	}
	return out
}
