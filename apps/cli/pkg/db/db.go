package db

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// rePassword matches the password component in a DSN (:<password>@) and replaces it with :***@.
var rePassword = regexp.MustCompile(`:[^:@/]+@`)

// sanitizeDSN replaces the password in a DSN string with *** so it is safe to surface in errors.
func sanitizeDSN(dsn string) string {
	return rePassword.ReplaceAllString(dsn, ":***@")
}

// NewPool returns a connection pool; conn may be from DATABASE_URL.
func NewPool(ctx context.Context, conn string) (*pgxpool.Pool, error) {
	conn = strings.TrimSpace(conn)
	if conn == "" {
		conn = os.Getenv("DATABASE_URL")
	}
	if conn == "" {
		return nil, fmt.Errorf("database connection string: use --db or set DATABASE_URL")
	}
	cfg, err := pgxpool.ParseConfig(conn)
	if err != nil {
		return nil, err
	}
	cfg.MaxConns = 8
	cfg.ConnConfig.ConnectTimeout = 10 * time.Second
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect to %s: %w", sanitizeDSN(conn), err)
	}
	return pool, nil
}
