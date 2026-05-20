//go:build integration

package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/nexg/pg-flux/pkg/inspector"
)

// TestInspector_RLSAndPolicies uses Docker to exercise catalog paths for RLS and policies
// (best verified with a real server; complements unit tests in pkg/inspector).
func TestInspector_RLSAndPolicies(t *testing.T) {
	if os.Getenv("PGFLUX_E2E") == "" {
		t.Skip("set PGFLUX_E2E=1 and ensure Docker is running")
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
	po, err := pgxpool.New(ctx, conn)
	require.NoError(t, err)
	t.Cleanup(func() { po.Close() })
	require.NoError(t, waitForPostgres(ctx, po))

	_, err = po.Exec(ctx, `
CREATE TABLE public.rls_t (id int primary key, v text);
ALTER TABLE public.rls_t ENABLE ROW LEVEL SECURITY;
CREATE POLICY p_read ON public.rls_t FOR SELECT TO public USING (true);
CREATE POLICY p_write ON public.rls_t FOR INSERT TO public WITH CHECK (true);
`)
	require.NoError(t, err)
	st, err := inspector.Inspect(ctx, po, inspector.Options{Schemas: []string{"public"}})
	require.NoError(t, err)
	tb := st.Tables["public.rls_t"]
	require.NotNil(t, tb)
	require.True(t, tb.RLSEnabled)
	require.GreaterOrEqual(t, len(st.Policies), 1)
}
