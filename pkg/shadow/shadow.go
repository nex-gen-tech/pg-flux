// Package shadow provides optional validation of migration plans against a disposable connection.
//
//   - ValidateSyntaxInTxn / ValidateSyntaxOnDatabase: each DDL in one transaction, then ROLLBACK (syntax only).
//   - ValidateSemanticApply / ValidateSemanticOnDatabase: each DDL in autocommit order (mutates DB; use empty instance).
//
// Neither mode proves logical equivalence with production; semantic apply is the stronger check for ordering and catalog effects.
package shadow

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nexg/pg-flux/pkg/plan"
)

// ValidateSyntaxInTxn runs each non-concurrent DDL in a single transaction and rolls back.
// It does not prove semantic equivalence with production; it catches basic SQL errors.
// Statements marked IsConcurrent are skipped (cannot run inside a transaction).
func ValidateSyntaxInTxn(ctx context.Context, pool *pgxpool.Pool, p *plan.ExecutionPlan) error {
	if p == nil || len(p.Statements) == 0 {
		return nil
	}
	ac, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer ac.Release()
	c := ac.Conn()
	if _, err := c.Exec(ctx, "BEGIN"); err != nil {
		return err
	}
	defer func() { _, _ = c.Exec(ctx, "ROLLBACK") }()
	for _, s := range p.Statements {
		if s.DDL == "" || s.IsConcurrent {
			continue
		}
		if _, err := c.Exec(ctx, s.DDL); err != nil {
			return fmt.Errorf("shadow validate (rolled back): statement %d: %w", s.ID, err)
		}
	}
	return nil
}

// ValidateSyntaxOnDatabase opens a pool from connString and runs ValidateSyntaxInTxn.
// Use a disposable database (or dedicated role) for PRD-style "second database" syntax checks.
func ValidateSyntaxOnDatabase(ctx context.Context, connString string, p *plan.ExecutionPlan) error {
	if connString == "" {
		return fmt.Errorf("shadow: empty connection string")
	}
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return err
	}
	defer pool.Close()
	return ValidateSyntaxInTxn(ctx, pool, p)
}
