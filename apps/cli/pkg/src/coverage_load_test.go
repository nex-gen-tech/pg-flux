package src

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nex-gen-tech/pg-flux/pkg/schema"
	"github.com/stretchr/testify/require"
)

func TestLoadDesiredState_broadSurface(t *testing.T) {
	dir := t.TempDir()
	sql := `
CREATE TABLE public.base_tbl (id int PRIMARY KEY, n text DEFAULT 'x');
CREATE INDEX idx_n ON public.base_tbl (n);
CREATE SEQUENCE public.s1;
CREATE OR REPLACE FUNCTION public.f1(i int) RETURNS int
  LANGUAGE sql IMMUTABLE AS 'SELECT i + 1';
CREATE VIEW public.v1 AS SELECT id FROM public.base_tbl;
CREATE MATERIALIZED VIEW public.mv1 AS SELECT id FROM public.base_tbl;
CREATE POLICY p1 ON public.base_tbl FOR ALL TO public USING (true);
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE TYPE public.ctype AS ENUM ('a', 'b');
CREATE TABLE public.child (id int PRIMARY KEY REFERENCES public.base_tbl (id));
CREATE TRIGGER tr1 AFTER INSERT ON public.base_tbl FOR EACH ROW EXECUTE FUNCTION public.f1(0);
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "all.sql"), []byte(sql), 0o644))
	st, err := LoadDesiredState(LoadOptions{SchemaDir: dir, ValidateSQL: true})
	require.NoError(t, err)
	require.NotNil(t, st.Tables[schema.TableKey("public", "base_tbl")])
	// V2-A: CREATE TYPE (enum) is captured as ExtraDDL, not silently dropped.
	var foundCtype bool
	for _, x := range st.ExtraDDL {
		if strings.Contains(strings.ToLower(x), "create type") && strings.Contains(x, "ctype") {
			foundCtype = true
			break
		}
	}
	require.True(t, foundCtype, "expected CREATE TYPE public.ctype in ExtraDDL")
	if len(st.Indexes) > 0 {
		_ = st.Indexes
	}
}
