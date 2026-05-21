package migrate

import (
	"crypto/sha256"
	"fmt"
	"os"
	"strings"
)

// RehashResult is returned by Rehash.
type RehashResult struct {
	// NewHash is the SHA-256 hex digest written into the file.
	NewHash string
	// HadHashLine is true when the file already contained a baseline-hash line.
	HadHashLine bool
}

// ContentHashOfMigration computes the SHA-256 hex digest of the migration file
// content with the "-- pg-flux-baseline-hash: ..." line removed. This is the
// value stored by Rehash and recognised by Apply's drift check as a
// "user-accepted edit" marker that bypasses the live-DB state comparison.
func ContentHashOfMigration(content []byte) string {
	stripped := stripHashLine(string(content))
	sum := sha256.Sum256([]byte(stripped))
	return fmt.Sprintf("%x", sum)
}

// stripHashLine removes the first occurrence of the baseline-hash header line
// from the SQL text. Lines that do not contain the header are returned unchanged.
func stripHashLine(sql string) string {
	lines := strings.Split(sql, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), strings.TrimSpace(BaselineHashHeader)) {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// Rehash reads the migration file at filePath, recomputes the SHA-256 hash of
// its content (excluding the existing "-- pg-flux-baseline-hash: ..." line),
// and updates that line in the file with the new hash. The rest of the file is
// never modified.
//
// If the file contains no baseline-hash line, a warning message is returned via
// the noHashLine flag in RehashResult but no error is returned and the file is
// left untouched.
func Rehash(filePath string) (*RehashResult, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", filePath, err)
	}

	// Detect whether a hash line is present.
	hadHashLine := extractBaselineHash(content) != ""

	if !hadHashLine {
		return &RehashResult{HadHashLine: false}, nil
	}

	newHash := ContentHashOfMigration(content)

	// Replace the existing hash line with the new one, preserving surrounding content.
	updated := replaceHashLine(string(content), newHash)

	if err := os.WriteFile(filePath, []byte(updated), 0o644); err != nil {
		return nil, fmt.Errorf("write %s: %w", filePath, err)
	}

	return &RehashResult{NewHash: newHash, HadHashLine: true}, nil
}

// replaceHashLine replaces the baseline-hash value in the first matching
// "-- pg-flux-baseline-hash: ..." line with newHash. All other lines are
// preserved exactly, including whitespace and line endings.
func replaceHashLine(sql, newHash string) string {
	// Use line-by-line replacement so we never touch any other content.
	lines := strings.Split(sql, "\n")
	replaced := false
	for i, line := range lines {
		if !replaced && strings.HasPrefix(strings.TrimSpace(line), strings.TrimSpace(BaselineHashHeader)) {
			// Preserve any leading whitespace before the comment marker.
			leading := line[:strings.Index(line, "--")]
			lines[i] = leading + BaselineHashHeader + newHash
			replaced = true
		}
	}
	return strings.Join(lines, "\n")
}
