// Package migrate implements the pg-flux migration file workflow:
// generate timestamped .sql files from a live-DB diff, apply pending files,
// and track applied migrations in a dedicated _pgflux schema.
package migrate

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)


const defaultTrackingSchema = "_pgflux"

// EnsureTrackingTable creates the _pgflux schema and migrations table if they
// do not already exist. Safe to call on every startup (idempotent).
func EnsureTrackingTable(ctx context.Context, pool *pgxpool.Pool, trackingSchema string) error {
	if trackingSchema == "" {
		trackingSchema = defaultTrackingSchema
	}
	ddl := fmt.Sprintf(`
CREATE SCHEMA IF NOT EXISTS %s;
CREATE TABLE IF NOT EXISTS %s.migrations (
	filename    text        PRIMARY KEY,
	applied_at  timestamptz NOT NULL DEFAULT now(),
	checksum    text        NOT NULL
);`, quoteIdent(trackingSchema), quoteIdent(trackingSchema))
	_, err := pool.Exec(ctx, ddl)
	if err != nil {
		return fmt.Errorf("ensure tracking table: %w", err)
	}
	return nil
}

// AppliedSet returns the set of filenames that have already been applied.
func AppliedSet(ctx context.Context, pool *pgxpool.Pool, trackingSchema string) (map[string]string, error) {
	if trackingSchema == "" {
		trackingSchema = defaultTrackingSchema
	}
	rows, err := pool.Query(ctx,
		fmt.Sprintf(`SELECT filename, checksum FROM %s.migrations ORDER BY filename`,
			quoteIdent(trackingSchema)))
	if err != nil {
		return nil, fmt.Errorf("query applied migrations: %w", err)
	}
	defer rows.Close()
	out := make(map[string]string)
	for rows.Next() {
		var fname, chk string
		if err := rows.Scan(&fname, &chk); err != nil {
			return nil, err
		}
		out[fname] = chk
	}
	return out, rows.Err()
}

// Checksum returns the hex-encoded SHA-256 of content.
func Checksum(content []byte) string {
	h := sha256.Sum256(content)
	return fmt.Sprintf("%x", h)
}

// TimestampFilename returns a migration filename of the form
// YYYYMMDD_HHMMSS[_label].sql using the current UTC time.
func TimestampFilename(label string) string {
	ts := time.Now().UTC().Format("20060102_150405")
	if label == "" {
		return ts + ".sql"
	}
	// sanitise label: keep alphanumeric and underscores only
	safe := make([]byte, 0, len(label))
	for _, c := range []byte(label) {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '_' {
			safe = append(safe, c)
		} else {
			safe = append(safe, '_')
		}
	}
	return ts + "_" + string(safe) + ".sql"
}

// quoteIdent double-quotes a PostgreSQL identifier.
func quoteIdent(s string) string {
	return `"` + s + `"`
}
