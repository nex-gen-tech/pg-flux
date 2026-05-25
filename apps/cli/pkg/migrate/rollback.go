package migrate

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nex-gen-tech/pg-flux/pkg/obs"
)

// RollbackOptions controls migration rollback behaviour.
type RollbackOptions struct {
	MigrationsDir  string
	TrackingSchema string
	N              int // number of migrations to roll back (default 1, 0 treated as 1)
	DryRun         bool
	Progress       io.Writer
}

// RollbackResult summarises what was done.
type RollbackResult struct {
	RolledBack []string // filenames successfully rolled back
	NoDownSQL  []string // filenames skipped because no Down SQL was found
}

// Rollback rolls back the last N applied migrations in reverse-applied order.
// For each migration it resolves the Down SQL, executes it in a transaction,
// and removes the tracking row. Migrations with no Down SQL are recorded in
// NoDownSQL and skipped (not an error).
func Rollback(ctx context.Context, pool *pgxpool.Pool, opts RollbackOptions) (*RollbackResult, error) {
	if opts.TrackingSchema == "" {
		opts.TrackingSchema = defaultTrackingSchema
	}
	n := opts.N
	if n <= 0 {
		n = 1
	}

	filenames, err := AppliedOrdered(ctx, pool, opts.TrackingSchema, n)
	if err != nil {
		return nil, fmt.Errorf("list applied migrations: %w", err)
	}

	res := &RollbackResult{}

	for _, filename := range filenames {
		downSQL, err := ResolveDownSQL(opts.MigrationsDir, filename)
		if err != nil {
			return nil, fmt.Errorf("resolve down sql for %s: %w", filename, err)
		}
		if downSQL == "" {
			logf(opts.Progress, "skip  %s (no down SQL)\n", filename)
			res.NoDownSQL = append(res.NoDownSQL, filename)
			continue
		}

		if opts.DryRun {
			logf(opts.Progress, "would rollback %s\n", filename)
			res.RolledBack = append(res.RolledBack, filename)
			continue
		}

		logf(opts.Progress, "rollback %s ...\n", filename)
		start := time.Now()
		if err := rollbackOne(ctx, pool, opts.TrackingSchema, filename, downSQL); err != nil {
			obs.ErrorCtx(ctx, "migrate.rollback.failed",
				"file", filename,
				"error", err.Error(),
				"duration_ms", time.Since(start).Milliseconds(),
			)
			return nil, fmt.Errorf("rollback %s: %w", filename, err)
		}
		res.RolledBack = append(res.RolledBack, filename)
		logf(opts.Progress, "         ok\n")
		obs.InfoCtx(ctx, "migrate.rolled_back",
			"file", filename,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	}

	obs.InfoCtx(ctx, "migrate.rollback.summary",
		"rolled_back_count", len(res.RolledBack),
		"skipped_count", len(res.NoDownSQL),
	)
	return res, nil
}

// rollbackOne executes the down SQL for a single migration and removes the tracking row.
func rollbackOne(ctx context.Context, pool *pgxpool.Pool, trackingSchema, filename, downSQL string) error {
	stmts := splitSQLStatements(downSQL)

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	for _, s := range stmts {
		if reTransactionControl.MatchString(s) {
			continue
		}
		if _, err := tx.Exec(ctx, s); err != nil {
			return fmt.Errorf("exec: %w", err)
		}
	}

	_, err = tx.Exec(ctx,
		fmt.Sprintf(`DELETE FROM %s.migrations WHERE filename = $1`, quoteIdent(trackingSchema)),
		filename)
	if err != nil {
		return fmt.Errorf("delete tracking row: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	committed = true
	return nil
}
