// Package codegen generates application-language type definitions (Go, TypeScript)
// from the pg-flux schema model. The same inspector pipeline that powers
// migrate/dump/verify drives codegen, so generated types always match what
// pg-flux trusts as the schema state.
//
// Workflow:
//
//	live or source SchemaState → Generator (per language) → FileSet → disk
//
// Run `pg-flux gen` after every migrate to regenerate; use `pg-flux gen --check`
// in CI to fail the build when the on-disk generated code doesn't match the
// schema (the codegen equivalent of "go generate" drift detection).
package codegen

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nexg/pg-flux/pkg/schema"
)

// TypeMap is the per-language PG-type → language-type resolver. Defined here
// (rather than in a separate file) so codegen.go compiles standalone; the
// concrete implementations live in typemap.go and per-emitter files.
type TypeMap interface {
	// Map returns the language-level type expression for a PG type. nullable=true
	// requests the optional/nullable form (Go pointer, TS `T | null`, etc.).
	// imports returns any package imports the type expression depends on; the
	// emitter aggregates these per file.
	Map(pgType string, nullable bool) (typeExpr string, imports []string)
}

// Language is the target language for code generation.
type Language string

const (
	LangGo         Language = "go"
	LangTypeScript Language = "ts"
)

// Generator is the per-language code emitter. Each language implementation
// (emit_go, emit_ts) registers itself by returning its Generator from a
// constructor. Generators must be deterministic — same SchemaState in must
// produce identical FileSet out, so --check can compare bytes safely.
type Generator interface {
	// Lang returns the language this generator emits.
	Lang() Language
	// Generate renders the schema into a FileSet (path → content).
	Generate(s *schema.SchemaState, opts Options) (FileSet, error)
}

// Options controls a single language's generation pass.
type Options struct {
	// OutDir is the destination directory. Files are written relative to it.
	OutDir string
	// Package is the language-specific package/module name (Go package,
	// TS namespace if any). Defaults vary per emitter.
	Package string
	// TypeMap is the resolved per-PG-type → language-type map. When nil,
	// the emitter's default map is used.
	TypeMap TypeMap
	// Schemas filters the schema kinds emitted by SQL schema (default: all).
	Schemas []string
}

// FileSet is the in-memory output of a Generator: relative paths → content bytes.
// Writing to disk happens through Write so the same FileSet powers both real
// emission and --check mode (which compares without writing).
type FileSet map[string][]byte

// Write materialises the FileSet under rootDir. Parent directories are created
// as needed. Existing files are overwritten only when their content differs,
// preserving mtimes for unchanged files (important for build-incremental tools).
func (fs FileSet) Write(rootDir string) (written, skipped int, err error) {
	for rel, content := range fs {
		full := filepath.Join(rootDir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return written, skipped, fmt.Errorf("mkdir %s: %w", filepath.Dir(full), err)
		}
		existing, _ := os.ReadFile(full)
		if existing != nil && bytesEqual(existing, content) {
			skipped++
			continue
		}
		if err := os.WriteFile(full, content, 0o644); err != nil {
			return written, skipped, fmt.Errorf("write %s: %w", full, err)
		}
		written++
	}
	return written, skipped, nil
}

// Check compares fs against the on-disk contents under rootDir and returns
// the relative paths whose content differs (or that are missing on disk).
// Used by `pg-flux gen --check` to gate CI on stale generated code.
func (fs FileSet) Check(rootDir string) (diffs []string, err error) {
	paths := make([]string, 0, len(fs))
	for p := range fs {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		full := filepath.Join(rootDir, p)
		existing, rerr := os.ReadFile(full)
		if rerr != nil {
			if os.IsNotExist(rerr) {
				diffs = append(diffs, p)
				continue
			}
			return diffs, fmt.Errorf("read %s: %w", full, rerr)
		}
		if !bytesEqual(existing, fs[p]) {
			diffs = append(diffs, p)
		}
	}
	return diffs, nil
}

// bytesEqual avoids a "bytes" import for one helper.
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// WriteSummary prints a human-readable summary of a Write or Check pass.
func WriteSummary(w io.Writer, lang Language, written, skipped int, diffs []string) {
	if len(diffs) > 0 {
		fmt.Fprintf(w, "[%s] %d file(s) differ:\n", lang, len(diffs))
		for _, d := range diffs {
			fmt.Fprintf(w, "  %s\n", d)
		}
		return
	}
	if written > 0 {
		fmt.Fprintf(w, "[%s] wrote %d, skipped %d (already up to date)\n", lang, written, skipped)
	} else {
		fmt.Fprintf(w, "[%s] %d file(s) already up to date\n", lang, skipped)
	}
}

// trimTrailingBlankLines normalises emitter output so trailing whitespace
// doesn't cause spurious diffs across editors. Returns the input with at most
// one trailing newline.
func trimTrailingBlankLines(s string) string {
	return strings.TrimRight(s, "\n") + "\n"
}
