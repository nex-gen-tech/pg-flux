package exec

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"regexp"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nexg/pg-flux/pkg/plan"
)

// validTimeout is a conservative allowlist for PostgreSQL timeout values (e.g. "3s", "500ms", "1min", "0").
// Only digits, ASCII letters, and spaces are permitted — prevents injection into SET LOCAL statements.
var validTimeout = regexp.MustCompile(`^[0-9a-zA-Z ]+$`)

// Options for applying a plan.
type Options struct {
	DryRun           bool
	LockTimeout      string    // e.g. "3s"; defaults to "3s"
	StatementTimeout string    // e.g. "0" (unlimited) or "20min"; empty = not set
	Progress         io.Writer // if non-nil, per-statement progress is written here
}

// Apply runs DDL: one transaction for non-concurrent statements, then each CONCURRENT statement autocommit (PRD).
func Apply(ctx context.Context, pool *pgxpool.Pool, p *plan.ExecutionPlan, o Options) error {
	if p == nil || len(p.Statements) == 0 {
		return nil
	}
	if o.DryRun {
		return nil
	}
	if o.LockTimeout == "" {
		o.LockTimeout = "3s"
	}
	if !validTimeout.MatchString(o.LockTimeout) {
		return fmt.Errorf("invalid lock_timeout value %q: only digits, letters, and spaces are allowed", o.LockTimeout)
	}
	if o.StatementTimeout != "" && !validTimeout.MatchString(o.StatementTimeout) {
		return fmt.Errorf("invalid statement_timeout value %q: only digits, letters, and spaces are allowed", o.StatementTimeout)
	}
	connString := os.Getenv("DATABASE_URL")
	if connString == "" {
		connString = "default"
	}
	h := sha256.Sum256([]byte(connString))
	lockID := int64(binary.BigEndian.Uint64(h[:8]))

	ac, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer ac.Release()
	c := ac.Conn()

	// lockAcquired tracks whether the session-level advisory lock was acquired so the
	// deferred cleanup can always release it — even on error or context cancellation.
	lockAcquired := false
	defer func() {
		if lockAcquired {
			// Use Background so a cancelled parent context does not prevent cleanup.
			_, _ = c.Exec(context.Background(), `SELECT pg_advisory_unlock($1::bigint)`, lockID)
		}
	}()

	var nonConcurrent, concurrent []plan.Statement
	for _, s := range p.Statements {
		if s.DDL == "" {
			continue
		}
		if s.IsConcurrent {
			concurrent = append(concurrent, s)
		} else {
			nonConcurrent = append(nonConcurrent, s)
		}
	}

	if len(nonConcurrent) > 0 {
		if _, err := c.Exec(ctx, "BEGIN"); err != nil {
			return err
		}
		var ok bool
		if err := c.QueryRow(ctx, `SELECT pg_try_advisory_lock($1::bigint)`, lockID).Scan(&ok); err != nil {
			_, _ = c.Exec(ctx, "ROLLBACK")
			return err
		}
		if !ok {
			_, _ = c.Exec(ctx, "ROLLBACK")
			return fmt.Errorf("could not acquire migration advisory lock (another apply in progress)")
		}
		lockAcquired = true
		if _, err := c.Exec(ctx, "SET LOCAL lock_timeout = '"+o.LockTimeout+"'"); err != nil {
			_, _ = c.Exec(ctx, "ROLLBACK")
			return err
		}
		if o.StatementTimeout != "" {
			if _, err := c.Exec(ctx, "SET LOCAL statement_timeout = '"+o.StatementTimeout+"'"); err != nil {
				_, _ = c.Exec(ctx, "ROLLBACK")
				return err
			}
		}
		for _, s := range nonConcurrent {
			if o.Progress != nil {
				fmt.Fprintf(o.Progress, "[%d] %s\n", s.ID, s.DDL)
			}
			if _, err := c.Exec(ctx, s.DDL); err != nil {
				_, _ = c.Exec(ctx, "ROLLBACK")
				return fmt.Errorf("statement %d: %w", s.ID, err)
			}
		}
		if _, err := c.Exec(ctx, "COMMIT"); err != nil {
			return err
		}
	}

	// CONCURRENT index ops cannot run in an open transaction; each Exec autocommits.
	if len(concurrent) > 0 {
		if len(nonConcurrent) == 0 {
			var ok bool
			if err := c.QueryRow(ctx, `SELECT pg_try_advisory_lock($1::bigint)`, lockID).Scan(&ok); err != nil {
				return err
			}
			if !ok {
				return fmt.Errorf("could not acquire migration advisory lock (another apply in progress)")
			}
			lockAcquired = true
		}
		for _, s := range concurrent {
			if o.Progress != nil {
				fmt.Fprintf(o.Progress, "[%d] %s (concurrent)\n", s.ID, s.DDL)
			}
			if _, err := c.Exec(ctx, s.DDL); err != nil {
				return fmt.Errorf("concurrent statement %d: %w", s.ID, err)
			}
		}
	}
	return nil
}
