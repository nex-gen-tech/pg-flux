package migrate

import (
	"bufio"
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nexg/pg-flux/pkg/hashstate"
	"github.com/nexg/pg-flux/pkg/inspector"
)

// BaselineDriftError is returned by Apply when the live DB no longer matches
// the state recorded in a pending migration's "pg-flux-baseline-hash" header.
// Pass --force-after-drift to bypass.
type BaselineDriftError struct {
	Filename       string
	ExpectedHash   string
	LiveHash       string
	HasBaseline    bool // false when the file was written by an older pg-flux without the header
}

func (e *BaselineDriftError) Error() string {
	if !e.HasBaseline {
		// Should never be returned in this case; we only construct one on mismatch.
		return fmt.Sprintf("baseline drift for %s (missing header)", e.Filename)
	}
	return fmt.Sprintf(
		"refusing to apply %s: live database state has drifted since this migration was generated "+
			"(expected baseline=%s, live=%s). "+
			"Someone or something modified the schema outside pg-flux between generate and apply. "+
			"Re-run `pg-flux migrate generate` to rebase the migration, or pass --force-after-drift to apply anyway",
		e.Filename, shortHash(e.ExpectedHash), shortHash(e.LiveHash))
}

func shortHash(h string) string {
	if len(h) > 12 {
		return h[:12] + "…"
	}
	return h
}

// extractBaselineHash scans the first ~20 lines of a migration file for the
// "-- pg-flux-baseline-hash: <hex>" header written by buildMigrationSQL.
// Returns empty string when the header is absent (older file or hand-written).
func extractBaselineHash(content []byte) string {
	sc := bufio.NewScanner(strings.NewReader(string(content)))
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for i := 0; sc.Scan() && i < 20; i++ {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, strings.TrimSpace(BaselineHashHeader)) {
			// Header is "-- pg-flux-baseline-hash: <hex>".
			return strings.TrimSpace(strings.TrimPrefix(line, strings.TrimSpace(BaselineHashHeader)))
		}
		// Stop scanning once non-comment content begins; baseline is always in the header block.
		if line != "" && !strings.HasPrefix(line, "--") {
			break
		}
	}
	return ""
}

// checkBaselineDrift inspects the live DB and compares its hash against the
// baseline embedded in the FIRST pending migration. Returns *BaselineDriftError
// on mismatch, nil on match or when there is no header to compare against.
//
// We only check the first pending file: subsequent files were generated against
// state-after-previous-file, which we don't materialize, so their baseline cannot
// be meaningfully compared against current live without reapplying intermediates.
func checkBaselineDrift(ctx context.Context, pool *pgxpool.Pool, schemas []string, firstPendingName string, firstPendingContent []byte) error {
	expected := extractBaselineHash(firstPendingContent)
	if expected == "" {
		// No baseline header — older file, hand-written, or generated before this feature.
		// Skip silently rather than fail; this preserves backward compatibility.
		return nil
	}
	live, err := inspector.Inspect(ctx, pool, inspector.Options{Schemas: schemas})
	if err != nil {
		return fmt.Errorf("inspect live for drift check: %w", err)
	}
	liveHash := hashstate.OfSchemaState(live)
	if liveHash == expected {
		return nil
	}
	return &BaselineDriftError{
		Filename:     firstPendingName,
		ExpectedHash: expected,
		LiveHash:     liveHash,
		HasBaseline:  true,
	}
}
