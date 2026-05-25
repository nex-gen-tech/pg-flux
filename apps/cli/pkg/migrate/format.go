package migrate

import (
	"fmt"
	"os"
	"strings"

	"github.com/nex-gen-tech/pg-flux/pkg/plan"
)

// SplitUpDown splits a combined migration file into its Up and Down sections.
// Returns (upSQL, downSQL, isCombined).
// If the file contains no "-- +migrate Up" marker it is treated as a legacy
// forward-only file: upSQL=full content, downSQL="", isCombined=false.
// The markers are case-insensitive and may have trailing comments.
func SplitUpDown(content []byte) (upSQL, downSQL string, isCombined bool) {
	lines := strings.Split(string(content), "\n")

	const markerUp = "-- +migrate up"
	const markerDown = "-- +migrate down"

	hasUp := false
	for _, line := range lines {
		trimmed := strings.ToLower(strings.TrimSpace(line))
		if trimmed == markerUp || strings.HasPrefix(trimmed, markerUp+" ") {
			hasUp = true
			break
		}
	}

	if !hasUp {
		return string(content), "", false
	}

	type section int
	const (
		sectionNone section = iota
		sectionUp
		sectionDown
	)

	var upLines, downLines []string
	cur := sectionNone

	for _, line := range lines {
		trimmed := strings.ToLower(strings.TrimSpace(line))
		if trimmed == markerUp || strings.HasPrefix(trimmed, markerUp+" ") {
			cur = sectionUp
			continue
		}
		if trimmed == markerDown || strings.HasPrefix(trimmed, markerDown+" ") {
			cur = sectionDown
			continue
		}
		switch cur {
		case sectionUp:
			upLines = append(upLines, line)
		case sectionDown:
			downLines = append(downLines, line)
		}
	}

	return trimBlankLines(upLines), trimBlankLines(downLines), true
}

// trimBlankLines joins lines and trims leading/trailing blank lines.
func trimBlankLines(lines []string) string {
	joined := strings.Join(lines, "\n")
	// trim leading blank lines
	for strings.HasPrefix(joined, "\n") {
		joined = joined[1:]
	}
	// trim trailing blank lines
	for strings.HasSuffix(joined, "\n\n") {
		joined = joined[:len(joined)-1]
	}
	return strings.TrimRight(joined, "\n")
}

// WriteCombinedFile rewrites the forward migration file (res.Filename) as a
// combined up/down file. The forward SQL is placed after "-- +migrate Up" and
// the auto-generated undo SQL after "-- +migrate Down". Returns the path.
func WriteCombinedFile(res *GenerateResult) (string, error) {
	if res == nil || res.Filename == "" {
		return "", fmt.Errorf("no filename in GenerateResult")
	}
	undoSQL := GenerateUndoSQL(&plan.ExecutionPlan{Statements: res.Statements})
	combined := "-- +migrate Up\n\n" + res.SQL + "\n\n-- +migrate Down\n\n" + undoSQL
	if err := os.WriteFile(res.Filename, []byte(combined), 0o644); err != nil {
		return "", fmt.Errorf("write combined file: %w", err)
	}
	return res.Filename, nil
}
