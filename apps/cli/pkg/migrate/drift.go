package migrate

import (
	"bufio"
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nex-gen-tech/pg-flux/pkg/hashstate"
	"github.com/nex-gen-tech/pg-flux/pkg/inspector"
)

// BaselineDriftError is returned by Apply when the live DB no longer matches
// the state recorded in a pending migration's "pg-flux-baseline-hash" header.
// Pass --force-after-drift to bypass.
type BaselineDriftError struct {
	Filename     string
	ExpectedHash string
	LiveHash     string
	HasBaseline  bool // false when the file was written by an older pg-flux without the header
	// OutOfOrder is true when the pending migration sorts before the last applied
	// migration by filename (timestamp). This is the typical fingerprint of
	// parallel-branch development rather than manual out-of-band schema changes.
	OutOfOrder bool
}

func (e *BaselineDriftError) Error() string {
	if !e.HasBaseline {
		return fmt.Sprintf("baseline drift for %s (missing header)", e.Filename)
	}
	if e.OutOfOrder {
		return fmt.Sprintf(
			"refusing to apply %s: this migration was generated before other changes were applied "+
				"(expected baseline=%s, live=%s).\n"+
				"This is a parallel-development conflict: two branches were developed against the same\n"+
				"schema state and another branch was deployed first. To fix:\n"+
				"  1. Pull the latest migrations from your main branch.\n"+
				"  2. Run `pg-flux migrate apply` on your local DB to bring it up to date.\n"+
				"  3. Run `pg-flux migrate rebase` to regenerate this migration on top of current state.\n"+
				"  4. Commit the updated migration file and re-open your PR.\n"+
				"Or pass --force-after-drift to skip this check (only safe when you have reviewed the DDL).",
			e.Filename, shortHash(e.ExpectedHash), shortHash(e.LiveHash))
	}
	return fmt.Sprintf(
		"refusing to apply %s: live database state has drifted since this migration was generated "+
			"(expected baseline=%s, live=%s).\n"+
			"The schema was modified outside pg-flux between generate and apply.\n"+
			"Re-run `pg-flux migrate generate` to rebase the migration, or pass --force-after-drift to apply anyway.",
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
//
// A file that was updated by "migrate rehash" stores a SHA-256 of its own
// content (minus the hash line) rather than a live-DB state hash. That value
// will never match hashstate.OfSchemaState(live), so we also accept it when
// ContentHashOfMigration(content) matches — indicating the user reviewed and
// accepted the edit.
//
// isOutOfOrder signals that this pending file has an earlier timestamp than the
// last applied migration — the hallmark of parallel-branch development. The error
// message is tailored accordingly.
func checkBaselineDrift(ctx context.Context, pool *pgxpool.Pool, schemas []string, firstPendingName string, firstPendingContent []byte, isOutOfOrder bool) error {
	expected := extractBaselineHash(firstPendingContent)
	if expected == "" {
		// No baseline header — older file, hand-written, or generated before this feature.
		// Skip silently rather than fail; this preserves backward compatibility.
		return nil
	}

	// Content-hash acceptance: "migrate rehash" writes ContentHashOfMigration
	// into the baseline-hash line to signal that the user reviewed and accepted
	// the manual edits.  No DB inspection is needed in this case.
	if expected == ContentHashOfMigration(firstPendingContent) {
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
		OutOfOrder:   isOutOfOrder,
	}
}
