package obs

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestInit_textFormat(t *testing.T) {
	var buf bytes.Buffer
	Init(FormatText, false, &buf)
	Info("migration.applied", "file", "01_baseline.sql", "duration_ms", 142)
	out := buf.String()
	if !strings.Contains(out, "migration.applied") || !strings.Contains(out, "01_baseline.sql") {
		t.Fatalf("text log missing keys/values: %s", out)
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
