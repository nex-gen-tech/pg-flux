package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/cobra"

	"github.com/nexg/pg-flux/pkg/db"
	"github.com/nexg/pg-flux/pkg/differ"
	"github.com/nexg/pg-flux/pkg/exec"
	"github.com/nexg/pg-flux/pkg/hashstate"
	"github.com/nexg/pg-flux/pkg/inspector"
	"github.com/nexg/pg-flux/pkg/plan"
	"github.com/nexg/pg-flux/pkg/render"
	"github.com/nexg/pg-flux/pkg/schema"
	"github.com/nexg/pg-flux/pkg/shadow"
	"github.com/nexg/pg-flux/pkg/src"
)

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
)

func main() {
	if err := newRoot().Execute(); err != nil {
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
	r.AddCommand(cmdInit(), cmdPlan(), cmdApply(), cmdDrift(), cmdInspect())
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
	var dir, dbname string
	withEx := true
	c := &cobra.Command{
		Use:   "init",
		Short: "Scaffold .pg-flux.yml and example SQL",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
			if _, err := os.Stat(dir + "/.pg-flux.yml"); err == nil {
				return fmt.Errorf("refusing to overwrite %s/.pg-flux.yml", dir)
			}
			if err := os.WriteFile(dir+"/.pg-flux.yml", []byte("version: 1\nschema_dir: ./\ntarget_schemas: [ public ]\n"), 0o644); err != nil {
				return err
			}
			for _, sub := range []string{"tables", "functions", "policies", "indexes", "types"} {
				_ = os.MkdirAll(dir+"/"+sub, 0o755)
			}
			if withEx {
				ex := "-- Example\nCREATE TABLE example_users (\n  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),\n  email text NOT NULL\n);\n"
				_ = os.WriteFile(dir+"/tables/example_users.sql", []byte(ex), 0o644)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Initialized in %s (db-name=%q). Next: pg-flux plan --db $DATABASE_URL --schema %s\n", dir, dbname, dir)
			return nil
		},
	}
	c.Flags().StringVar(&dir, "dir", "./schema", "target directory")
	c.Flags().StringVar(&dbname, "db-name", "myapp", "label in messages")
	c.Flags().BoolVar(&withEx, "with-examples", true, "write example_users.sql")
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
	dry := false
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
			return exec.Apply(ctx, pool, dr.Plan, exec.Options{DryRun: dry})
		},
	}
	c.Flags().BoolVar(&dry, "dry-run", false, "do not execute")
	return c
}

func cmdDrift() *cobra.Command {
	return &cobra.Command{
		Use:   "drift",
		Short: "Exit 1 if live DB differs from desired SQL",
		RunE: func(cmd *cobra.Command, args []string) error {
			dr, err := runDiff()
			if err != nil {
				return err
			}
			diffs := diffSummary(dr.Plan)
			if globalFormat == "json" {
				return render.DriftToJSON(cmd.OutOrStdout(), render.DriftJSON{IsDrift: len(diffs) > 0, Differences: diffs})
			}
			if len(diffs) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No drift.")
				return nil
			}
			for _, d := range diffs {
				fmt.Fprintf(cmd.OutOrStdout(), "%s %s: %s\n", d.ObjectType, d.ObjectName, d.Details)
			}
			os.Exit(1)
			return nil
		},
	}
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
