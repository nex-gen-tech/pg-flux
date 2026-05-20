package codegen

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadConfig_missingFile(t *testing.T) {
	c, err := LoadConfig(filepath.Join(t.TempDir(), "nope.yml"))
	if err != nil {
		t.Fatalf("missing file should be (nil, nil); got err=%v", err)
	}
	if c != nil {
		t.Fatalf("missing file should return nil config; got %+v", c)
	}
}

func TestLoadConfig_basic(t *testing.T) {
	p := filepath.Join(t.TempDir(), "cfg.yml")
	os.WriteFile(p, []byte(`outputs:
  - lang: go
    out: ./internal/dbgen
    package: dbgen
    type_overrides:
      numeric: github.com/shopspring/decimal.Decimal
  - lang: ts
    out: ./src/generated
`), 0o644)
	c, err := LoadConfig(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Outputs) != 2 {
		t.Fatalf("expected 2 outputs, got %d", len(c.Outputs))
	}
	if c.Outputs[0].Lang != LangGo || c.Outputs[0].Package != "dbgen" {
		t.Fatalf("go output mis-parsed: %+v", c.Outputs[0])
	}
	if got := c.Outputs[0].TypeOverrides["numeric"]; got != "github.com/shopspring/decimal.Decimal" {
		t.Fatalf("type override missing: %v", c.Outputs[0].TypeOverrides)
	}
}

func TestLoadConfig_missingLang(t *testing.T) {
	p := filepath.Join(t.TempDir(), "cfg.yml")
	os.WriteFile(p, []byte(`outputs:
  - out: ./foo
`), 0o644)
	_, err := LoadConfig(p)
	if err == nil {
		t.Fatal("expected error for missing lang")
	}
}

func TestParseCommentHints_plainDoc(t *testing.T) {
	h := ParseCommentHints("user email address")
	if h.Doc != "user email address" {
		t.Errorf("doc: %q", h.Doc)
	}
	if h.Overrides != nil {
		t.Errorf("expected nil overrides, got %v", h.Overrides)
	}
}

func TestParseCommentHints_overridesOnly(t *testing.T) {
	h := ParseCommentHints("pg-flux: gotype=foo.Bar tstype=Foo")
	if h.Doc != "" {
		t.Errorf("doc should be empty, got %q", h.Doc)
	}
	want := map[string]string{"gotype": "foo.Bar", "tstype": "Foo"}
	if !reflect.DeepEqual(h.Overrides, want) {
		t.Errorf("overrides: %v", h.Overrides)
	}
}

func TestParseCommentHints_docPlusOverrides(t *testing.T) {
	h := ParseCommentHints("user role pg-flux: gotype=Role")
	if h.Doc != "user role" {
		t.Errorf("doc: %q", h.Doc)
	}
	if h.Overrides["gotype"] != "Role" {
		t.Errorf("override: %v", h.Overrides)
	}
}

func TestParseCommentHints_quotedValue(t *testing.T) {
	// Quoted value may contain spaces.
	h := ParseCommentHints(`pg-flux: gotype="map[string]string"`)
	if got := h.Overrides["gotype"]; got != "map[string]string" {
		t.Errorf("quoted override: %q", got)
	}
}

func TestTokenizeCommentHints_emptyAndQuotes(t *testing.T) {
	got := tokenizeCommentHints(`a=1 b='x y' c=3`)
	want := []string{"a=1", "b=x y", "c=3"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("tokens: %v", got)
	}
}

func TestTokenizeCommentHints_balancedBraces(t *testing.T) {
	// Complex TS object type with spaces inside braces must stay atomic.
	got := tokenizeCommentHints(`tstype={ source: string; ip?: string } gotype=int`)
	want := []string{"tstype={ source: string; ip?: string }", "gotype=int"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("balanced-brace tokenization: %v", got)
	}
}

func TestTokenizeCommentHints_balancedBrackets(t *testing.T) {
	got := tokenizeCommentHints(`gotype=map[string]int other=1`)
	want := []string{"gotype=map[string]int", "other=1"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("bracketed type tokenization: %v", got)
	}
}

func TestTokenizeCommentHints_nestedGroups(t *testing.T) {
	got := tokenizeCommentHints(`tstype=Array<{ k: string; v: number }>`)
	want := []string{"tstype=Array<{ k: string; v: number }>"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("nested grouping: %v", got)
	}
}

func TestParseCommentHints_complexTSType(t *testing.T) {
	h := ParseCommentHints(`pg-flux: tstype={ source: string; ip?: string }`)
	got, ok := h.Overrides["tstype"]
	if !ok || got != "{ source: string; ip?: string }" {
		t.Errorf("expected full TS object expression, got %q", got)
	}
}
