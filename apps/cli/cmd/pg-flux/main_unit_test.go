package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/nex-gen-tech/pg-flux/pkg/plan"
	"github.com/nex-gen-tech/pg-flux/pkg/schema"
	"github.com/stretchr/testify/require"
)

func TestParseSchemas(t *testing.T) {
	schemasFlag = " app , public  "
	t.Cleanup(func() { schemasFlag = "public" })
	s := parseSchemas()
	require.Equal(t, []string{"app", "public"}, s)
	schemasFlag = ""
	s2 := parseSchemas()
	require.Equal(t, []string{"public"}, s2)
}

func TestDiffSummary(t *testing.T) {
	require.Nil(t, diffSummary(nil))
	p := &plan.ExecutionPlan{Statements: []plan.Statement{
		{DDL: "CREATE TABLE x (id int)", OpType: "table", Object: "x"},
	}}
	ds := diffSummary(p)
	require.Len(t, ds, 1)
	require.Equal(t, "table", ds[0].ObjectType)
	require.Equal(t, "x", ds[0].ObjectName)
}

func TestDifferOptions_defaultsAndReltupleSkip(t *testing.T) {
	xR := reltupleThresh
	xA := appendValidateF
	t.Cleanup(func() {
		reltupleThresh = xR
		appendValidateF = xA
	})
	reltupleThresh = 1_000_000
	appendValidateF = true
	opt, err := differOptions(context.Background(), nil, &schema.SchemaState{Tables: map[string]*schema.Table{}})
	require.NoError(t, err)
	require.True(t, opt.AppendValidateAfterNotValid)
	require.InDelta(t, 1_000_000, opt.SetNotNullReltupleThreshold, 1)
	require.Nil(t, opt.Reltuples)
}

func TestRunDiff_noDatabaseURL(t *testing.T) {
	p := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(p, "t.sql"), []byte("CREATE TABLE x (id int);"), 0o644))
	xS, xU, xE := schemaPath, dbURL, os.Getenv("DATABASE_URL")
	t.Cleanup(func() {
		schemaPath = xS
		dbURL = xU
		_ = os.Setenv("DATABASE_URL", xE)
	})
	schemaPath = p
	schemaFile = ""
	dbURL = ""
	_ = os.Unsetenv("DATABASE_URL")
	_, err := runDiff()
	require.Error(t, err)
}

func TestLoadDesired_TempDir(t *testing.T) {
	p := t.TempDir()
	schemaPath = p
	schemaFile = ""
	validatePlpgsqlF = false
	validateSQLF = false
	t.Cleanup(func() {
		schemaPath = "./schema"
		schemaFile = ""
	})
	require.NoError(t, os.WriteFile(filepath.Join(p, "a.sql"), []byte("CREATE TABLE t_ld (id int);"), 0o644))
	st, err := loadDesired()
	require.NoError(t, err)
	require.NotEmpty(t, st.Tables)
}

func TestLoadConfig_missing(t *testing.T) {
	cfg, err := loadConfig(filepath.Join(t.TempDir(), "nonexistent.yml"))
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, "", cfg.SchemaDir)
}

func TestLoadConfig_valid(t *testing.T) {
	f := filepath.Join(t.TempDir(), "cfg.yml")
	require.NoError(t, os.WriteFile(f, []byte("version: 1\nschema_dir: ./migrations\ntarget_schemas:\n  - public\n  - app\n"), 0o644))
	cfg, err := loadConfig(f)
	require.NoError(t, err)
	require.Equal(t, "./migrations", cfg.SchemaDir)
	require.Equal(t, []string{"public", "app"}, cfg.TargetSchemas)
}

func TestLoadConfig_invalid(t *testing.T) {
	f := filepath.Join(t.TempDir(), "bad.yml")
	require.NoError(t, os.WriteFile(f, []byte("version: [invalid yaml\n"), 0o644))
	_, err := loadConfig(f)
	require.Error(t, err)
}

func TestErrDriftDetected_sentinel(t *testing.T) {
	require.ErrorIs(t, errDriftDetected, errDriftDetected)
	require.NotErrorIs(t, errDriftDetected, os.ErrNotExist)
}
