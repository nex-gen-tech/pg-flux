package codegen

import (
	"os"
	"path/filepath"
	"testing"
)

// variantScenarios is the matrix of emit-option combinations whose output we
// lock down. Adding a new scenario is a one-liner: append a struct, run
// `go test ./pkg/codegen -run Variants -update`, review the diff, commit.
type variantScenario struct {
	name string
	lang Language
	opts EmitOptions
}

var variantScenarios = []variantScenario{
	{
		name: "ts_camel_number_zod",
		lang: LangTypeScript,
		opts: EmitOptions{
			ColumnCase:          ColumnCaseCamel,
			BigintAs:            "number",
			DateAs:              "string",
			NullStyle:           "optional",
			EnumStyle:           "const-object",
			BrandedIDs:          true,
			Readonly:            ReadonlyDefaults,
			InsertUpdateHelpers: true,
			Validators:          "zod",
		},
	},
	{
		name: "ts_undefined_ts_enum",
		lang: LangTypeScript,
		opts: EmitOptions{
			NullStyle: "undefined",
			EnumStyle: "ts-enum",
		},
	},
	{
		name: "go_gorm_omitempty",
		lang: LangGo,
		opts: EmitOptions{
			ORMTags:   "gorm",
			OmitEmpty: "defaults",
			Readonly:  ReadonlyIdentity,
		},
	},
	{
		name: "go_sqlx",
		lang: LangGo,
		opts: EmitOptions{
			ORMTags: "sqlx",
		},
	},
	{
		name: "go_bun",
		lang: LangGo,
		opts: EmitOptions{
			ORMTags: "bun",
		},
	},
	{
		name: "ts_filter_exclude",
		lang: LangTypeScript,
		opts: EmitOptions{
			Filter: Filter{ExcludeTables: []string{"posts"}},
		},
	},
	{
		name: "ts_with_functions",
		lang: LangTypeScript,
		opts: EmitOptions{
			Functions: true,
		},
	},
	{
		name: "go_with_functions",
		lang: LangGo,
		opts: EmitOptions{
			Functions: true,
		},
	},
}

// TestGoldenVariants exercises every entry of variantScenarios. Each scenario
// gets its own subdirectory under testdata/golden/variants/<name>/ so any
// option-shift shows up as a localised diff during review.
func TestGoldenVariants(t *testing.T) {
	for _, sc := range variantScenarios {
		t.Run(sc.name, func(t *testing.T) {
			var gen Generator
			switch sc.lang {
			case LangGo:
				gen = NewGoGenerator()
			case LangTypeScript:
				gen = NewTSGenerator()
			}
			fs, err := gen.Generate(fixtureState(), Options{Package: "dbgen", Emit: sc.opts})
			if err != nil {
				t.Fatal(err)
			}
			compareVariantGolden(t, sc.name, fs)
		})
	}
}

func compareVariantGolden(t *testing.T, name string, fs FileSet) {
	t.Helper()
	dir := filepath.Join("testdata", "golden", "variants", name)
	if *updateGolden {
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
	got := map[string]bool{}
	for path, content := range fs {
		got[path] = true
		full := filepath.Join(dir, path)
		want, err := os.ReadFile(full)
		if err != nil {
			t.Errorf("missing golden %s: %v (run with -update)", full, err)
			continue
		}
		if !bytesEqual(want, content) {
			t.Errorf("variant %s file %s differs:\nWANT:\n%s\n\nGOT:\n%s",
				name, path, string(want), string(content))
		}
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !got[e.Name()] {
			t.Errorf("variant %s: stale golden %s no longer emitted (run with -update)", name, e.Name())
		}
	}
}
