//go:build integration

package dump

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/nex-gen-tech/pg-flux/pkg/inspector"
	"github.com/nex-gen-tech/pg-flux/pkg/src"
)

// TestVerify_enumStructured_applyThenVerifyAndOutOfBand is the integration gate
// for first-class enum support (Step 6 of the feature spec). It exercises the
// full cycle:
//
//  1. Apply a schema that declares CREATE TYPE public.test_status AS ENUM ('a','b').
//  2. Verify against that live state → must be clean.
//  3. Create public.test_status2 directly in the DB (out-of-band).
//  4. Verify again → must report public.test_status2 as undeclared,
//     must NOT report public.test_status as undeclared.
func TestVerify_enumStructured_applyThenVerifyAndOutOfBand(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	pool := freshDB(t, ctx, "pgflux_enum_structured")
	ensureRoles(t, ctx, pool)

	// Step 1: Apply the declared enum type + a table that uses it.
	_, err := pool.Exec(ctx, `
		CREATE TYPE public.test_status AS ENUM ('a', 'b');
		CREATE TABLE public.items (
			id     bigserial PRIMARY KEY,
			status public.test_status NOT NULL DEFAULT 'a'
		);
	`)
	require.NoError(t, err)

	// Write matching source files.
	srcDir := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(srcDir, "types.sql"),
		[]byte("CREATE TYPE public.test_status AS ENUM ('a', 'b');\n"),
		0o644))
	require.NoError(t, os.WriteFile(
		filepath.Join(srcDir, "items.sql"),
		[]byte(`CREATE TABLE public.items (
	id     bigserial PRIMARY KEY,
	status public.test_status NOT NULL DEFAULT 'a'
);`), 0o644))

	desired, err := src.LoadDesiredState(src.LoadOptions{SchemaDir: srcDir})
	require.NoError(t, err)

	// Sanity: loader populated the Enums map.
	require.NotNil(t, desired.Enums, "loader must populate Enums map")
	e, ok := desired.Enums["public.test_status"]
	require.True(t, ok, "loader must capture public.test_status in Enums")
	require.Equal(t, []string{"a", "b"}, e.Values, "enum values must match declaration order")

	// Step 2: Inspect and verify — must be clean.
	live, err := inspector.Inspect(ctx, pool, inspector.Options{Schemas: []string{"public"}})
	require.NoError(t, err)

	// Inspector must populate the structured Enums map.
	require.NotNil(t, live.Enums, "inspector must populate Enums map")
	require.Contains(t, live.Enums, "public.test_status",
		"inspector must include public.test_status in Enums")

	report1 := Verify(desired, live)
	if report1.Count() != 0 {
		t.Fatalf("expected clean verify after applying declared schema; got %d undeclared: enums=%v tables=%v",
			report1.Count(), report1.Enums, report1.Tables)
	}

	// Step 3: Create an out-of-band enum directly in the DB.
	_, err = pool.Exec(ctx, `CREATE TYPE public.test_status2 AS ENUM ('x', 'y');`)
	require.NoError(t, err)

	// Step 4: Re-inspect and verify.
	live2, err := inspector.Inspect(ctx, pool, inspector.Options{Schemas: []string{"public"}})
	require.NoError(t, err)

	report2 := Verify(desired, live2)
	require.Contains(t, report2.Enums, "public.test_status2",
		"out-of-band enum must be reported as undeclared")
	require.NotContains(t, report2.Enums, "public.test_status",
		"declared enum must NOT be reported as undeclared")
	require.Greater(t, report2.Count(), 0,
		"verify must be non-clean when an out-of-band enum exists")
}
