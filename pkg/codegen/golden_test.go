package codegen

import (
	"flag"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/nexg/pg-flux/pkg/schema"
)

// updateGolden toggles refresh mode: `go test -tags='' ./pkg/codegen/ -run Golden -update`
// rewrites the testdata files to match current emitter output. Without -update
// the tests fail when output differs, which is the correctness gate.
var updateGolden = flag.Bool("update", false, "refresh codegen golden files")

// fixtureState builds a representative SchemaState exercising every emitter
// branch: tables with NULL/NOT NULL/comments/PK/identity, an enum, a composite
// type, a domain, a view, all in the public schema.
func fixtureState() *schema.SchemaState {
	return &schema.SchemaState{
		Tables: map[string]*schema.Table{
			"public.users": {
				Schema: "public", Name: "users",
				Comment: "Application user accounts.",
				Columns: []*schema.Column{
					{Name: "id", TypeSQL: "bigint", NotNull: true, IsPrimaryKey: true, Comment: "Stable user identifier."},
					{Name: "email", TypeSQL: "text", NotNull: true, Comment: "Unique login email."},
					{Name: "display_name", TypeSQL: "text"}, // nullable
					{Name: "role", TypeSQL: "public.user_role", NotNull: true, Comment: "pg-flux: gotype=UserRole tstype=UserRole"},
					{Name: "created_at", TypeSQL: "timestamptz", NotNull: true, DefaultSQL: "now()"},
					{Name: "deleted_at", TypeSQL: "timestamptz"},
					{Name: "metadata", TypeSQL: "jsonb"},
				},
			},
			"public.posts": {
				Schema: "public", Name: "posts",
				Columns: []*schema.Column{
					{Name: "id", TypeSQL: "bigint", NotNull: true, IsPrimaryKey: true},
					{Name: "title", TypeSQL: "varchar(200)", NotNull: true},
					{Name: "tags", TypeSQL: "text[]"},
					{Name: "view_count", TypeSQL: "integer", NotNull: true, DefaultSQL: "0"},
				},
			},
		},
		EnumValues: map[string][]string{
			"public.user_role": {"admin", "member", "guest"},
		},
		CompositeTypes: map[string]*schema.CompositeType{
			"public.address": {
				Schema: "public", Name: "address",
				Attributes: []schema.CompositeAttribute{
					{Name: "street", Type: "text"},
					{Name: "city", Type: "text"},
					{Name: "zip", Type: "varchar(10)"},
				},
			},
		},
		Domains: map[string]*schema.Domain{
			"public.short_text": {Schema: "public", Name: "short_text", BaseType: "text"},
		},
		Views: map[string]*schema.View{
			"public.active_users": {Schema: "public", Name: "active_users"},
		},
		Functions: map[string]*schema.Function{
			// Scalar return — emits a Row alias.
			"public.length_of(text)": {
				Schema: "public", Name: "length_of",
				Kind: "f", Identity: "public.length_of(text)",
				Args: []schema.FunctionArg{
					{Name: "s", Type: "text", Mode: "i"},
				},
				ReturnType: "integer",
			},
			// RETURNS TABLE — emits a Result struct/interface.
			"public.calculate_score(bigint,numeric)": {
				Schema: "public", Name: "calculate_score",
				Kind: "f", Identity: "public.calculate_score(bigint,numeric)",
				Args: []schema.FunctionArg{
					{Name: "user_id", Type: "bigint", Mode: "i"},
					{Name: "weight", Type: "numeric", Mode: "i", HasDefault: true},
				},
				ReturnsTable: []schema.FunctionArg{
					{Name: "score", Type: "numeric", Mode: "t"},
					{Name: "tier", Type: "text", Mode: "t"},
				},
				ReturnsSet: true,
			},
			// Procedure — Params only.
			"public.refresh_caches()": {
				Schema: "public", Name: "refresh_caches",
				Kind: "p", Identity: "public.refresh_caches()",
			},
			// Trigger function — skipped (void-like return).
			"public.bump_updated_at()": {
				Schema: "public", Name: "bump_updated_at",
				Kind: "f", Identity: "public.bump_updated_at()",
				ReturnType: "trigger",
			},
		},
	}
}

// TestGoldenGo asserts the Go emitter output matches testdata/golden/go/*.
func TestGoldenGo(t *testing.T) {
	g := NewGoGenerator()
	fs, err := g.Generate(fixtureState(), Options{Package: "dbgen"})
	if err != nil {
		t.Fatal(err)
	}
	compareGolden(t, "go", fs)
}

// TestGoldenTS asserts the TypeScript emitter output matches testdata/golden/ts/*.
func TestGoldenTS(t *testing.T) {
	g := NewTSGenerator()
	fs, err := g.Generate(fixtureState(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	compareGolden(t, "ts", fs)
}

// compareGolden either rewrites the golden files (-update) or asserts byte-equality.
// Missing-on-disk and missing-in-output are both reported.
func compareGolden(t *testing.T, lang string, fs FileSet) {
	t.Helper()
	dir := filepath.Join("testdata", "golden", lang)
	if *updateGolden {
		// Clean and rewrite.
		_ = os.RemoveAll(dir)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for path, content := range fs {
			full := filepath.Join(dir, path)
			if err := os.WriteFile(full, content, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		t.Logf("updated %d golden file(s) in %s", len(fs), dir)
		return
	}
	// Compare existing.
	got := make(map[string]bool, len(fs))
	for path, content := range fs {
		got[path] = true
		full := filepath.Join(dir, path)
		want, err := os.ReadFile(full)
		if err != nil {
			t.Errorf("missing golden %s: %v (re-run with -update to create)", full, err)
			continue
		}
		if !bytesEqual(want, content) {
			t.Errorf("golden mismatch for %s:\nWANT:\n%s\n\nGOT:\n%s", full, string(want), string(content))
		}
	}
	// Detect golden files that aren't emitted anymore.
	entries, _ := os.ReadDir(dir)
	var dropped []string
	for _, e := range entries {
		if !got[e.Name()] {
			dropped = append(dropped, e.Name())
		}
	}
	sort.Strings(dropped)
	for _, d := range dropped {
		t.Errorf("extra golden %s on disk but not emitted (re-run with -update)", d)
	}
}
