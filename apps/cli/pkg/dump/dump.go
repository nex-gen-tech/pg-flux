// Package dump extracts a live PostgreSQL database's catalog state back into
// pg-flux-compatible source SQL files, so a team adopting pg-flux against an
// existing DB doesn't have to hand-translate hundreds of objects. The dump is
// round-trip clean: running "pg-flux migrate generate" immediately after a
// dump must produce zero pending changes.
package dump

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nex-gen-tech/pg-flux/pkg/inspector"
	"github.com/nex-gen-tech/pg-flux/pkg/obs"
	"github.com/nex-gen-tech/pg-flux/pkg/pgver"
	"github.com/nex-gen-tech/pg-flux/pkg/schema"
)

// Layout selects how the dump organises output files on disk.
type Layout string

const (
	// LayoutPerKind groups files into kind subdirectories:
	//   schema/tables/<sch>.<name>.sql
	//   schema/views/<sch>.<name>.sql
	//   schema/types/<sch>.<name>.sql
	// One object per file. The default.
	LayoutPerKind Layout = "per-kind"
	// LayoutFlat writes a single schema.sql containing every object, ordered
	// by dependency (types → tables → indexes → views → triggers → grants).
	// Useful for small schemas or one-off snapshots.
	LayoutFlat Layout = "flat"
)

// Options controls Dump.
type Options struct {
	OutputDir string   // target directory; created if missing
	Layout    Layout   // file organisation; default LayoutPerKind
	Schemas   []string // PG schemas to dump (default: ["public"])
	Force     bool     // overwrite if OutputDir is non-empty
}

// Result summarises what Dump wrote.
type Result struct {
	FilesWritten int
	Objects      int
	Layout       Layout
}

// Dump reads the live schema and writes pg-flux source files to opts.OutputDir.
// The output is loadable by pkg/src.LoadDesiredState and round-trips cleanly
// against the differ (round-trip is enforced by the integration test gate).
func Dump(ctx context.Context, pool *pgxpool.Pool, opts Options) (*Result, error) {
	if opts.OutputDir == "" {
		return nil, fmt.Errorf("dump: OutputDir is required")
	}
	if opts.Layout == "" {
		opts.Layout = LayoutPerKind
	}
	if len(opts.Schemas) == 0 {
		opts.Schemas = []string{"public"}
	}
	if err := guardOutputDir(opts.OutputDir, opts.Force); err != nil {
		return nil, err
	}

	live, err := inspector.Inspect(ctx, pool, inspector.Options{Schemas: opts.Schemas})
	if err != nil {
		return nil, fmt.Errorf("dump: inspect live: %w", err)
	}
	pv, _ := pgver.Detect(ctx, pool)

	objs := renderAll(live, pv)
	if err := writeObjects(opts.OutputDir, opts.Layout, objs); err != nil {
		return nil, err
	}

	obs.InfoCtx(ctx, "dump.completed",
		"output", opts.OutputDir,
		"layout", string(opts.Layout),
		"objects", len(objs),
	)
	return &Result{
		FilesWritten: countFiles(opts.OutputDir, opts.Layout, objs),
		Objects:      len(objs),
		Layout:       opts.Layout,
	}, nil
}

// object represents one rendered DDL chunk plus the kind / name used to
// organise it into a file.
type object struct {
	Kind   string // "tables", "views", "types", "functions", "indexes", ...
	Schema string
	Name   string
	// SortKey controls intra-kind ordering (e.g., domains before tables that use them).
	SortKey int
	// SQL is one or more complete statements terminated by `;\n`. May include
	// trailing COMMENT/GRANT/ALTER OWNER/etc. statements scoped to this object.
	SQL string
}

// renderAll walks every supported model field and produces source SQL.
func renderAll(s *schema.SchemaState, pgv pgver.Version) []object {
	var out []object
	out = append(out, renderExtensions(s)...)
	out = append(out, renderEnums(s)...)
	out = append(out, renderDomains(s)...)
	out = append(out, renderCompositeTypes(s)...)
	out = append(out, renderRangeTypes(s)...)
	out = append(out, renderSequences(s)...)
	out = append(out, renderTables(s, pgv)...)
	out = append(out, renderIndexes(s)...)
	out = append(out, renderViews(s)...)
	out = append(out, renderFunctions(s)...)
	out = append(out, renderTriggers(s)...)
	out = append(out, renderPolicies(s)...)
	out = append(out, renderEventTriggers(s)...)
	out = append(out, renderStatistics(s)...)
	out = append(out, renderForeignServers(s)...)
	out = append(out, renderForeignTables(s)...)
	out = append(out, renderDefaultPrivileges(s)...)
	return out
}

// guardOutputDir refuses to overwrite an existing non-empty directory unless
// force is set; creates it if missing.
func guardOutputDir(dir string, force bool) error {
	st, err := os.Stat(dir)
	if err == nil {
		if !st.IsDir() {
			return fmt.Errorf("dump: %s exists and is not a directory", dir)
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return fmt.Errorf("dump: read %s: %w", dir, err)
		}
		nonHidden := 0
		for _, e := range entries {
			if !strings.HasPrefix(e.Name(), ".") {
				nonHidden++
			}
		}
		if nonHidden > 0 && !force {
			return fmt.Errorf(
				"dump: %s is not empty (%d entries). "+
					"Re-run with --force to overwrite or choose a different --output",
				dir, nonHidden)
		}
		return nil
	}
	if os.IsNotExist(err) {
		return os.MkdirAll(dir, 0o755)
	}
	return err
}

// writeObjects materialises rendered objects to disk per the layout strategy.
func writeObjects(dir string, layout Layout, objs []object) error {
	switch layout {
	case LayoutFlat:
		return writeFlat(dir, objs)
	default:
		return writePerKind(dir, objs)
	}
}

func writePerKind(dir string, objs []object) error {
	// Group by kind.
	byKind := map[string][]object{}
	for _, o := range objs {
		byKind[o.Kind] = append(byKind[o.Kind], o)
	}
	for kind, list := range byKind {
		sortObjects(list)
		kdir := filepath.Join(dir, kind)
		if err := os.MkdirAll(kdir, 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", kdir, err)
		}
		for _, o := range list {
			fname := filepath.Join(kdir, fileNameFor(o)+".sql")
			if err := os.WriteFile(fname, []byte(o.SQL), 0o644); err != nil {
				return fmt.Errorf("write %s: %w", fname, err)
			}
		}
	}
	return nil
}

func writeFlat(dir string, objs []object) error {
	sortObjects(objs)
	// Group by kind for section headers.
	var b strings.Builder
	b.WriteString("-- pg-flux dump (flat layout)\n")
	b.WriteString("-- regenerated; edit at your own risk\n\n")
	var prevKind string
	for _, o := range objs {
		if o.Kind != prevKind {
			fmt.Fprintf(&b, "\n-- ============================================================\n")
			fmt.Fprintf(&b, "-- %s\n", strings.ToUpper(o.Kind))
			fmt.Fprintf(&b, "-- ============================================================\n\n")
			prevKind = o.Kind
		}
		b.WriteString(o.SQL)
		b.WriteString("\n")
	}
	return os.WriteFile(filepath.Join(dir, "schema.sql"), []byte(b.String()), 0o644)
}

func countFiles(dir string, layout Layout, objs []object) int {
	switch layout {
	case LayoutFlat:
		return 1
	default:
		return len(objs)
	}
}

// sortObjects stably orders objects by (Kind, SortKey, Schema, Name).
func sortObjects(o []object) {
	sort.SliceStable(o, func(i, j int) bool {
		if o[i].Kind != o[j].Kind {
			return o[i].Kind < o[j].Kind
		}
		if o[i].SortKey != o[j].SortKey {
			return o[i].SortKey < o[j].SortKey
		}
		if o[i].Schema != o[j].Schema {
			return o[i].Schema < o[j].Schema
		}
		return o[i].Name < o[j].Name
	})
}

// fileNameFor produces a filesystem-safe "<schema>.<name>" base name. Identifiers
// containing characters that are unsafe in file paths get replaced with `_`.
func fileNameFor(o object) string {
	name := o.Name
	if o.Schema != "" {
		name = o.Schema + "." + o.Name
	}
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '.', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}
