package migrate

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nexg/pg-flux/pkg/obs"
	"github.com/nexg/pg-flux/pkg/shadow"
)

// ApplyOptions controls migration application.
type ApplyOptions struct {
	// MigrationsDir is the folder containing .sql migration files.
	MigrationsDir string
	// TrackingSchema is the schema used for the migrations tracking table (default: _pgflux).
	TrackingSchema string
	// DryRun prints what would be applied without executing anything.
	DryRun bool
	// ShadowDSN is an optional DSN for a shadow database used for pre-flight validation.
	// When non-empty, each pending migration is validated in a rolled-back transaction on
	// this database before being applied to the real one. Requires pkg/shadow.
	ShadowDSN string
	// Progress receives log lines (may be nil).
	Progress io.Writer
	// Schemas selects which schemas the inspector reads when performing the
	// baseline-drift check. Defaults to ["public"] when empty.
	Schemas []string
	// ForceAfterDrift bypasses the baseline-hash drift check. The check refuses to
	// apply when the live DB no longer matches the state the first pending migration
	// was generated against. Default false: refuse on drift.
	ForceAfterDrift bool
}

// ApplyResult summarises what was done.
type ApplyResult struct {
	Applied []string // filenames applied
	Skipped []string // already-applied filenames
}

// Apply applies all pending migration files in timestamp order.
// Each migration runs inside its own transaction; the tracking row is inserted
// in the same transaction so it is atomically committed or rolled back.
func Apply(ctx context.Context, pool *pgxpool.Pool, opts ApplyOptions) (*ApplyResult, error) {
	if opts.TrackingSchema == "" {
		opts.TrackingSchema = defaultTrackingSchema
	}

	if !opts.DryRun {
		if err := EnsureTrackingTable(ctx, pool, opts.TrackingSchema); err != nil {
			return nil, err
		}
	}

	files, err := migrationFiles(opts.MigrationsDir)
	if err != nil {
		return nil, err
	}

	var applied map[string]string
	if !opts.DryRun {
		applied, err = AppliedSet(ctx, pool, opts.TrackingSchema)
		if err != nil {
			return nil, err
		}
	} else {
		applied = make(map[string]string)
	}

	res := &ApplyResult{}

	var shadowPool *pgxpool.Pool
	defer func() {
		if shadowPool != nil {
			shadowPool.Close()
		}
	}()

	driftChecked := false
	for _, fname := range files {
		base := filepath.Base(fname)

		content, err := os.ReadFile(fname)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", fname, err)
		}
		chk := Checksum(content)

		if prevChk, done := applied[base]; done {
			// Tamper detection: applied file content must match recorded checksum.
			if prevChk != chk {
				return nil, fmt.Errorf(
					"checksum mismatch for already-applied migration %s: "+
						"recorded=%s current=%s — do not edit applied migrations",
					base, prevChk, chk)
			}
			res.Skipped = append(res.Skipped, base)
			logf(opts.Progress, "skip  %s (already applied)\n", base)
			continue
		}

		if opts.DryRun {
			logf(opts.Progress, "would apply  %s\n", base)
			res.Applied = append(res.Applied, base)
			continue
		}

		// Baseline-hash drift check: compare live state against the hash recorded
		// at generate time in the FIRST pending migration's header. Subsequent
		// migrations were generated against intermediate states we don't materialize,
		// so we can't reliably check them here.
		if !driftChecked {
			driftChecked = true
			if !opts.ForceAfterDrift {
				schemas := opts.Schemas
				if len(schemas) == 0 {
					schemas = []string{"public"}
				}
				if err := checkBaselineDrift(ctx, pool, schemas, base, content); err != nil {
					obs.ErrorCtx(ctx, "migrate.drift_detected",
						"file", base,
						"error", err.Error(),
					)
					return nil, err
				}
			}
		}

		// Pre-flight shadow validation: apply in a rolled-back transaction on the shadow DB
		// to catch SQL syntax / semantic errors before touching the live database.
		if opts.ShadowDSN != "" {
			logf(opts.Progress, "shadow  %s ...\n", base)
			if shadowPool == nil {
				shadowPool, err = pgxpool.New(ctx, opts.ShadowDSN)
				if err != nil {
					return nil, fmt.Errorf("shadow connect: %w", err)
				}
			}
			if err := shadow.ValidateMigrationSQL(ctx, shadowPool, base, content); err != nil {
				return nil, fmt.Errorf("shadow validate %s: %w", base, err)
			}
			logf(opts.Progress, "        ok (shadow)\n")
		}

		logf(opts.Progress, "apply %s ...\n", base)
		start := time.Now()
		if err := applyOne(ctx, pool, opts.TrackingSchema, base, content, chk); err != nil {
			obs.ErrorCtx(ctx, "migrate.apply.failed",
				"file", base,
				"error", err.Error(),
				"duration_ms", time.Since(start).Milliseconds(),
			)
			return nil, fmt.Errorf("apply %s: %w", base, err)
		}
		res.Applied = append(res.Applied, base)
		logf(opts.Progress, "      ok\n")
		obs.InfoCtx(ctx, "migrate.applied",
			"file", base,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	}

	obs.InfoCtx(ctx, "migrate.apply.summary",
		"applied_count", len(res.Applied),
		"skipped_count", len(res.Skipped),
	)
	return res, nil
}

// StatusOptions controls the status listing.
type StatusOptions struct {
	MigrationsDir  string
	TrackingSchema string
}

// MigrationStatus describes a single migration file's state.
type MigrationStatus struct {
	Filename string
	Applied  bool
	// AppliedAt is non-zero only when Applied is true.
	AppliedAt string
}

// Status returns the ordered list of migration files with their applied state.
func Status(ctx context.Context, pool *pgxpool.Pool, opts StatusOptions) ([]MigrationStatus, error) {
	if opts.TrackingSchema == "" {
		opts.TrackingSchema = defaultTrackingSchema
	}

	files, err := migrationFiles(opts.MigrationsDir)
	if err != nil {
		return nil, err
	}

	// Query tracking table — if it doesn't exist yet, treat all as pending.
	rows, qErr := pool.Query(ctx,
		fmt.Sprintf(`SELECT filename, applied_at::text FROM %s.migrations ORDER BY filename`,
			quoteIdent(opts.TrackingSchema)))
	appliedAt := make(map[string]string)
	if qErr == nil {
		defer rows.Close()
		for rows.Next() {
			var fn, at string
			if err := rows.Scan(&fn, &at); err != nil {
				return nil, err
			}
			appliedAt[fn] = at
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}

	var out []MigrationStatus
	for _, f := range files {
		base := filepath.Base(f)
		at, ok := appliedAt[base]
		out = append(out, MigrationStatus{Filename: base, Applied: ok, AppliedAt: at})
	}
	return out, nil
}

// applyOne executes a migration file and records it in the tracking table.
//
// Strategy:
//   - Split the file into individual SQL statements.
//   - Statements that do NOT contain CONCURRENTLY are batched into a single
//     transaction together with the tracking row INSERT (fully atomic).
//   - CONCURRENTLY statements (e.g. CREATE INDEX CONCURRENTLY) cannot run
//     inside a transaction (PostgreSQL restriction); they are executed outside
//     any transaction in autocommit mode, after the transactional batch commits.
//   - If all statements are non-concurrent the tracking row is committed in the
//     same transaction as the DDL.
//   - If any concurrent statements exist, the tracking row is inserted in its
//     own transaction after all concurrent statements succeed.
// reTransactionControl matches bare BEGIN or COMMIT statements emitted by
// buildMigrationSQL as human-readable transaction markers.  applyOne strips them
// because the Go-level transaction wrapper (pool.Begin / tx.Commit) handles
// atomicity together with the tracking-table INSERT.
var reTransactionControl = regexp.MustCompile(`(?i)^\s*(begin|commit)\s*$`)

func applyOne(ctx context.Context, pool *pgxpool.Pool, trackingSchema, base string, content []byte, chk string) error {
	stmts := splitSQLStatements(string(content))

	// The migration file is laid out as:
	//   BEGIN; <regular DDL>; COMMIT; <CONCURRENTLY statements + their dependents>
	// Any statement appearing AFTER the closing COMMIT marker must run outside the
	// main transaction even if it doesn't contain CONCURRENTLY itself — this matters
	// for `COMMENT ON INDEX` which has to follow `CREATE INDEX CONCURRENTLY`.
	var regular, concurrent []string
	pastCommit := false
	for _, s := range stmts {
		if reTransactionControl.MatchString(s) {
			if strings.EqualFold(strings.TrimSpace(s), "COMMIT") {
				pastCommit = true
			}
			continue
		}
		if pastCommit || isConcurrent(s) {
			concurrent = append(concurrent, s)
		} else {
			regular = append(regular, s)
		}
	}

	// --- transactional block: regular DDL + tracking row (when no concurrent stmts) ---
	if len(regular) == 0 && len(concurrent) == 0 {
		// Nothing to execute — still record the migration so it is not re-applied every
		// run, but skip the DDL transaction entirely.
		_, err := pool.Exec(ctx,
			fmt.Sprintf(`INSERT INTO %s.migrations (filename, checksum) VALUES ($1, $2)`,
				quoteIdent(trackingSchema)),
			base, chk)
		if err != nil {
			return fmt.Errorf("record empty migration: %w", err)
		}
		return nil
	}

	// --- transactional block: regular DDL + tracking row (when no concurrent stmts) ---
	if len(regular) > 0 {
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
		for _, s := range regular {
			if _, err := tx.Exec(ctx, s); err != nil {
				return fmt.Errorf("exec: %w", err)
			}
		}
		// Commit the tracking row together with the DDL when there are no
		// concurrent statements that follow — this is fully atomic.
		if len(concurrent) == 0 {
			_, err = tx.Exec(ctx,
				fmt.Sprintf(`INSERT INTO %s.migrations (filename, checksum) VALUES ($1, $2)`,
					quoteIdent(trackingSchema)),
				base, chk)
			if err != nil {
				return fmt.Errorf("record: %w", err)
			}
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit: %w", err)
		}
		committed = true
	}

	// --- autocommit block: CONCURRENT statements ---
	for _, s := range concurrent {
		if _, err := pool.Exec(ctx, s); err != nil {
			return fmt.Errorf("exec concurrent: %w", err)
		}
	}

	// When concurrent statements exist, insert the tracking row after they all
	// succeed. This is not atomic with the concurrent DDL (a crash between the
	// last CONCURRENTLY and this insert would leave the migration un-recorded),
	// but that is an inherent PostgreSQL limitation of CONCURRENTLY outside a txn.
	if len(concurrent) > 0 {
		_, err := pool.Exec(ctx,
			fmt.Sprintf(`INSERT INTO %s.migrations (filename, checksum) VALUES ($1, $2)`,
				quoteIdent(trackingSchema)),
			base, chk)
		if err != nil {
			return fmt.Errorf("record: %w", err)
		}
	}

	return nil
}

// splitSQLStatements splits a SQL script into individual non-empty statements,
// correctly handling:
//   - dollar-quoted strings ($tag$...$tag$), e.g. DO $pgflux$ ... END $pgflux$
//   - single-quoted string literals ('...' with '' escaping)
//   - double-quoted identifiers ("...")
//   - line comments (--)
//   - block comments (/* ... */)
//
// Only semicolons that appear outside all of the above are treated as statement
// terminators.
func splitSQLStatements(sql string) []string {
	var stmts []string
	var cur strings.Builder
	i := 0
	n := len(sql)

	for i < n {
		ch := sql[i]

		switch {
		// Line comment: skip to end of line.
		case ch == '-' && i+1 < n && sql[i+1] == '-':
			cur.WriteByte(ch)
			i++
			for i < n && sql[i] != '\n' {
				cur.WriteByte(sql[i])
				i++
			}

		// Block comment: skip /* ... */ (non-nested).
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

		// Single-quoted string literal (with '' escape).
		case ch == '\'':
			cur.WriteByte(ch)
			i++
			for i < n {
				c := sql[i]
				cur.WriteByte(c)
				i++
				if c == '\'' {
					// Check for '' escape sequence.
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
					if i < n && sql[i] == '"' { // "" escape inside identifier
						cur.WriteByte(sql[i])
						i++
					} else {
						break
					}
				}
			}

		// Dollar-quoted string: $tag$...$tag$
		case ch == '$':
			// Find the closing $ of the opening tag.
			j := i + 1
			for j < n && sql[j] != '$' {
				j++
			}
			if j >= n {
				// Not a dollar-quote delimiter, treat as regular char.
				cur.WriteByte(ch)
				i++
				break
			}
			tag := sql[i : j+1] // e.g. "$pgflux$" or "$$"
			cur.WriteString(tag)
			i = j + 1
			// Scan until the matching closing tag.
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
			s := strings.TrimSpace(cur.String())
			cur.Reset()
			// Strip leading comment lines to check if anything real is left.
			check := s
			for strings.HasPrefix(check, "--") {
				if nl := strings.Index(check, "\n"); nl >= 0 {
					check = strings.TrimSpace(check[nl+1:])
				} else {
					check = ""
					break
				}
			}
			if check != "" {
				stmts = append(stmts, s)
			}
			i++

		default:
			cur.WriteByte(ch)
			i++
		}
	}

	// Trailing content without a final semicolon.
	if s := strings.TrimSpace(cur.String()); s != "" {
		check := s
		for strings.HasPrefix(check, "--") {
			if nl := strings.Index(check, "\n"); nl >= 0 {
				check = strings.TrimSpace(check[nl+1:])
			} else {
				check = ""
				break
			}
		}
		if check != "" {
			stmts = append(stmts, s)
		}
	}

	return stmts
}

// isConcurrent reports whether a single SQL statement uses CONCURRENTLY.
var reConcurrently = regexp.MustCompile(`(?i)\bCONCURRENTLY\b`)

func isConcurrent(stmt string) bool {
	return reConcurrently.MatchString(stmt)
}


// migrationFiles returns sorted absolute paths of *.sql files in dir.
func migrationFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read migrations dir %s: %w", dir, err)
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(files)
	return files, nil
}

func logf(w io.Writer, format string, args ...any) {
	if w != nil {
		fmt.Fprintf(w, format, args...)
	}
}
