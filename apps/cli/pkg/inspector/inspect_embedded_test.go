package inspector

import (
	"context"
	"net"
	"strconv"
	"testing"
	"time"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestInspect_embeddedPostgres(t *testing.T) {
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
		t.Skipf("embedded postgres not available: %v", err)
	}
	t.Cleanup(func() { _ = ep.Stop() })

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	dsn := "postgres://euser:esecret@127.0.0.1:" + strconv.Itoa(port) + "/eapp?sslmode=disable"
	po, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(func() { po.Close() })

	_, err = po.Exec(ctx, `CREATE TABLE public.embed_t (id int primary key, note text not null);
CREATE INDEX embed_t_note ON public.embed_t (note);
CREATE OR REPLACE FUNCTION public.embed_fn() RETURNS int LANGUAGE sql IMMUTABLE AS 'SELECT 1';`)
	require.NoError(t, err)
	st, err := Inspect(ctx, po, Options{Schemas: []string{"public"}})
	require.NoError(t, err)
	require.NotEmpty(t, st.Tables)
	_, err = Reltuples(ctx, po, "public", "embed_t")
	require.NoError(t, err)
	if st.Indexes != nil {
		_ = len(st.Indexes)
	}
}
