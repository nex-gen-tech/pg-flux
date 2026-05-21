//go:build integration

package exec

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/nex-gen-tech/pg-flux/pkg/plan"
)

func TestApply_SmokeDocker(t *testing.T) {
	if os.Getenv("PGFLUX_E2E") == "" {
		t.Skip("set PGFLUX_E2E=1 to run testcontainers")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	c, err := postgres.Run(ctx, "postgres:18-alpine",
		postgres.WithDatabase("a"),
		postgres.WithUsername("a"),
		postgres.WithPassword("a"),
	)
	if err != nil {
		t.Skipf("docker: %v", err)
	}
	t.Cleanup(func() { _ = c.Terminate(context.Background()) })
	conn, err := c.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	po, err := pgxpool.New(ctx, conn)
	require.NoError(t, err)
	t.Cleanup(func() { po.Close() })
	p := &plan.ExecutionPlan{Statements: []plan.Statement{
		{ID: 1, DDL: "CREATE TABLE t_exec (id serial PRIMARY KEY)"},
	}}
	t.Setenv("DATABASE_URL", conn)
	err = Apply(ctx, po, p, Options{DryRun: false})
	require.NoError(t, err)
}
