package src

import (
	"fmt"
	"regexp"
	"strings"
)

var reRenameFrom = regexp.MustCompile(`^\s*--\s*@renamed\s+from\s*=\s*(.+?)\s*$`)

func extractRenameFromComment(line string) (string, bool) {
	if m := reRenameFrom.FindStringSubmatch(line); m != nil {
		id, err := parseIdent(strings.TrimSpace(m[1]))
		if err != nil {
			return "", false
		}
		return id, true
	}
	return "", false
}

// rlsFromCreateTableDeparse extracts ENABLE/DISABLE and FORCE flags from a deparsed CREATE TABLE.
func rlsFromCreateTableDeparse(deparsed string) (enable, force bool) {
	l := strings.ToLower(deparsed)
	if strings.Contains(l, "enable row level security") {
		enable = true
	}
	if strings.Contains(l, "disable row level security") {
		enable = false
	}
	if strings.Contains(l, "force row level security") {
		force = true
		enable = true
	}
	if strings.Contains(l, "no force row level security") {
		force = false
	}
	return enable, force
}

func isDeprecatedTableComment(line string) bool {
	s := strings.TrimSpace(line)
	return strings.EqualFold(s, "-- @deprecated")
}

func parseIdent(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("empty identifier")
	}
	if s[0] == '"' {
		if len(s) < 2 || s[len(s)-1] != '"' {
			return "", fmt.Errorf("unterminated quoted identifier")
		}
		return s[1 : len(s)-1], nil
	}
	return strings.ToLower(s), nil
}

// lineIndex0ForByte returns 0-based line index containing byte offset.
func lineIndex0ForByte(content string, offset int) int {
	if offset < 0 {
		offset = 0
	}
	if offset > len(content) {
		offset = len(content)
	}
	line := 0
	for i, c := range content {
		if i >= offset {
			break
		}
		if c == '\n' {
			line++
		}
	}
	return line
}

// lineByIndex0 returns the raw line (no \n) for 0-based index.
func lineByIndex0(lines []string, idx int) string {
	if idx < 0 || idx >= len(lines) {
		return ""
	}
	return lines[idx]
}

// previousNonEmptyLineIndex walks back skipping blank lines, returns -1 if none.
func previousNonEmptyLineIndex(lines []string, from int) int {
	for i := from - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			return i
		}
	}
	return -1
}
