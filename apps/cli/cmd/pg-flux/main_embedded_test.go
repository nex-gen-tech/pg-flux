package main

import (
	"bytes"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/stretchr/testify/require"
)

func TestRunDiff_embedded(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("free port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	root := t.TempDir()
	cfg := embeddedpostgres.DefaultConfig().
		DataPath(root + "/data").
		RuntimePath(root + "/rt").
		Port(uint32(port)).Database("app").Username("u").Password("p")
	ep := embeddedpostgres.NewDatabase(cfg)
	if e := ep.Start(); e != nil {
		t.Skipf("embedded postgres: %v", e)
	}
	t.Cleanup(func() { _ = ep.Stop() })
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.sql"), []byte("CREATE TABLE public.rd (id int primary key);"), 0o644))
	xS, xF, xU, xA := schemaPath, schemaFile, dbURL, appendValidateF
	t.Cleanup(func() {
		schemaPath, schemaFile, dbURL, appendValidateF = xS, xF, xU, xA
		_ = os.Unsetenv("DATABASE_URL")
	})
	schemaPath, schemaFile, dbURL, appendValidateF = dir, "", "", false
	dsn := "postgres://u:p@127.0.0.1:" + strconv.Itoa(port) + "/app?sslmode=disable"
	_ = os.Setenv("DATABASE_URL", dsn)
	// Allow loadDesired to parse and runDiff to connect, inspect, and diff.
	require.NoError(t, os.Setenv("PGCONNECT_TIMEOUT", "5"))
	t.Cleanup(func() { _ = os.Unsetenv("PGCONNECT_TIMEOUT") })
	_, err = runDiff()
	require.NoError(t, err)
}

func TestCmdPlan_embedded(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("free port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	root := t.TempDir()
	cfg := embeddedpostgres.DefaultConfig().
		DataPath(root + "/data").
		RuntimePath(root + "/rt").
		Port(uint32(port)).Database("app2").Username("u2").Password("p2")
	ep := embeddedpostgres.NewDatabase(cfg)
	if e := ep.Start(); e != nil {
		t.Skipf("embedded postgres: %v", e)
	}
	t.Cleanup(func() { _ = ep.Stop() })
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.sql"), []byte("CREATE TABLE public.plan_t (id int);"), 0o644))
	dsn := "postgres://u2:p2@127.0.0.1:" + strconv.Itoa(port) + "/app2?sslmode=disable"
	gf := globalFormat
	t.Cleanup(func() { globalFormat = gf })
	globalFormat = "human"
	var buf bytes.Buffer
	r := newRoot()
	r.SetOut(&buf)
	r.SetArgs([]string{"plan", "--db", dsn, "--schema", dir})
	require.NoError(t, r.Execute())
	require.Contains(t, buf.String(), "source_hash=")
}

func TestCmdPlan_json_embedded(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("free port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	runtimeRoot := t.TempDir()
	cfg := embeddedpostgres.DefaultConfig().
		DataPath(runtimeRoot + "/data").
		RuntimePath(runtimeRoot + "/rt").
		Port(uint32(port)).Database("app3").Username("u3").Password("p3")
	ep := embeddedpostgres.NewDatabase(cfg)
	if e := ep.Start(); e != nil {
		t.Skipf("embedded postgres: %v", e)
	}
	t.Cleanup(func() { _ = ep.Stop() })
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.sql"), []byte("CREATE TABLE public.jt (id int);"), 0o644))
	dsn := "postgres://u3:p3@127.0.0.1:" + strconv.Itoa(port) + "/app3?sslmode=disable"
	var buf bytes.Buffer
	r := newRoot()
	r.SetOut(&buf)
	r.SetArgs([]string{"plan", "--format", "json", "--db", dsn, "--schema", dir})
	require.NoError(t, r.Execute())
	require.Contains(t, buf.String(), `"version"`)
}

func startEphemeral(t *testing.T) (dsn, dir string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("free port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	runtimeRoot := t.TempDir()
	uniq := runtimeRoot + "/pg"
	cfg := embeddedpostgres.DefaultConfig().
		DataPath(uniq + "/data").
		RuntimePath(uniq + "/rt").
		Port(uint32(port)).Database("d").Username("u").Password("p")
	ep := embeddedpostgres.NewDatabase(cfg)
	if e := ep.Start(); e != nil {
		t.Skipf("embedded postgres: %v", e)
	}
	t.Cleanup(func() { _ = ep.Stop() })
	dir = t.TempDir()
	dsn = "postgres://u:p@127.0.0.1:" + strconv.Itoa(port) + "/d?sslmode=disable"
	return dsn, dir
}

func TestCmdDrift_embedded_noDrift(t *testing.T) {
	dsn, dir := startEphemeral(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.sql"), []byte("CREATE TABLE public.m (id int);"), 0o644))
	{
		var buf bytes.Buffer
		r := newRoot()
		r.SetOut(&buf)
		r.SetArgs([]string{"apply", "--db", dsn, "--schema", dir})
		require.NoError(t, r.Execute())
	}
	var buf2 bytes.Buffer
	r := newRoot()
	r.SetOut(&buf2)
	r.SetArgs([]string{"drift", "--db", dsn, "--schema", dir})
	require.NoError(t, r.Execute())
	require.Contains(t, buf2.String(), "No drift")
}

func TestCmdInspect_embedded(t *testing.T) {
	dsn, dir := startEphemeral(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.sql"), []byte("CREATE TABLE public.ins (id int);"), 0o644))
	{
		r := newRoot()
		r.SetArgs([]string{"apply", "--db", dsn, "--schema", dir})
		require.NoError(t, r.Execute())
	}

	t.Run("stdout SQL", func(t *testing.T) {
		var buf bytes.Buffer
		r := newRoot()
		r.SetOut(&buf)
		r.SetArgs([]string{"inspect", "--db", dsn, "--schemas", "public"})
		require.NoError(t, r.Execute())
		require.Contains(t, buf.String(), "CREATE TABLE")
		require.Contains(t, buf.String(), "ins")
	})

	t.Run("summary", func(t *testing.T) {
		var buf bytes.Buffer
		r := newRoot()
		r.SetOut(&buf)
		r.SetArgs([]string{"inspect", "--db", dsn, "--schemas", "public", "--summary"})
		require.NoError(t, r.Execute())
		require.Contains(t, buf.String(), "table")
		require.Contains(t, buf.String(), "ins")
	})

	t.Run("type filter", func(t *testing.T) {
		var buf bytes.Buffer
		r := newRoot()
		r.SetOut(&buf)
		r.SetArgs([]string{"inspect", "--db", dsn, "--schemas", "public", "--type", "table"})
		require.NoError(t, r.Execute())
		require.Contains(t, buf.String(), "CREATE TABLE")
	})

	t.Run("out file", func(t *testing.T) {
		outFile := filepath.Join(t.TempDir(), "schema.sql")
		var buf bytes.Buffer
		r := newRoot()
		r.SetOut(&buf)
		r.SetArgs([]string{"inspect", "--db", dsn, "--schemas", "public", "--out", outFile})
		require.NoError(t, r.Execute())
		require.Contains(t, buf.String(), "Wrote")
		data, err := os.ReadFile(outFile)
		require.NoError(t, err)
		require.Contains(t, string(data), "CREATE TABLE")
	})
}

func TestCmdApply_dryRun_embedded(t *testing.T) {
	dsn, dir := startEphemeral(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.sql"), []byte("CREATE TABLE public.ap (id int);"), 0o644))
	var buf bytes.Buffer
	r := newRoot()
	r.SetOut(&buf)
	r.SetArgs([]string{"apply", "--dry-run", "true", "--db", dsn, "--schema", dir})
	require.NoError(t, r.Execute())
	_ = buf
}

func TestCmdPlan_withShadowDsn_embedded(t *testing.T) {
	dsn, dir := startEphemeral(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.sql"), []byte("CREATE TABLE public.sd (id int);"), 0o644))
	var buf bytes.Buffer
	r := newRoot()
	r.SetOut(&buf)
	r.SetArgs([]string{"plan", "--db", dsn, "--schema", dir, "--shadow-dsn", dsn})
	require.NoError(t, r.Execute())
	require.Contains(t, buf.String(), "source_hash=")
}
