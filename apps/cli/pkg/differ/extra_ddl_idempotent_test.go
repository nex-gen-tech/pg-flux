package differ

import (
	"testing"

	"github.com/nex-gen-tech/pg-flux/pkg/schema"
	"github.com/stretchr/testify/require"
)

func TestMakeExtraDDLIdempotent_createSchema(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"CREATE SCHEMA myschema", "CREATE SCHEMA IF NOT EXISTS myschema"},
		{"CREATE SCHEMA IF NOT EXISTS myschema", "CREATE SCHEMA IF NOT EXISTS myschema"},
		{"create schema public", "create schema IF NOT EXISTS public"},
	}
	for _, c := range cases {
		require.Equal(t, c.want, makeExtraDDLIdempotent(c.in), c.in)
	}
}

func TestMakeExtraDDLIdempotent_alterTypeAddValue(t *testing.T) {
	got := makeExtraDDLIdempotent("ALTER TYPE public.status ADD VALUE 'pending'")
	require.Contains(t, got, "IF NOT EXISTS")
	require.Contains(t, got, "'pending'")

	// Already has IF NOT EXISTS — should not double-insert.
	got2 := makeExtraDDLIdempotent("ALTER TYPE public.status ADD VALUE IF NOT EXISTS 'pending'")
	require.Equal(t, 1, countSubstring(got2, "IF NOT EXISTS"))
}

func TestMakeExtraDDLIdempotent_createType(t *testing.T) {
	got := makeExtraDDLIdempotent("CREATE TYPE public.mood AS ENUM ('happy', 'sad')")
	require.Contains(t, got, "DO $pgflux$")
	require.Contains(t, got, "duplicate_object")
	require.Contains(t, got, "CREATE TYPE public.mood")
}

func TestMakeExtraDDLIdempotent_createDomain(t *testing.T) {
	got := makeExtraDDLIdempotent("CREATE DOMAIN public.posint AS integer CHECK (value > 0)")
	require.Contains(t, got, "DO $pgflux$")
	require.Contains(t, got, "duplicate_object")
}

func TestMakeExtraDDLIdempotent_other(t *testing.T) {
	raw := "ALTER TABLE public.t ATTACH PARTITION public.t_2024 FOR VALUES FROM ('2024-01-01') TO ('2025-01-01')"
	require.Equal(t, raw, makeExtraDDLIdempotent(raw))
}

func TestMakeExtraDDLIdempotent_empty(t *testing.T) {
	require.Equal(t, "", makeExtraDDLIdempotent(""))
	require.Equal(t, "   ", makeExtraDDLIdempotent("   "))
}

func TestDiffMiscObjects(t *testing.T) {
	d := &schema.SchemaState{
		MiscObjects: []*schema.MiscObject{
			{Kind: "GRANT", DefSQL: "GRANT SELECT ON TABLE public.t TO app_user", Name: "GRANT"},
			{Kind: "GRANT", DefSQL: "REVOKE INSERT ON TABLE public.t FROM app_user", Name: "GRANT"},
		},
	}
	out := diffMiscObjects(d)
	require.Len(t, out, 2)
	require.Equal(t, "GRANT SELECT ON TABLE public.t TO app_user", out[0].rawSQL)
	require.Equal(t, "REVOKE INSERT ON TABLE public.t FROM app_user", out[1].rawSQL)
}

func TestDiffMiscObjects_nilOrEmpty(t *testing.T) {
	require.Nil(t, diffMiscObjects(nil))
	require.Nil(t, diffMiscObjects(&schema.SchemaState{}))
	require.Nil(t, diffMiscObjects(&schema.SchemaState{MiscObjects: []*schema.MiscObject{{Kind: "GRANT", DefSQL: "  ", Name: "GRANT"}}}))
}

// countSubstring counts non-overlapping occurrences of sub in s.
func countSubstring(s, sub string) int {
	count := 0
	for i := 0; i+len(sub) <= len(s); {
		if s[i:i+len(sub)] == sub {
			count++
			i += len(sub)
		} else {
			i++
		}
	}
	return count
}
