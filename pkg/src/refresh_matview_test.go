package src

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestParser_RefreshMatViewPassthrough confirms that REFRESH MATERIALIZED VIEW
// statements in schema files are captured into ExtraDDL so they flow through to
// the next generated migration as pass-through DDL.
func TestParser_RefreshMatViewPassthrough(t *testing.T) {
	dir := t.TempDir()
	contents := `
CREATE MATERIALIZED VIEW public.metrics_daily AS
  SELECT 1 AS x;
REFRESH MATERIALIZED VIEW public.metrics_daily;
`
	if err := os.WriteFile(filepath.Join(dir, "schema.sql"), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := LoadDesiredState(LoadOptions{SchemaDir: dir})
	if err != nil {
		t.Fatalf("LoadDesiredState: %v", err)
	}
	found := false
	for _, ddl := range st.ExtraDDL {
		if strings.Contains(strings.ToUpper(ddl), "REFRESH MATERIALIZED VIEW") {
			found = true
		}
	}
	if !found {
		t.Fatalf("REFRESH MATERIALIZED VIEW not captured in ExtraDDL: %v", st.ExtraDDL)
	}
}

func TestParser_RefreshConcurrentlyPassthrough(t *testing.T) {
	dir := t.TempDir()
	contents := `
CREATE MATERIALIZED VIEW public.m1 AS SELECT 1 AS x;
CREATE UNIQUE INDEX m1_pk ON public.m1(x);
REFRESH MATERIALIZED VIEW CONCURRENTLY public.m1;
`
	if err := os.WriteFile(filepath.Join(dir, "schema.sql"), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := LoadDesiredState(LoadOptions{SchemaDir: dir})
	if err != nil {
		t.Fatalf("LoadDesiredState: %v", err)
	}
	gotConcurrent := false
	for _, ddl := range st.ExtraDDL {
		up := strings.ToUpper(ddl)
		if strings.Contains(up, "REFRESH MATERIALIZED VIEW") && strings.Contains(up, "CONCURRENTLY") {
			gotConcurrent = true
		}
	}
	if !gotConcurrent {
		t.Fatalf("CONCURRENTLY not preserved in pass-through: %v", st.ExtraDDL)
	}
}
