package schema

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReferenceTableKeyFromDefSQL(t *testing.T) {
	require.Equal(t, "public.a_parents", ReferenceTableKeyFromDefSQL(
		"FOREIGN KEY (parent_id) REFERENCES public.a_parents (id)"))
	require.Equal(t, "public.t", ReferenceTableKeyFromDefSQL(
		"FOREIGN KEY (x) REFERENCES t (y)"))
}
