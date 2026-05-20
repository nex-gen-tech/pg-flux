package codegen

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config is the user-facing .pg-flux-codegen.yml format.
//
// Example:
//
//	outputs:
//	  - lang: go
//	    out: ./internal/dbgen
//	    package: dbgen
//	    type_overrides:
//	      numeric: "github.com/shopspring/decimal.Decimal"
//	      jsonb:   "MyJSON"
//	  - lang: ts
//	    out: ./apps/web/src/generated
//	    type_overrides:
//	      numeric: string
//	      bytea:   string  # base64
type Config struct {
	// Outputs declares one or more language/destination targets.
	Outputs []OutputConfig `yaml:"outputs"`
}

// OutputConfig is one (language, destination, options) triplet from Config.Outputs.
type OutputConfig struct {
	Lang    Language `yaml:"lang"`
	Out     string   `yaml:"out"`
	Package string   `yaml:"package,omitempty"`
	// TypeOverrides maps PG types → language types. Go values may be fully-
	// qualified ("github.com/foo/bar.Baz"); TS values are emitted verbatim.
	TypeOverrides map[string]string `yaml:"type_overrides,omitempty"`
	// NameOverrides forces a specific generated identifier for a PG identifier.
	// Keys are "schema.name" or just "name". Values are the literal language
	// identifier to emit (no PascalCase transformation applied).
	NameOverrides map[string]string `yaml:"name_overrides,omitempty"`

	// --- Emit options ---

	Layout              string            `yaml:"layout,omitempty"`               // per-kind | per-object | single
	ColumnCase          string            `yaml:"column_case,omitempty"`          // snake | camel | pascal
	Readonly            string            `yaml:"readonly,omitempty"`             // identity | generated | defaults | all | none
	InsertUpdateHelpers bool              `yaml:"insert_update_helpers,omitempty"`
	BrandedIDs          bool              `yaml:"branded_ids,omitempty"`
	BigintAs            string            `yaml:"bigint_as,omitempty"`            // bigint | number | string  (TS)
	DateAs              string            `yaml:"date_as,omitempty"`              // Date | string | temporal (TS)
	NullStyle           string            `yaml:"null_style,omitempty"`           // union | undefined | optional (TS)
	EnumStyle           string            `yaml:"enum_style,omitempty"`           // union | const-object | ts-enum (TS)
	Validators          string            `yaml:"validators,omitempty"`           // zod | "" (TS)
	ORMTags             string            `yaml:"orm_tags,omitempty"`             // gorm | sqlx | bun | ent | "" (Go)
	OmitEmpty           string            `yaml:"omitempty,omitempty"`            // nullable | defaults | all | "" (Go)
	JSONShapes          map[string]string `yaml:"json_shapes,omitempty"`          // "schema.table.column" → TS type
	Functions           bool              `yaml:"functions,omitempty"`            // emit function/procedure param + result types

	// --- Filtering ---

	IncludeTables  []string `yaml:"include_tables,omitempty"`
	ExcludeTables  []string `yaml:"exclude_tables,omitempty"`
	ExcludeSchemas []string `yaml:"exclude_schemas,omitempty"`
}

// ToEmitOptions translates the YAML-friendly OutputConfig into the strongly-
// typed EmitOptions consumed by the generators.
func (o OutputConfig) ToEmitOptions() EmitOptions {
	eo := EmitOptions{
		Layout:              Layout(o.Layout),
		ColumnCase:          ColumnCase(o.ColumnCase),
		Readonly:            ReadonlyPolicy(o.Readonly),
		InsertUpdateHelpers: o.InsertUpdateHelpers,
		BrandedIDs:          o.BrandedIDs,
		BigintAs:            o.BigintAs,
		DateAs:              o.DateAs,
		NullStyle:           o.NullStyle,
		EnumStyle:           o.EnumStyle,
		Validators:          o.Validators,
		ORMTags:             o.ORMTags,
		OmitEmpty:           o.OmitEmpty,
		JSONShapes:          o.JSONShapes,
		Functions:           o.Functions,
		Filter: Filter{
			IncludeTables:  o.IncludeTables,
			ExcludeTables:  o.ExcludeTables,
			ExcludeSchemas: o.ExcludeSchemas,
		},
	}
	(&eo).normalize()
	return eo
}

// LoadConfig reads .pg-flux-codegen.yml from path. Missing file returns
// (nil, nil) so callers can fall back to CLI-flag-only operation.
func LoadConfig(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var c Config
	if err := yaml.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	for i, o := range c.Outputs {
		if o.Lang == "" {
			return nil, fmt.Errorf("%s: outputs[%d].lang is required", path, i)
		}
		if o.Out == "" {
			return nil, fmt.Errorf("%s: outputs[%d].out is required", path, i)
		}
	}
	return &c, nil
}

// CommentHints parses inline pg-flux directives from a column or table COMMENT.
// Syntax: "pg-flux: gotype=foo.Bar tstype=Foo nullable=force".
// Multiple key=value pairs are separated by whitespace. Anything before the
// "pg-flux:" prefix is treated as the human-readable doc comment.
type CommentHints struct {
	Doc       string            // human-readable text before "pg-flux:"
	Overrides map[string]string // key → value, e.g. {"gotype": "foo.Bar"}
}

// ParseCommentHints splits a PG COMMENT into a documentation prefix and a
// directive map. The "pg-flux:" prefix is the boundary; tokens after it are
// parsed as key=value pairs. Tokens without "=" are skipped silently.
func ParseCommentHints(comment string) CommentHints {
	h := CommentHints{Doc: comment}
	idx := strings.Index(comment, "pg-flux:")
	if idx < 0 {
		return h
	}
	h.Doc = strings.TrimSpace(comment[:idx])
	rest := strings.TrimSpace(comment[idx+len("pg-flux:"):])
	if rest == "" {
		return h
	}
	h.Overrides = map[string]string{}
	for _, tok := range tokenizeCommentHints(rest) {
		eq := strings.IndexByte(tok, '=')
		if eq <= 0 {
			continue
		}
		k := strings.ToLower(strings.TrimSpace(tok[:eq]))
		v := strings.TrimSpace(tok[eq+1:])
		if k != "" && v != "" {
			h.Overrides[k] = v
		}
	}
	return h
}

// tokenizeCommentHints splits a hint string on whitespace, treating "..." or
// '...' quoted runs as single tokens so a quoted Go type with parens etc.
// stays atomic.
func tokenizeCommentHints(s string) []string {
	var out []string
	var cur strings.Builder
	inQuote := byte(0)
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case inQuote != 0:
			if c == inQuote {
				inQuote = 0
				continue
			}
			cur.WriteByte(c)
		case c == '"' || c == '\'':
			inQuote = c
		case c == ' ' || c == '\t' || c == '\n':
			if cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteByte(c)
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}
