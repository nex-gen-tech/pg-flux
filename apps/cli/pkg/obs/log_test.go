package obs

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestInit_textFormatSuppressesInfo(t *testing.T) {
	// Text mode without --verbose is the CLI default; INFO must NOT print
	// (otherwise the human progress lines and structured logs both emit).
	var buf bytes.Buffer
	Init(FormatText, false, &buf)
	Info("migration.applied", "file", "01_baseline.sql", "duration_ms", 142)
	if strings.Contains(buf.String(), "migration.applied") {
		t.Fatalf("text mode (non-verbose) must suppress INFO, got: %q", buf.String())
	}
	// WARN must still surface.
	Warn("something.fishy", "kind", "bad")
	if !strings.Contains(buf.String(), "something.fishy") {
		t.Fatalf("text mode must keep WARN, got: %q", buf.String())
	}
}

func TestInit_textFormatVerboseEmitsInfo(t *testing.T) {
	// With --verbose, text mode emits INFO (operator opted in to chatter).
	var buf bytes.Buffer
	Init(FormatText, true, &buf)
	Info("migration.applied", "file", "01_baseline.sql")
	if !strings.Contains(buf.String(), "migration.applied") {
		t.Fatalf("text+verbose should emit INFO: %q", buf.String())
	}
}

func TestCurrentFormat(t *testing.T) {
	Init(FormatJSON, false, &bytes.Buffer{})
	if CurrentFormat() != FormatJSON {
		t.Fatalf("CurrentFormat=%q after Init(FormatJSON), want json", CurrentFormat())
	}
	Init(FormatText, false, &bytes.Buffer{})
	if CurrentFormat() != FormatText {
		t.Fatalf("CurrentFormat=%q after Init(FormatText), want text", CurrentFormat())
	}
}

func TestInit_jsonFormat(t *testing.T) {
	var buf bytes.Buffer
	Init(FormatJSON, false, &buf)
	Info("migration.applied", "file", "01_baseline.sql", "duration_ms", 142)
	var got map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &got); err != nil {
		t.Fatalf("expected valid JSON; got %q (%v)", buf.String(), err)
	}
	if got["msg"] != "migration.applied" {
		t.Fatalf("msg field wrong: %v", got)
	}
	if got["file"] != "01_baseline.sql" {
		t.Fatalf("file field wrong: %v", got)
	}
}

func TestInit_verboseEnablesDebug(t *testing.T) {
	var buf bytes.Buffer
	Init(FormatText, true, &buf)
	Debug("statement.executed", "id", 1)
	if !strings.Contains(buf.String(), "statement.executed") {
		t.Fatalf("debug level should emit when verbose=true: %s", buf.String())
	}
}

func TestInit_nonVerboseSuppressesDebug(t *testing.T) {
	var buf bytes.Buffer
	Init(FormatText, false, &buf)
	Debug("not.emitted")
	if strings.Contains(buf.String(), "not.emitted") {
		t.Fatalf("debug suppressed when verbose=false, but emitted: %s", buf.String())
	}
}
