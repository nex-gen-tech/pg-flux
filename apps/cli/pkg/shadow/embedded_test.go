package shadow

import (
	"context"
	"net"
	"strconv"
	"testing"
	"time"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/nexg/pg-flux/pkg/plan"
	"github.com/stretchr/testify/require"
)

func TestValidateSyntaxOnDatabase_embedded(t *testing.T) {
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
		Port(uint32(port)).Database("sapp").Username("suser").Password("ssec")
	ep := embeddedpostgres.NewDatabase(cfg)
	if e := ep.Start(); e != nil {
		t.Skipf("embedded postgres: %v", e)
	}
	t.Cleanup(func() { _ = ep.Stop() })
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	dsn := "postgres://suser:ssec@127.0.0.1:" + strconv.Itoa(port) + "/sapp?sslmode=disable"
	plan1 := &plan.ExecutionPlan{Statements: []plan.Statement{
		{ID: 1, DDL: "CREATE TABLE sh_x (i int);"},
	}}
	err = ValidateSyntaxOnDatabase(ctx, dsn, plan1)
	require.NoError(t, err)
}

func TestValidateSemanticOnDatabase_embedded(t *testing.T) {
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
		Port(uint32(port)).Database("sapp2").Username("suser2").Password("ssec2")
	ep := embeddedpostgres.NewDatabase(cfg)
	if e := ep.Start(); e != nil {
		t.Skipf("embedded postgres: %v", e)
	}
	t.Cleanup(func() { _ = ep.Stop() })
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	dsn := "postgres://suser2:ssec2@127.0.0.1:" + strconv.Itoa(port) + "/sapp2?sslmode=disable"
	plan1 := &plan.ExecutionPlan{Statements: []plan.Statement{
		{ID: 1, DDL: "CREATE TABLE sh_sem (i int primary key);"},
	}}
	err = ValidateSemanticOnDatabase(ctx, dsn, plan1)
	require.NoError(t, err)
}
