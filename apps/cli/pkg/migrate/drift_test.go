package migrate

import (
	"strings"
	"testing"
)

func TestExtractBaselineHash_present(t *testing.T) {
	content := []byte(`-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.
-- pg-flux-baseline-hash: abc123def456789

BEGIN;
CREATE TABLE x (id int);
COMMIT;
`)
	got := extractBaselineHash(content)
	if got != "abc123def456789" {
		t.Fatalf("expected hash, got %q", got)
	}
}

func TestExtractBaselineHash_absent(t *testing.T) {
	content := []byte(`-- pg-flux generated migration
-- DO NOT EDIT unless you know what you are doing.

BEGIN;
CREATE TABLE x (id int);
COMMIT;
`)
	got := extractBaselineHash(content)
	if got != "" {
		t.Fatalf("expected empty (older file), got %q", got)
	}
}

func TestExtractBaselineHash_stopsAtFirstDDL(t *testing.T) {
	// Defensive: a baseline-hash-like line appearing AFTER real SQL should not match.
	content := []byte(`-- pg-flux generated migration

BEGIN;
-- pg-flux-baseline-hash: SHOULD_NOT_MATCH
CREATE TABLE x (id int);
COMMIT;
`)
	got := extractBaselineHash(content)
	if got != "" {
		t.Fatalf("expected empty (header must be in preamble), got %q", got)
	}
}

func TestExtractBaselineHash_handlesCRLF(t *testing.T) {
	content := []byte("-- pg-flux generated migration\r\n-- pg-flux-baseline-hash: deadbeef\r\n\r\nBEGIN;\r\nCOMMIT;\r\n")
	got := extractBaselineHash(content)
	if got != "deadbeef" {
		t.Fatalf("CRLF handling: got %q", got)
	}
}

func TestBaselineDriftError_message(t *testing.T) {
	e := &BaselineDriftError{
		Filename:     "20260101_test.sql",
		ExpectedHash: "abcdef1234567890",
		LiveHash:     "fedcba0987654321",
		HasBaseline:  true,
	}
	msg := e.Error()
	for _, want := range []string{"drifted", "--force-after-drift", "abcdef123456", "fedcba098765", "20260101_test.sql"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("error message missing %q: %s", want, msg)
		}
	}
}

func TestShortHash_truncates(t *testing.T) {
	if got := shortHash("abcdef1234567890abcdef"); got != "abcdef123456…" {
		t.Fatalf("short hash truncation: %s", got)
	}
	if got := shortHash("abc"); got != "abc" {
		t.Fatalf("short hash passthrough: %s", got)
	}
}
