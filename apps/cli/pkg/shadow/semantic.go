package shadow

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nex-gen-tech/pg-flux/pkg/plan"
)

// shadowLockID derives a deterministic advisory lock ID from a connection string.
// This serializes concurrent shadow validations against the same disposable DB.
func shadowLockID(connString string) int64 {
	h := sha256.Sum256([]byte("pg-flux-shadow\x00" + connString))
	return int64(binary.BigEndian.Uint64(h[:8]))
}

// ValidateSemanticApply runs each non-concurrent DDL statement with autocommit against the pool.
// Unlike ValidateSyntaxInTxn, later statements see objects created by earlier ones, which catches
// ordering and many semantic failures on an empty (or reset) database.
//
// Warning: this applies DDL for real on the target database. Use only on a disposable shadow instance.
// CONCURRENTLY index statements are skipped (same as syntax validation).
// An advisory lock is held for the duration of the call to prevent concurrent shadow validations
// from interfering with each other on the same DB.
func ValidateSemanticApply(ctx context.Context, pool *pgxpool.Pool, p *plan.ExecutionPlan, connString string) error {
	if p == nil || len(p.Statements) == 0 {
		return nil
	}
	ac, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("shadow: acquire connection: %w", err)
	}
	defer ac.Release()
	c := ac.Conn()

	lockID := shadowLockID(connString)
	// pg_advisory_lock blocks until the lock is available — serializes concurrent shadow runs.
	if _, err := c.Exec(ctx, `SELECT pg_advisory_lock($1::bigint)`, lockID); err != nil {
		return fmt.Errorf("shadow: advisory lock: %w", err)
	}
	defer func() {
		_, _ = c.Exec(context.Background(), `SELECT pg_advisory_unlock($1::bigint)`, lockID)
	}()

	for _, s := range p.Statements {
		if s.DDL == "" || s.IsConcurrent {
			continue
		}
		if _, err := c.Exec(ctx, s.DDL); err != nil {
			return fmt.Errorf("semantic shadow apply: statement %d: %w", s.ID, err)
		}
	}
	return nil
}

// ValidateSemanticOnDatabase opens a pool and runs ValidateSemanticApply.
func ValidateSemanticOnDatabase(ctx context.Context, connString string, p *plan.ExecutionPlan) error {
	if connString == "" {
		return fmt.Errorf("shadow: empty connection string")
	}
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return err
	}
	defer pool.Close()
	return ValidateSemanticApply(ctx, pool, p, connString)
}
