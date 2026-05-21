package src

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestLoadDesiredState_EnumPopulatesEnumsMap verifies that a CREATE TYPE ... AS ENUM
// statement in a source file correctly populates SchemaState.Enums with an EnumType
// that has the right schema, name, and ordered values.
func TestLoadDesiredState_EnumPopulatesEnumsMap(t *testing.T) {
	dir := t.TempDir()
	sql := "CREATE TYPE public.foo AS ENUM ('a', 'b', 'c');\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "types.sql"), []byte(sql), 0o644))

	st, err := LoadDesiredState(LoadOptions{SchemaDir: dir})
	require.NoError(t, err)

	key := "public.foo"

	// Structured Enums map.
	require.NotNil(t, st.Enums, "Enums map must be populated")
	e, ok := st.Enums[key]
	require.True(t, ok, "Enums map must contain %q", key)
	require.Equal(t, "public", e.Schema)
	require.Equal(t, "foo", e.Name)
	require.Equal(t, []string{"a", "b", "c"}, e.Values, "enum values must be in declaration order")

	// Legacy EnumValues map must also be populated for backward compatibility.
	require.Contains(t, st.EnumValues, key, "EnumValues must also be populated for backward compat")
	require.Equal(t, []string{"a", "b", "c"}, st.EnumValues[key])

	// UserTypes must contain the key.
	_, inUT := st.UserTypes[key]
	require.True(t, inUT, "UserTypes must contain the enum key")
}

// TestLoadDesiredState_EnumDefaultSchema verifies that an unqualified CREATE TYPE
// defaults to the public schema in Enums.
func TestLoadDesiredState_EnumDefaultSchema(t *testing.T) {
	dir := t.TempDir()
	sql := "CREATE TYPE bar AS ENUM ('x', 'y');\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "t.sql"), []byte(sql), 0o644))

	st, err := LoadDesiredState(LoadOptions{SchemaDir: dir})
	require.NoError(t, err)

	key := "public.bar"
	e, ok := st.Enums[key]
	require.True(t, ok, "unqualified enum must default to public schema in Enums map")
	require.Equal(t, "public", e.Schema)
	require.Equal(t, "bar", e.Name)
	require.Equal(t, []string{"x", "y"}, e.Values)
}

// TestLoadDesiredState_MultipleEnums verifies that multiple enum declarations in
// separate files are all captured into Enums.
func TestLoadDesiredState_MultipleEnums(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.sql"),
		[]byte("CREATE TYPE public.color AS ENUM ('red', 'green', 'blue');\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.sql"),
		[]byte("CREATE TYPE public.priority AS ENUM ('low', 'high');\n"), 0o644))

	st, err := LoadDesiredState(LoadOptions{SchemaDir: dir})
	require.NoError(t, err)

	require.Contains(t, st.Enums, "public.color")
	require.Contains(t, st.Enums, "public.priority")
	require.Equal(t, []string{"red", "green", "blue"}, st.Enums["public.color"].Values)
	require.Equal(t, []string{"low", "high"}, st.Enums["public.priority"].Values)
}
