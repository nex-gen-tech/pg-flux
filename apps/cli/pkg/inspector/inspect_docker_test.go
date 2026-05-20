//go:build integration

package inspector

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestInspect_SmokeWithDocker(t *testing.T) {
	if os.Getenv("PGFLUX_E2E") == "" {
		t.Skip("set PGFLUX_E2E=1 to run testcontainers")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	c, err := postgres.Run(ctx, "postgres:18-alpine",
		postgres.WithDatabase("appdb"),
		postgres.WithUsername("app"),
		postgres.WithPassword("app"),
	)
	if err != nil {
		t.Skipf("docker not available: %v", err)
	}
	t.Cleanup(func() { _ = c.Terminate(context.Background()) })
	conn, err := c.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}
	po, err := pgxpool.New(ctx, conn)
	require.NoError(t, err)
	t.Cleanup(func() { po.Close() })
	_, err = po.Exec(ctx, `CREATE TABLE public.dock_t (id int primary key);`)
	require.NoError(t, err)
	st, err := Inspect(ctx, po, Options{Schemas: []string{"public"}})
	require.NoError(t, err)
	require.NotEmpty(t, st.Tables)
	_, err = Reltuples(ctx, po, "public", "dock_t")
	require.NoError(t, err)
}
