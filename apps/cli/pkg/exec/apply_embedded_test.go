package exec

import (
	"context"
	"net"
	"os"
	"strconv"
	"testing"
	"time"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/nexg/pg-flux/pkg/plan"
)

func TestApply_embeddedPostgres(t *testing.T) {
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
		Port(uint32(port)).Database("eapp").Username("euser").Password("esecret")
	ep := embeddedpostgres.NewDatabase(cfg)
	if err := ep.Start(); err != nil {
		t.Skipf("embedded postgres: %v", err)
	}
	t.Cleanup(func() { _ = ep.Stop() })
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	dsn := "postgres://euser:esecret@127.0.0.1:" + strconv.Itoa(port) + "/eapp?sslmode=disable"
	po, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(func() { po.Close() })
	t.Setenv("DATABASE_URL", dsn)
	t.Cleanup(func() { _ = os.Unsetenv("DATABASE_URL") })
	p := &plan.ExecutionPlan{Statements: []plan.Statement{
		{ID: 1, DDL: "CREATE TABLE apply_embed (x serial PRIMARY KEY)"},
	}}
	err = Apply(ctx, po, p, Options{DryRun: false})
	require.NoError(t, err)
}
