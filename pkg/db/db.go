package db

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

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
	return pgxpool.NewWithConfig(ctx, cfg)
}
