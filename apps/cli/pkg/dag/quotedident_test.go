package dag

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUnquoteIdent_basic(t *testing.T) {
	// unquoteIdent strips double-quote delimiters but does NOT lowercase; that is normalizeObjKey's job.
	require.Equal(t, "My Type", unquoteIdent(`"My Type"`))
	// Schema-qualified: both parts have their quotes stripped.
	require.Equal(t, "public.My Type", unquoteIdent(`"public"."My Type"`))
	require.Equal(t, "plain", unquoteIdent("plain"))
	require.Equal(t, "", unquoteIdent(""))
}

func TestUnquoteIdent_escapedQuote(t *testing.T) {
	// "" inside quotes is an escaped double-quote
	require.Equal(t, `say "hello"`, unquoteIdent(`"say ""hello"""`))
}
// normalizeObjKey lowercases and strips quotes.

func TestNormalizeObjKey_quotedIdent(t *testing.T) {
	// normalizeObjKey lowercases and strips quotes.
	got := normalizeObjKey(`"My Schema"."My Type"`)
	require.Equal(t, "my schema.my type", got)
}

func TestNormalizeObjKey_bareIdent(t *testing.T) {
	require.Equal(t, "public.my_type", normalizeObjKey("my_type"))
	require.Equal(t, "public.my_type", normalizeObjKey("public.my_type"))
}
