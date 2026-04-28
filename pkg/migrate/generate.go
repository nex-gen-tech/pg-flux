package migrate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nexg/pg-flux/pkg/differ"
	"github.com/nexg/pg-flux/pkg/inspector"
	"github.com/nexg/pg-flux/pkg/plan"
	"github.com/nexg/pg-flux/pkg/schema"
)

// GenerateOptions controls migration file generation.
type GenerateOptions struct {
	// MigrationsDir is the folder where the .sql file will be written.
	MigrationsDir string
	// Label is appended to the timestamp in the filename (optional).
	Label string
	// Schemas is the list of PostgreSQL schemas to inspect (default: ["public"]).
	Schemas []string
	// AllowHazards is passed through to the differ (comma-separated list).
	AllowHazards string
}

// GenerateResult is returned by Generate.
type GenerateResult struct {
	// Filename is the path of the written migration file (empty if nothing to generate).
	Filename string
	// SQL is the content that was written.
	SQL string
	// Statements is the ordered list of plan statements included.
	Statements []plan.Statement
}

// Generate diffs the desired state against the live database and writes a
// timestamped migration file. Returns a GenerateResult with an empty Filename
// when there are no differences.
func Generate(
	ctx context.Context,
	pool *pgxpool.Pool,
	desired *schema.SchemaState,
	opts GenerateOptions,
) (*GenerateResult, error) {
	if len(opts.Schemas) == 0 {
		opts.Schemas = []string{"public"}
	}

	live, err := inspector.Inspect(ctx, pool, inspector.Options{Schemas: opts.Schemas})
	if err != nil {
		return nil, fmt.Errorf("inspect live schema: %w", err)
	}

	diffResult, err := differ.Diff(desired, live, differ.Options{})
	if err != nil {
		return nil, fmt.Errorf("diff: %w", err)
	}

	if len(diffResult.Plan.Statements) == 0 {
		return &GenerateResult{}, nil
	}

	// Advisory-only plans (no actual DDL to execute) should not generate a file.
	// Advisories are surfaced as comments when bundled with real changes.
	hasActionable := false
	for _, s := range diffResult.Plan.Statements {
		if strings.TrimSpace(s.DDL) != "" {
			hasActionable = true
			break
		}
	}
	if !hasActionable {
		return &GenerateResult{}, nil
	}

	sql := buildMigrationSQL(diffResult.Plan)
	filename := TimestampFilename(opts.Label)
	fullPath := filepath.Join(opts.MigrationsDir, filename)

	if err := os.MkdirAll(opts.MigrationsDir, 0o755); err != nil {
		return nil, fmt.Errorf("create migrations dir: %w", err)
	}
	if err := os.WriteFile(fullPath, []byte(sql), 0o644); err != nil {
		return nil, fmt.Errorf("write migration file: %w", err)
	}

	return &GenerateResult{
		Filename:   fullPath,
		SQL:        sql,
		Statements: diffResult.Plan.Statements,
	}, nil
}

// buildMigrationSQL renders the plan statements as SQL with comments.
func buildMigrationSQL(p *plan.ExecutionPlan) string {
	var b strings.Builder
	b.WriteString("-- pg-flux generated migration\n")
	b.WriteString("-- DO NOT EDIT unless you know what you are doing.\n\n")
	for _, s := range p.Statements {
		// Emit advisory hazard notices as SQL comments (no DDL to execute).
		for _, h := range s.Hazards {
			if h.Severity == "advisory" {
				fmt.Fprintf(&b, "-- [ADVISORY %s] %s\n", h.Type, h.Message)
			}
		}
		if s.DDL == "" {
			// Advisory-only statement: already emitted above, add blank line separator.
			if len(s.Hazards) > 0 {
				b.WriteString("\n")
			}
			continue
		}
		fmt.Fprintf(&b, "-- [%d] %s: %s\n", s.ID, s.OpType, s.Object)
		ddl := strings.TrimRight(s.DDL, ";")
		b.WriteString(ddl)
		b.WriteString(";\n\n")
	}
	return b.String()
}
