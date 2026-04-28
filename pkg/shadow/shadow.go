// Package shadow provides optional validation of migration plans against a disposable connection.
//
//   - ValidateSyntaxInTxn / ValidateSyntaxOnDatabase: each DDL in one transaction, then ROLLBACK (syntax only).
//   - ValidateSemanticApply / ValidateSemanticOnDatabase: each DDL in autocommit order (mutates DB; use empty instance).
//   - ValidateMigrationSQL: validate one migration file's SQL in a rolled-back transaction.
//
// Neither mode proves logical equivalence with production; semantic apply is the stronger check for ordering and catalog effects.
package shadow

import (
	"context"
	"fmt"
	"strings"

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

// reTransactionControl matches standalone BEGIN / COMMIT lines that pg-flux adds to
// migration files as human-readable markers; these must be stripped before wrapping the
// content in our own rolled-back validation transaction.
var reValidateTxnControl = strings.NewReplacer(
	"BEGIN;", "",
	"begin;", "",
	"COMMIT;", "",
	"commit;", "",
)

// ValidateMigrationSQL validates a single migration file's SQL by executing all
// non-CONCURRENTLY statements inside a transaction that is always rolled back.
// It does not mutate any live table; it only catches SQL syntax and semantic errors
// that would be visible before data access (missing table, wrong column type, etc.).
//
// CONCURRENTLY statements cannot run inside a transaction; they are validated by parsing
// alone (BEGIN; <stmt> COMMIT would also fail for them, so we skip execution entirely).
//
// pool should point to a disposable shadow database or the live DB. Using the live
// DB is safe because the transaction is always rolled back, but it does take brief locks.
func ValidateMigrationSQL(ctx context.Context, pool *pgxpool.Pool, filename string, content []byte) error {
	if pool == nil {
		return fmt.Errorf("shadow: nil pool")
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
	committed := false
	defer func() {
		if !committed {
			_, _ = c.Exec(context.Background(), "ROLLBACK")
		}
	}()

	sql := reValidateTxnControl.Replace(string(content))
	for _, stmt := range splitSQLForShadow(sql) {
		upper := strings.ToUpper(stmt)
		if strings.Contains(upper, "CONCURRENTLY") {
			// Cannot run inside a transaction; skip execution (syntax was already validated
			// by pg_query.Parse in a prior step when the migration was generated).
			continue
		}
		if _, err := c.Exec(ctx, stmt); err != nil {
			_, _ = c.Exec(context.Background(), "ROLLBACK")
			committed = true // prevent double-rollback in defer
			return fmt.Errorf("shadow validate %s (rolled back): %w", filename, err)
		}
	}
	_, _ = c.Exec(ctx, "ROLLBACK")
	committed = true
	return nil
}

// splitSQLForShadow splits a SQL string into individual statements, correctly handling
// dollar-quoted strings (e.g. $$ ... $$ in function bodies), single-quoted strings,
// and double-quoted identifiers. A naïve strings.Split(sql, ";") would break on
// semicolons inside function bodies.
func splitSQLForShadow(sql string) []string {
	var out []string
	var cur strings.Builder
	n := len(sql)
	i := 0
	for i < n {
		ch := sql[i]
		switch {
		// Single-line comment — consume until newline.
		case ch == '-' && i+1 < n && sql[i+1] == '-':
			for i < n && sql[i] != '\n' {
				cur.WriteByte(sql[i])
				i++
			}
		// Block comment /* ... */
		case ch == '/' && i+1 < n && sql[i+1] == '*':
			cur.WriteByte(ch)
			i++
			cur.WriteByte(sql[i])
			i++
			for i < n {
				if sql[i] == '*' && i+1 < n && sql[i+1] == '/' {
					cur.WriteByte(sql[i])
					i++
					cur.WriteByte(sql[i])
					i++
					break
				}
				cur.WriteByte(sql[i])
				i++
			}
		// Single-quoted literal 'string'.
		case ch == '\'':
			cur.WriteByte(ch)
			i++
			for i < n {
				c := sql[i]
				cur.WriteByte(c)
				i++
				if c == '\'' {
					if i < n && sql[i] == '\'' {
						cur.WriteByte(sql[i])
						i++
					} else {
						break
					}
				}
			}
		// Double-quoted identifier.
		case ch == '"':
			cur.WriteByte(ch)
			i++
			for i < n {
				c := sql[i]
				cur.WriteByte(c)
				i++
				if c == '"' {
					if i < n && sql[i] == '"' {
						cur.WriteByte(sql[i])
						i++
					} else {
						break
					}
				}
			}
		// Dollar-quoted string: $tag$...$tag$
		case ch == '$':
			j := i + 1
			for j < n && sql[j] != '$' {
				j++
			}
			if j >= n {
				cur.WriteByte(ch)
				i++
				break
			}
			tag := sql[i : j+1]
			cur.WriteString(tag)
			i = j + 1
			closing := tag
			for i < n {
				if strings.HasPrefix(sql[i:], closing) {
					cur.WriteString(closing)
					i += len(closing)
					break
				}
				cur.WriteByte(sql[i])
				i++
			}
		// Statement terminator.
		case ch == ';':
			stmt := strings.TrimSpace(cur.String())
			cur.Reset()
			i++
			if stmt == "" {
				continue
			}
			// Strip leading comment-only lines so an all-comment "statement" is skipped.
			var lines []string
			for _, line := range strings.Split(stmt, "\n") {
				if !strings.HasPrefix(strings.TrimSpace(line), "--") {
					lines = append(lines, line)
				}
			}
			if s := strings.TrimSpace(strings.Join(lines, "\n")); s != "" {
				out = append(out, s)
			}
		default:
			cur.WriteByte(ch)
			i++
		}
	}
	// Handle trailing statement without a trailing semicolon.
	if stmt := strings.TrimSpace(cur.String()); stmt != "" {
		out = append(out, stmt)
	}
	return out
}
