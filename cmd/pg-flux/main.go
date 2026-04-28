package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/nexg/pg-flux/pkg/db"
	"github.com/nexg/pg-flux/pkg/differ"
	"github.com/nexg/pg-flux/pkg/exec"
	"github.com/nexg/pg-flux/pkg/hashstate"
	"github.com/nexg/pg-flux/pkg/inspector"
	"github.com/nexg/pg-flux/pkg/migrate"
	"github.com/nexg/pg-flux/pkg/plan"
	"github.com/nexg/pg-flux/pkg/render"
	"github.com/nexg/pg-flux/pkg/schema"
	"github.com/nexg/pg-flux/pkg/shadow"
	"github.com/nexg/pg-flux/pkg/src"
)

// Version is the build-time version string (set via -ldflags).
var Version = "dev"

var (
	globalFormat     string
	dbURL            string
	schemaPath       string
	schemaFile       string
	allowHaz         string
	schemasFlag      string
	validatePlpgsqlF bool
	validateSQLF     bool
	appendValidateF  bool
	reltupleThresh   float64
	shadowDSN        string
	shadowSemanticF  bool
	shadowEquivF     bool
	configFile       string
	migrationsDir    string
	trackingSchema   string
)

// pgfluxConfig mirrors the .pg-flux.yml config file format.
type pgfluxConfig struct {
	Version        int      `yaml:"version"`
	DatabaseURL    string   `yaml:"db"`
	SchemaDir      string   `yaml:"schema_dir"`
	TargetSchemas  []string `yaml:"target_schemas"`
	MigrationsDir  string   `yaml:"migrations_dir"`
	TrackingSchema string   `yaml:"tracking_schema"`
}

// loadConfig reads a .pg-flux.yml config file. Missing file is not an error.
func loadConfig(path string) (*pgfluxConfig, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &pgfluxConfig{}, nil
		}
		return nil, fmt.Errorf("config %s: %w", path, err)
	}
	var c pgfluxConfig
	if err := yaml.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("config %s: %w", path, err)
	}
	return &c, nil
}

// errDriftDetected is a sentinel returned by cmdDrift when schema differs from desired state.
// main() translates this to exit code 1 without printing a redundant message.
var errDriftDetected = errors.New("drift: schema has changed")

func main() {
	if err := newRoot().Execute(); err != nil {
		if errors.Is(err, errDriftDetected) {
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRoot() *cobra.Command {
	r := &cobra.Command{Use: "pg-flux", Short: "Declarative PostgreSQL schema migration"}
	r.PersistentFlags().StringVar(&globalFormat, "format", "human", "human or json")
	r.PersistentFlags().StringVar(&dbURL, "db", os.Getenv("DATABASE_URL"), "database URL")
	r.PersistentFlags().StringVar(&schemaPath, "schema", "./schema", "schema directory (see also --schema-dir)")
	r.PersistentFlags().StringVar(&schemaPath, "schema-dir", "./schema", "schema directory (PRD: same as --schema)")
	r.PersistentFlags().StringVar(&schemaFile, "schema-file", "", "single .sql file")
	r.PersistentFlags().StringVar(&allowHaz, "allow-hazards", "", "allowed hazards, comma-separated")
	r.PersistentFlags().StringVar(&schemasFlag, "schemas", "public", "database schemas (comma list)")
	r.PersistentFlags().BoolVar(&validatePlpgsqlF, "validate-plpgsql", false, "parse-check LANGUAGE plpgsql functions with pg_query (stricter load)")
	r.PersistentFlags().BoolVar(&validateSQLF, "validate-sql", false, "re-parse each top-level statement (pg_query FR-01 check)")
	r.PersistentFlags().BoolVar(&appendValidateF, "append-validate-not-valid", false, "emit synthetic VALIDATE CONSTRAINT after ADD ... NOT VALID")
	r.PersistentFlags().Float64Var(&reltupleThresh, "set-not-null-reltuple-hint", 0, "if >0, advisory STAGED_SET_NOT_NULL when reltuples exceeds this (SET NOT NULL on large tables)")
	r.PersistentFlags().StringVar(&shadowDSN, "shadow-dsn", os.Getenv("PGFLUX_SHADOW_DSN"), "optional disposable DB DSN for shadow validation (see --shadow-semantic, --shadow-equivalence)")
	r.PersistentFlags().BoolVar(&shadowSemanticF, "shadow-semantic", false, "if set with --shadow-dsn, apply the plan with autocommit on that DB (mutates DB — use disposable instance; stronger than rolled-back syntax check)")
	r.PersistentFlags().BoolVar(&shadowEquivF, "shadow-equivalence", false, "with --shadow-dsn, run semantic apply on an empty shadow DB then require inspected catalog to match desired (structural check; not a formal proof vs production)")
	r.PersistentFlags().StringVar(&configFile, "config", ".pg-flux.yml", "path to config file")
	r.PersistentFlags().StringVar(&migrationsDir, "migrations-dir", "./migrations", "directory for migration .sql files")
	r.PersistentFlags().StringVar(&trackingSchema, "tracking-schema", "_pgflux", "schema used to track applied migrations")
	r.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig(configFile)
		if err != nil {
			return err
		}
		pf := r.PersistentFlags()
		// Apply config file values only when the corresponding CLI flag was not
		// explicitly provided (config < CLI flag precedence).
		if cfg.SchemaDir != "" && !pf.Changed("schema") && !pf.Changed("schema-dir") {
			schemaPath = cfg.SchemaDir
		}
		if len(cfg.TargetSchemas) > 0 && !pf.Changed("schemas") {
			schemasFlag = strings.Join(cfg.TargetSchemas, ",")
		}
		if cfg.MigrationsDir != "" && !pf.Changed("migrations-dir") {
			migrationsDir = cfg.MigrationsDir
		}
		if cfg.TrackingSchema != "" && !pf.Changed("tracking-schema") {
			trackingSchema = cfg.TrackingSchema
		}
		if cfg.DatabaseURL != "" && !pf.Changed("db") {
			dbURL = cfg.DatabaseURL
		}
		return nil
	}
	r.AddCommand(cmdInit(), cmdPlan(), cmdApply(), cmdDrift(), cmdInspect(), cmdMigrate(), cmdVersion())
	return r
}

func parseSchemas() []string {
	if strings.TrimSpace(schemasFlag) == "" {
		return []string{"public"}
	}
	var o []string
	for _, s := range strings.Split(schemasFlag, ",") {
		if t := strings.TrimSpace(s); t != "" {
			o = append(o, t)
		}
	}
	return o
}

func loadDesired() (*schema.SchemaState, error) {
	return src.LoadDesiredState(src.LoadOptions{
		SchemaDir: schemaPath, SchemaFile: schemaFile,
		ValidatePlpgsql: validatePlpgsqlF, ValidateSQL: validateSQLF,
	})
}

func cmdInit() *cobra.Command {
	var dir, migrDir, dbname string
	withEx := true
	c := &cobra.Command{
		Use:   "init",
		Short: "Scaffold .pg-flux.yml, schema dir, and migrations dir",
		RunE: func(cmd *cobra.Command, args []string) error {
			stdin := bufio.NewReader(os.Stdin)

			// Prompt for schema dir when interactive and flag not set.
			if !cmd.Flags().Changed("dir") && isTerminal(os.Stdin) {
				fmt.Fprintf(cmd.OutOrStdout(), "Schema directory [%s]: ", dir)
				if line, _ := stdin.ReadString('\n'); strings.TrimSpace(line) != "" {
					dir = strings.TrimSpace(line)
				}
			}
			// Prompt for migrations dir when interactive and flag not set.
			if !cmd.Flags().Changed("migrations-dir") && isTerminal(os.Stdin) {
				fmt.Fprintf(cmd.OutOrStdout(), "Migrations directory [%s]: ", migrDir)
				if line, _ := stdin.ReadString('\n'); strings.TrimSpace(line) != "" {
					migrDir = strings.TrimSpace(line)
				}
			}

			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
			if err := os.MkdirAll(migrDir, 0o755); err != nil {
				return err
			}

			cfgPath := ".pg-flux.yml"
			if _, err := os.Stat(cfgPath); err == nil {
				return fmt.Errorf("refusing to overwrite %s", cfgPath)
			}
			cfgContent := fmt.Sprintf(
				"version: 1\nschema_dir: %s\nmigrations_dir: %s\ntarget_schemas: [ public ]\n",
				dir, migrDir)
			if err := os.WriteFile(cfgPath, []byte(cfgContent), 0o644); err != nil {
				return err
			}

			if withEx {
				ex := "-- Example table. Rename or replace with your own schema.\n" +
					"CREATE TABLE public.users (\n" +
					"  id         bigserial   PRIMARY KEY,\n" +
					"  email      text        NOT NULL,\n" +
					"  username   text        NOT NULL,\n" +
					"  created_at timestamptz NOT NULL DEFAULT now(),\n\n" +
					"  CONSTRAINT users_email_unique    UNIQUE (email),\n" +
					"  CONSTRAINT users_username_unique UNIQUE (username),\n" +
					"  CONSTRAINT users_email_format    CHECK  (email LIKE '%@%')\n" +
					");\n\n" +
					"CREATE INDEX idx_users_email   ON public.users (email);\n" +
					"CREATE INDEX idx_users_created ON public.users (created_at DESC);\n"
				_ = os.WriteFile(dir+"/users.sql", []byte(ex), 0o644)
			}

			fmt.Fprintf(cmd.OutOrStdout(),
				"Initialized (db-name=%q).\n  schema_dir: %s\n  migrations_dir: %s\nNext: pg-flux plan --db $DATABASE_URL\n",
				dbname, dir, migrDir)
			return nil
		},
	}
	c.Flags().StringVar(&dir, "dir", "./schema", "schema directory")
	c.Flags().StringVar(&migrDir, "migrations-dir", "./migrations", "migrations directory")
	c.Flags().StringVar(&dbname, "db-name", "myapp", "label in messages")
	c.Flags().BoolVar(&withEx, "with-examples", true, "write example_users.sql")
	return c
}

// isTerminal returns true when f is a real TTY.
func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func cmdMigrate() *cobra.Command {
	m := &cobra.Command{
		Use:   "migrate",
		Short: "Manage timestamped migration files",
	}
	m.AddCommand(cmdMigrateGenerate(), cmdMigrateApply(), cmdMigrateStatus(), cmdMigrateRepair(), cmdMigrateBaseline())
	return m
}

func cmdVersion() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print pg-flux version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(cmd.OutOrStdout(), "pg-flux %s\n", Version)
		},
	}
}

func cmdMigrateGenerate() *cobra.Command {
	var label string
	var generateUndo bool
	c := &cobra.Command{
		Use:   "generate",
		Short: "Generate a new migration file from schema diff",
		RunE: func(cmd *cobra.Command, args []string) error {
			des, err := loadDesired()
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			pool, err := db.NewPool(ctx, dbURL)
			if err != nil {
				return err
			}
			defer pool.Close()
			res, err := migrate.Generate(ctx, pool, des, migrate.GenerateOptions{
				MigrationsDir: migrationsDir,
				Label:         label,
				Schemas:       parseSchemas(),
			})
			if err != nil {
				return err
			}
			if res.Filename == "" {
				fmt.Fprintln(cmd.OutOrStdout(), "No changes detected — no migration generated.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Generated: %s (%d statements)\n", res.Filename, len(res.Statements))
			if generateUndo {
				undoFile, err := migrate.WriteUndoFile(res)
				if err != nil {
					return fmt.Errorf("write undo file: %w", err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Undo:      %s\n", undoFile)
			}
			return nil
		},
	}
	c.Flags().StringVar(&label, "label", "", "short description appended to the filename")
	c.Flags().BoolVar(&generateUndo, "generate-undo", false, "also write a best-effort undo/rollback .sql file alongside the migration")
	return c
}

func cmdMigrateApply() *cobra.Command {
	var dry bool
	var shadowDSNFlag string
	c := &cobra.Command{
		Use:   "apply",
		Short: "Apply pending migration files to the database",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
			defer cancel()
			pool, err := db.NewPool(ctx, dbURL)
			if err != nil {
				return err
			}
			defer pool.Close()
			res, err := migrate.Apply(ctx, pool, migrate.ApplyOptions{
				MigrationsDir:  migrationsDir,
				TrackingSchema: trackingSchema,
				DryRun:         dry,
				ShadowDSN:      shadowDSNFlag,
				Progress:       cmd.OutOrStdout(),
			})
			if err != nil {
				return err
			}
			if dry {
				fmt.Fprintf(cmd.OutOrStdout(), "\nDry run: %d pending, %d already applied.\n", len(res.Applied), len(res.Skipped))
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "\nApplied %d migration(s), %d already up to date.\n", len(res.Applied), len(res.Skipped))
			}
			return nil
		},
	}
	c.Flags().BoolVar(&dry, "dry-run", false, "show what would be applied without executing")
	c.Flags().StringVar(&shadowDSNFlag, "shadow-dsn", os.Getenv("PGFLUX_SHADOW_DSN"),
		"optional disposable DB DSN: validate each pending migration in a rolled-back transaction before applying to the live DB")
	return c
}

func cmdMigrateStatus() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show applied / pending migration files",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			pool, err := db.NewPool(ctx, dbURL)
			if err != nil {
				return err
			}
			defer pool.Close()
			statuses, err := migrate.Status(ctx, pool, migrate.StatusOptions{
				MigrationsDir:  migrationsDir,
				TrackingSchema: trackingSchema,
			})
			if err != nil {
				return err
			}
			if len(statuses) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No migration files found.")
				return nil
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "STATUS\tFILENAME\tAPPLIED AT")
			for _, s := range statuses {
				status := "pending"
				at := ""
				if s.Applied {
					status = "applied"
					at = s.AppliedAt
				}
				fmt.Fprintf(w, "%s\t%s\t%s\n", status, s.Filename, at)
			}
			return w.Flush()
		},
	}
}

func cmdMigrateRepair() *cobra.Command {
	var file string
	c := &cobra.Command{
		Use:   "repair",
		Short: "Update stored checksum for a migration file that was edited after apply",
		Long: `Repair re-hashes the current on-disk content of a migration file and updates
the checksum stored in the tracking table. Use this only when you have deliberately
edited an already-applied migration and accept that the recorded history no longer
matches the original content.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			pool, err := db.NewPool(ctx, dbURL)
			if err != nil {
				return err
			}
			defer pool.Close()
			repaired, err := migrate.Repair(ctx, pool, migrate.RepairOptions{
				MigrationsDir:  migrationsDir,
				TrackingSchema: trackingSchema,
				Filename:       file,
			})
			if err != nil {
				return err
			}
			if len(repaired) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No checksum mismatches found — nothing to repair.")
				return nil
			}
			for _, f := range repaired {
				fmt.Fprintf(cmd.OutOrStdout(), "repaired  %s\n", f)
			}
			return nil
		},
	}
	c.Flags().StringVar(&file, "file", "", "repair only this specific filename (default: all mismatches)")
	return c
}

func cmdMigrateBaseline() *cobra.Command {
	var upTo string
	c := &cobra.Command{
		Use:   "baseline",
		Short: "Mark existing migration files as applied without executing them",
		Long: `Baseline is used when onboarding an existing database that was set up
manually or by another tool. It marks migration files as applied in the tracking
table so pg-flux does not attempt to re-run them.

Use --up-to to baseline only migrations up to (and including) a specific filename.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			pool, err := db.NewPool(ctx, dbURL)
			if err != nil {
				return err
			}
			defer pool.Close()
			baselined, err := migrate.Baseline(ctx, pool, migrate.BaselineOptions{
				MigrationsDir:  migrationsDir,
				TrackingSchema: trackingSchema,
				UpTo:           upTo,
			})
			if err != nil {
				return err
			}
			if len(baselined) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No pending migrations to baseline.")
				return nil
			}
			for _, f := range baselined {
				fmt.Fprintf(cmd.OutOrStdout(), "baselined  %s\n", f)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\nBaselined %d migration(s).\n", len(baselined))
			return nil
		},
	}
	c.Flags().StringVar(&upTo, "up-to", "", "baseline only files up to and including this filename")
	return c
}

func cmdPlan() *cobra.Command {
	return &cobra.Command{
		Use:   "plan",
		Short: "Compute diff and execution plan",
		RunE: func(cmd *cobra.Command, args []string) error {
			des, err := loadDesired()
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			pool, err := db.NewPool(ctx, dbURL)
			if err != nil {
				return err
			}
			defer pool.Close()
			live, err := inspector.Inspect(ctx, pool, inspector.Options{Schemas: parseSchemas()})
			if err != nil {
				return err
			}
			dopt, err := differOptions(ctx, pool, live)
			if err != nil {
				return err
			}
			dr, err := differ.Diff(des, live, dopt)
			if err != nil {
				return err
			}
			if shadowDSN != "" {
				if shadowEquivF {
					if err := shadow.ValidateStructuralEquivalence(ctx, shadowDSN, des, dr.Plan, dopt); err != nil {
						return fmt.Errorf("shadow structural equivalence: %w", err)
					}
				} else if shadowSemanticF {
					if err := shadow.ValidateSemanticOnDatabase(ctx, shadowDSN, dr.Plan); err != nil {
						return fmt.Errorf("shadow semantic apply: %w", err)
					}
				} else {
					if err := shadow.ValidateSyntaxOnDatabase(ctx, shadowDSN, dr.Plan); err != nil {
						return fmt.Errorf("shadow syntax validate: %w", err)
					}
				}
			}
			allow := render.ParseAllowHazards(allowHaz)
			srcH := hashstate.OfSchemaState(des)
			liveH := hashstate.OfSchemaState(live)
			if globalFormat == "json" {
				return render.PlanToJSON(cmd.OutOrStdout(), dr.Plan, srcH, liveH, allow)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "source_hash=%s live_hash=%s\n", srcH[:12], liveH[:12])
			if dr.Plan.HasBlockingHazards(allow) {
				fmt.Fprintln(cmd.OutOrStdout(), "Blocking hazards present; use --allow-hazards or fix plan.")
			}
			for _, s := range dr.Plan.Statements {
				fmt.Fprintf(cmd.OutOrStdout(), "[%d] %s\n", s.ID, s.DDL)
			}
			if dr.Plan.HasBlockingHazards(allow) {
				os.Exit(1)
			}
			return nil
		},
	}
}

func cmdApply() *cobra.Command {
	var dry bool
	var stmtTimeout string
	c := &cobra.Command{
		Use:   "apply",
		Short: "Apply planned DDL",
		RunE: func(cmd *cobra.Command, args []string) error {
			dr, err := runDiff()
			if err != nil {
				return err
			}
			allow := render.ParseAllowHazards(allowHaz)
			if dr.Plan.HasBlockingHazards(allow) {
				return fmt.Errorf("refusing to apply: blocking hazards; pass --allow-hazards or change schema")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
			defer cancel()
			pool, err := db.NewPool(ctx, dbURL)
			if err != nil {
				return err
			}
			defer pool.Close()
			return exec.Apply(ctx, pool, dr.Plan, exec.Options{
				DryRun:           dry,
				StatementTimeout: stmtTimeout,
				Progress:         cmd.OutOrStdout(),
			})
		},
	}
	c.Flags().BoolVar(&dry, "dry-run", false, "do not execute")
	c.Flags().StringVar(&stmtTimeout, "statement-timeout", "0", "per-statement timeout passed to SET LOCAL statement_timeout (e.g. 20min; 0 = unlimited)")
	return c
}

func cmdDrift() *cobra.Command {
	c := &cobra.Command{
		Use:          "drift",
		Short:        "Exit 1 if live DB differs from desired SQL",
		SilenceErrors: true, // main() handles errDriftDetected without printing it
		RunE: func(cmd *cobra.Command, args []string) error {
			dr, err := runDiff()
			if err != nil {
				return err
			}
			diffs := diffSummary(dr.Plan)
			if globalFormat == "json" {
				if err := render.DriftToJSON(cmd.OutOrStdout(), render.DriftJSON{IsDrift: len(diffs) > 0, Differences: diffs}); err != nil {
					return err
				}
				if len(diffs) > 0 {
					return errDriftDetected
				}
				return nil
			}
			if len(diffs) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No drift.")
				return nil
			}
			for _, d := range diffs {
				fmt.Fprintf(cmd.OutOrStdout(), "%s %s: %s\n", d.ObjectType, d.ObjectName, d.Details)
			}
			return errDriftDetected
		},
	}
	return c
}

func cmdInspect() *cobra.Command {
	var out string
	c := &cobra.Command{
		Use:   "inspect",
		Short: "Dump table DDL skeletons to --out (subset)",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			pool, err := db.NewPool(ctx, dbURL)
			if err != nil {
				return err
			}
			defer pool.Close()
			live, err := inspector.Inspect(ctx, pool, inspector.Options{Schemas: parseSchemas()})
			if err != nil {
				return err
			}
			_ = os.MkdirAll(out+"/tables", 0o755)
			for k, t := range live.Tables {
				if t == nil {
					continue
				}
				var b strings.Builder
				fmt.Fprintf(&b, "CREATE TABLE %s (\n", t.Name)
				for i, c := range t.Columns {
					if c == nil {
						continue
					}
					if i > 0 {
						b.WriteString(",\n")
					}
					fmt.Fprintf(&b, "  %s %s", c.Name, c.TypeSQL)
					if c.NotNull {
						b.WriteString(" NOT NULL")
					}
				}
				b.WriteString("\n);\n")
				safe := strings.NewReplacer(".", "_").Replace(k)
				_ = os.WriteFile(out+"/tables/"+safe+".sql", []byte(b.String()), 0o644)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Wrote table stubs under %s/tables/\n", out)
			return nil
		},
	}
	c.Flags().StringVar(&out, "out", "./schema", "output directory")
	return c
}

func runDiff() (*differ.DiffResult, error) {
	des, err := loadDesired()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool, err := db.NewPool(ctx, dbURL)
	if err != nil {
		return nil, err
	}
	defer pool.Close()
	live, err := inspector.Inspect(ctx, pool, inspector.Options{Schemas: parseSchemas()})
	if err != nil {
		return nil, err
	}
	dopt, err := differOptions(ctx, pool, live)
	if err != nil {
		return nil, err
	}
	return differ.Diff(des, live, dopt)
}

func differOptions(ctx context.Context, pool *pgxpool.Pool, live *schema.SchemaState) (differ.Options, error) {
	opt := differ.Options{
		AppendValidateAfterNotValid: appendValidateF,
		SetNotNullReltupleThreshold: reltupleThresh,
	}
	if reltupleThresh <= 0 || pool == nil || live == nil {
		return opt, nil
	}
	m, err := inspector.ReltuplesByTable(ctx, pool, live.Tables)
	if err != nil {
		return opt, err
	}
	opt.Reltuples = m
	return opt, nil
}

func diffSummary(p *plan.ExecutionPlan) []render.Difference {
	if p == nil {
		return nil
	}
	var o []render.Difference
	for _, s := range p.Statements {
		if s.DDL == "" {
			continue
		}
		o = append(o, render.Difference{ObjectType: s.OpType, ObjectName: s.Object, ChangeType: s.OpType, Details: s.DDL})
	}
	return o
}
