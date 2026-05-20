//go:build integration

package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/nexg/pg-flux/pkg/db"
	"github.com/nexg/pg-flux/pkg/inspector"
)

func TestReltuples(t *testing.T) {
	if os.Getenv("PGFLUX_E2E") == "" {
		t.Skip("set PGFLUX_E2E=1 and Docker")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	c, err := postgres.Run(ctx, "postgres:18-alpine",
		postgres.WithDatabase("appdb"),
		postgres.WithUsername("app"),
		postgres.WithPassword("app"),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Terminate(context.Background()) })
	conn, err := c.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	po, err := db.NewPool(ctx, conn)
	require.NoError(t, err)
	t.Cleanup(func() { po.Close() })
	_, err = po.Exec(ctx, `CREATE TABLE public.rtest (id int);`)
	require.NoError(t, err)
	n, err := inspector.Reltuples(ctx, po, "public", "rtest")
	require.NoError(t, err)
	_ = n
}
