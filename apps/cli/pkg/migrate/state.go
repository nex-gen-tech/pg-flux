// Package migrate implements the pg-flux migration file workflow:
// generate timestamped .sql files from a live-DB diff, apply pending files,
// and track applied migrations in a dedicated _pgflux schema.
package migrate

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
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

// RepairOptions controls the repair command.
type RepairOptions struct {
	MigrationsDir  string
	TrackingSchema string
	// Filename restricts repair to a single file. Empty = repair all mismatches.
	Filename string
}

// Repair updates the stored checksum for migration files whose on-disk content
// has changed since they were applied. Returns the list of repaired filenames.
// Use only when a migration was deliberately edited after application.
func Repair(ctx context.Context, pool *pgxpool.Pool, opts RepairOptions) ([]string, error) {
	if opts.TrackingSchema == "" {
		opts.TrackingSchema = defaultTrackingSchema
	}

	applied, err := AppliedSet(ctx, pool, opts.TrackingSchema)
	if err != nil {
		return nil, err
	}

	files, err := migrationFiles(opts.MigrationsDir)
	if err != nil {
		return nil, err
	}

	var repaired []string
	for _, fpath := range files {
		base := filepath.Base(fpath)
		if opts.Filename != "" && base != opts.Filename {
			continue
		}
		prevChk, wasApplied := applied[base]
		if !wasApplied {
			continue
		}
		content, err := os.ReadFile(fpath)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", base, err)
		}
		newChk := Checksum(content)
		if newChk == prevChk {
			continue // already consistent
		}
		_, err = pool.Exec(ctx,
			fmt.Sprintf(`UPDATE %s.migrations SET checksum = $1 WHERE filename = $2`,
				quoteIdent(opts.TrackingSchema)),
			newChk, base)
		if err != nil {
			return nil, fmt.Errorf("repair %s: %w", base, err)
		}
		repaired = append(repaired, base)
	}
	return repaired, nil
}

// BaselineOptions controls the baseline command.
type BaselineOptions struct {
	MigrationsDir  string
	TrackingSchema string
	// UpTo baselines only files up to and including this filename.
	// Empty = baseline all pending files.
	UpTo string
}

// Baseline marks migration files as applied in the tracking table without executing
// their SQL. Used to onboard existing databases managed outside pg-flux.
// Returns the list of filenames that were baselined.
func Baseline(ctx context.Context, pool *pgxpool.Pool, opts BaselineOptions) ([]string, error) {
	if opts.TrackingSchema == "" {
		opts.TrackingSchema = defaultTrackingSchema
	}

	if err := EnsureTrackingTable(ctx, pool, opts.TrackingSchema); err != nil {
		return nil, err
	}

	applied, err := AppliedSet(ctx, pool, opts.TrackingSchema)
	if err != nil {
		return nil, err
	}

	files, err := migrationFiles(opts.MigrationsDir)
	if err != nil {
		return nil, err
	}

	var baselined []string
	for _, fpath := range files {
		base := filepath.Base(fpath)
		if _, done := applied[base]; done {
			continue // already applied or baselined
		}
		content, err := os.ReadFile(fpath)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", base, err)
		}
		chk := Checksum(content)
		_, err = pool.Exec(ctx,
			fmt.Sprintf(`INSERT INTO %s.migrations (filename, checksum) VALUES ($1, $2) ON CONFLICT (filename) DO NOTHING`,
				quoteIdent(opts.TrackingSchema)),
			base, chk)
		if err != nil {
			return nil, fmt.Errorf("baseline %s: %w", base, err)
		}
		baselined = append(baselined, base)
		if opts.UpTo != "" && base == opts.UpTo {
			break
		}
	}
	return baselined, nil
}
