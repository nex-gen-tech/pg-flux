package shadow

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nexg/pg-flux/pkg/plan"
)

// ValidateSemanticApply runs each non-concurrent DDL statement with autocommit against the pool.
// Unlike ValidateSyntaxInTxn, later statements see objects created by earlier ones, which catches
// ordering and many semantic failures on an empty (or reset) database.
//
// Warning: this applies DDL for real on the target database. Use only on a disposable shadow instance.
// CONCURRENTLY index statements are skipped (same as syntax validation).
func ValidateSemanticApply(ctx context.Context, pool *pgxpool.Pool, p *plan.ExecutionPlan) error {
	if p == nil || len(p.Statements) == 0 {
		return nil
	}
	for _, s := range p.Statements {
		if s.DDL == "" || s.IsConcurrent {
			continue
		}
		if _, err := pool.Exec(ctx, s.DDL); err != nil {
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
	return ValidateSemanticApply(ctx, pool, p)
}
