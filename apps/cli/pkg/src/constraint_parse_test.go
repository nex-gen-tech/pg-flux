package src

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nexg/pg-flux/pkg/schema"
)

func TestLoadDesiredState_UniqueAndExclude(t *testing.T) {
	dir := t.TempDir()
	sql := `CREATE TABLE public.t_con (
  id int PRIMARY KEY,
  r int NOT NULL,
  CONSTRAINT uq_r UNIQUE (r),
  CONSTRAINT ex_r EXCLUDE USING btree (r WITH =)
);`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "c.sql"), []byte(sql), 0o644))
	st, err := LoadDesiredState(LoadOptions{SchemaDir: dir})
	require.NoError(t, err)
	tb := st.Tables[schema.TableKey("public", "t_con")]
	require.NotNil(t, tb)
	require.Len(t, tb.Uniques, 1)
	require.Equal(t, "uq_r", tb.Uniques[0].Name)
	require.Contains(t, strings.ToLower(tb.Uniques[0].DefSQL), "unique")
	require.Len(t, tb.Excludes, 1)
	require.Equal(t, "ex_r", tb.Excludes[0].Name)
	require.Contains(t, strings.ToLower(tb.Excludes[0].DefSQL), "exclude")
}
