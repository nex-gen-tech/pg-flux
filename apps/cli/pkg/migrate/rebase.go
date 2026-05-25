package migrate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nex-gen-tech/pg-flux/pkg/differ"
	"github.com/nex-gen-tech/pg-flux/pkg/hashstate"
	"github.com/nex-gen-tech/pg-flux/pkg/inspector"
	"github.com/nex-gen-tech/pg-flux/pkg/schema"
)

// RebaseOptions controls rebase behaviour.
type RebaseOptions struct {
	// MigrationsDir is the folder containing migration .sql files.
	MigrationsDir string
	// TrackingSchema is the schema used for the migrations tracking table (default: _pgflux).
	TrackingSchema string
	// Schemas is the list of PostgreSQL schemas to inspect (default: ["public"]).
	Schemas []string
	// AllowHazards is passed through to the differ.
	AllowHazards string
	// Differ holds extra differ options (auto-rewrites, thresholds, etc.).
	Differ differ.Options
}

// RebaseResult describes what rebase did.
type RebaseResult struct {
	// Rebased is the list of migration files that were regenerated.
	Rebased []string
	// Unchanged is the list of pending migration files whose DDL did not change.
	Unchanged []string
}

// Rebase regenerates all pending (unapplied) migration files against the current
// live database state. It is the `migrate apply` counterpart to `git rebase`:
// when two branches were developed in parallel and one has already been applied,
// rebase regenerates the other branch's migrations so they apply cleanly on top.
//
// Each rebased file keeps its original filename (timestamp + label) so ordering
// relative to already-applied migrations is preserved. Only the SQL content and
// the embedded baseline-hash are updated.
//
// After rebase, run `migrate apply` to apply the regenerated migrations, then
// commit the updated files and push your PR.
func Rebase(
	ctx context.Context,
	pool *pgxpool.Pool,
	desired *schema.SchemaState,
	opts RebaseOptions,
) (*RebaseResult, error) {
	if opts.TrackingSchema == "" {
		opts.TrackingSchema = defaultTrackingSchema
	}
	if len(opts.Schemas) == 0 {
		opts.Schemas = []string{"public"}
	}

	// Load the list of applied migrations.
	applied, err := AppliedSet(ctx, pool, opts.TrackingSchema)
	if err != nil {
		return nil, fmt.Errorf("query applied migrations: %w", err)
	}

	// Collect pending (unapplied) migration files.
	allFiles, err := migrationFiles(opts.MigrationsDir)
	if err != nil {
		return nil, err
	}
	var pending []string
	for _, f := range allFiles {
		if _, done := applied[filepath.Base(f)]; !done {
			pending = append(pending, f)
		}
	}
	if len(pending) == 0 {
		return &RebaseResult{}, nil
	}

	// Inspect the current live state (which includes any other branches' applied changes).
	live, err := inspector.Inspect(ctx, pool, inspector.Options{Schemas: opts.Schemas})
	if err != nil {
		return nil, fmt.Errorf("inspect live schema: %w", err)
	}

	// Diff desired schema against current live state.
	diffResult, err := differ.Diff(desired, live, opts.Differ)
	if err != nil {
		return nil, fmt.Errorf("diff: %w", err)
	}

	res := &RebaseResult{}

	if len(diffResult.Plan.Statements) == 0 {
		// Nothing left to do: the live DB already matches the desired schema.
		// Mark all pending files as unchanged (they will be no-ops when applied).
		for _, f := range pending {
			res.Unchanged = append(res.Unchanged, filepath.Base(f))
		}
		return res, nil
	}

	// Compute the new migration SQL using the current live state as baseline.
	newBaselineHash := hashstate.OfSchemaState(live)
	newSQL := buildMigrationSQL(diffResult.Plan, newBaselineHash)

	// Multiple pending files: we collapse them into the FIRST one (earliest
	// timestamp) and remove the rest. This preserves the original generation
	// order while producing a single clean migration that applies on top of
	// the current live state.
	firstPending := pending[0]
	firstBase := filepath.Base(firstPending)

	// Read current content to check if anything actually changed.
	oldContent, err := os.ReadFile(firstPending)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", firstPending, err)
	}

	if string(oldContent) != newSQL {
		if err := os.WriteFile(firstPending, []byte(newSQL), 0o644); err != nil {
			return nil, fmt.Errorf("write %s: %w", firstPending, err)
		}
		res.Rebased = append(res.Rebased, firstBase)
	} else {
		res.Unchanged = append(res.Unchanged, firstBase)
	}

	// Remove any additional pending files — their changes are now folded into
	// the first file. The developer should commit the deletion.
	for _, extra := range pending[1:] {
		if err := os.Remove(extra); err != nil {
			return nil, fmt.Errorf("remove superseded migration %s: %w", extra, err)
		}
		res.Rebased = append(res.Rebased, filepath.Base(extra)+" (removed, folded into "+firstBase+")")
	}

	return res, nil
}
