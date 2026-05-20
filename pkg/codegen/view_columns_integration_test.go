//go:build integration

package codegen

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/nexg/pg-flux/pkg/inspector"
)

// TestViewColumns_realInference proves the inspector → codegen path emits real
// columns for both regular views and materialized views, including aggregate /
// JOIN / COALESCE columns whose types PG resolves at view-creation time.
func TestViewColumns_realInference(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	admin, err := pgxpool.New(ctx, adminDSN())
	require.NoError(t, err)
	defer admin.Close()
	const dbname = "pgflux_viewcols_test"
	_, _ = admin.Exec(ctx, "DROP DATABASE IF EXISTS "+dbname)
	_, err = admin.Exec(ctx, "CREATE DATABASE "+dbname)
	require.NoError(t, err)
	defer func() { _, _ = admin.Exec(context.Background(), "DROP DATABASE IF EXISTS "+dbname) }()
	cfg, _ := pgxpool.ParseConfig(adminDSN())
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.ConnConfig.User, cfg.ConnConfig.Password,
		cfg.ConnConfig.Host, cfg.ConnConfig.Port, dbname)
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	defer pool.Close()

	// Exercise: aggregate (count → bigint), COALESCE (heterogeneous → text),
	// JOIN-derived nullable column, a matview with the same shape.
	_, err = pool.Exec(ctx, `
		CREATE TABLE users (id bigint PRIMARY KEY, email text NOT NULL, display_name text);
		CREATE TABLE posts (id bigint PRIMARY KEY, user_id bigint NOT NULL);
		CREATE VIEW user_stats AS
		  SELECT u.id,
		         count(p.id)                                 AS post_count,
		         COALESCE(u.display_name, u.email)           AS display,
		         u.display_name                              AS optional_name
		  FROM users u
		  LEFT JOIN posts p ON p.user_id = u.id
		  GROUP BY u.id;
		CREATE MATERIALIZED VIEW user_stats_cached AS SELECT * FROM user_stats;
	`)
	require.NoError(t, err)

	state, err := inspector.Inspect(ctx, pool, inspector.Options{Schemas: []string{"public"}})
	require.NoError(t, err)

	vs := state.Views["public.user_stats"]
	require.NotNil(t, vs, "view user_stats should be inspected")
	require.Len(t, vs.Columns, 4, "expected 4 view columns")
	colTypes := map[string]string{}
	for _, c := range vs.Columns {
		colTypes[c.Name] = c.TypeSQL
	}
	require.Equal(t, "bigint", colTypes["id"])
	require.Equal(t, "bigint", colTypes["post_count"], "count(*) resolves to bigint")
	require.Equal(t, "text", colTypes["display"], "COALESCE(text, text) → text")
	require.Equal(t, "text", colTypes["optional_name"])

	// Materialized view goes through the same pg_attribute path.
	vc := state.Views["public.user_stats_cached"]
	require.NotNil(t, vc)
	require.True(t, vc.Materialized)
	require.Len(t, vc.Columns, 4)

	// Generate Go and confirm the struct has real fields, not the marker.
	g := NewGoGenerator()
	fs, err := g.Generate(state, Options{Package: "dbgen"})
	require.NoError(t, err)
	viewsGo := string(fs["views.go"])
	for _, want := range []string{
		"type UserStat struct",
		"*int64  `db:\"id\"",
		"*int64  `db:\"post_count\"",
		"*string `db:\"display\"",
	} {
		require.Contains(t, viewsGo, want, "Go view output missing %q:\n%s", want, viewsGo)
	}
	require.NotContains(t, viewsGo, "not yet inferred", "marker should be gone")

	// And TS.
	tg := NewTSGenerator()
	tsfs, err := tg.Generate(state, Options{})
	require.NoError(t, err)
	viewsTs := string(tsfs["views.ts"])
	for _, want := range []string{
		"export interface UserStat",
		"id: bigint | null",
		"post_count: bigint | null",
		"display: string | null",
	} {
		require.Contains(t, viewsTs, want, "TS view output missing %q:\n%s", want, viewsTs)
	}
	// Sanity: no leftover marker.
	require.NotContains(t, viewsTs, "not yet inferred")
	_ = os.Stdout
	_ = strings.TrimSpace
}
