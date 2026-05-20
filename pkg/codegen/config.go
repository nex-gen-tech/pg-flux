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
	Lang          Language          `yaml:"lang"`
	Out           string            `yaml:"out"`
	Package       string            `yaml:"package,omitempty"`
	TypeOverrides map[string]string `yaml:"type_overrides,omitempty"`
	// NameOverrides forces a specific generated identifier for a PG identifier.
	// Keys are "schema.name" or just "name". Values are the literal language
	// identifier to emit (no PascalCase transformation applied).
	NameOverrides map[string]string `yaml:"name_overrides,omitempty"`
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
