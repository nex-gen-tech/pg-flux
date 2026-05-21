package dump

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nex-gen-tech/pg-flux/pkg/schema"
)

func TestGuardOutputDir_emptyDir(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, guardOutputDir(dir, false))
}

func TestGuardOutputDir_nonEmptyDirRefuses(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "stuff.sql"), []byte("--"), 0o644))
	err := guardOutputDir(dir, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "--force")
}

func TestGuardOutputDir_force(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "stuff.sql"), []byte("--"), 0o644))
	require.NoError(t, guardOutputDir(dir, true))
}

func TestGuardOutputDir_missingDirCreated(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "new")
	require.NoError(t, guardOutputDir(dir, false))
	st, err := os.Stat(dir)
	require.NoError(t, err)
	require.True(t, st.IsDir())
}

func TestFileNameFor_safe(t *testing.T) {
	tests := []struct {
		in  object
		out string
	}{
		{object{Schema: "public", Name: "users"}, "public.users"},
		{object{Schema: "Audit-DB", Name: "we ird"}, "Audit-DB.we_ird"},
		{object{Schema: "", Name: "global"}, "global"},
	}
	for _, tc := range tests {
		got := fileNameFor(tc.in)
		require.Equal(t, tc.out, got)
	}
}

func TestRenderPolicyFromFields_smoke(t *testing.T) {
	p := &schema.Policy{
		Schema: "public", Table: "users", Name: "u_select",
		Cmd: "SELECT", Permissive: true, Roles: []string{"app_reader"},
		UsingSQL: "id = current_setting('app.uid')::bigint",
	}
	out := renderPolicyFromFields(p)
	for _, want := range []string{"CREATE POLICY u_select", "ON public.users", "FOR SELECT", "TO app_reader", "USING ("} {
		require.Contains(t, out, want)
	}
}

func TestRenderPolicyFromFields_restrictive(t *testing.T) {
	p := &schema.Policy{
		Schema: "public", Table: "t", Name: "deny",
		Cmd: "DELETE", Permissive: false, Roles: []string{"public"},
		UsingSQL: "false",
	}
	out := renderPolicyFromFields(p)
	require.Contains(t, out, "AS RESTRICTIVE")
	require.Contains(t, out, "TO PUBLIC")
}

func TestRenderPolicyFromFields_allCmd(t *testing.T) {
	// FOR ALL is the default; omit the FOR clause.
	p := &schema.Policy{
		Schema: "public", Table: "t", Name: "any",
		Cmd: "ALL", Permissive: true, Roles: []string{"app"},
		UsingSQL: "true",
	}
	out := renderPolicyFromFields(p)
	require.NotContains(t, out, "FOR ALL")
}

func TestIsIdentitySequence_bigserial(t *testing.T) {
	s := &schema.SchemaState{
		Tables: map[string]*schema.Table{
			"public.users": {
				Schema: "public", Name: "users",
				Columns: []*schema.Column{
					{Name: "id", DefaultSQL: "nextval('users_id_seq'::regclass)"},
				},
			},
		},
	}
	sq := &schema.Sequence{Schema: "public", Name: "users_id_seq", OwnedBy: "public.users.id"}
	require.True(t, isIdentitySequence(s, sq), "bigserial backing seq should be detected")
}

func TestIsIdentitySequence_freestanding(t *testing.T) {
	s := &schema.SchemaState{}
	sq := &schema.Sequence{Schema: "public", Name: "my_seq"}
	require.False(t, isIdentitySequence(s, sq), "no OwnedBy ⇒ not implicit")
}

func TestRenderPrivileges_skipsOwner(t *testing.T) {
	privs := []schema.Privilege{
		{Grantee: "app_owner", Priv: "SELECT"},
		{Grantee: "app_reader", Priv: "SELECT"},
	}
	out := renderPrivileges("TABLE", "public.t", "app_owner", privs)
	require.NotContains(t, out, "TO app_owner")
	require.Contains(t, out, "TO app_reader")
}

func TestRenderPrivileges_groupsByGrantee(t *testing.T) {
	privs := []schema.Privilege{
		{Grantee: "app", Priv: "SELECT"},
		{Grantee: "app", Priv: "INSERT"},
		{Grantee: "app", Priv: "UPDATE"},
	}
	out := renderPrivileges("TABLE", "public.t", "", privs)
	// Single GRANT line with all three privileges.
	lines := strings.Split(strings.TrimSpace(out), "\n")
	require.Len(t, lines, 1, "expected one GRANT line, got %d: %q", len(lines), out)
	require.Contains(t, lines[0], "INSERT")
	require.Contains(t, lines[0], "SELECT")
	require.Contains(t, lines[0], "UPDATE")
}

func TestVerify_emptyInputs(t *testing.T) {
	r := Verify(nil, nil)
	require.Equal(t, 0, r.Count())
}

func TestVerify_findsExtraTable(t *testing.T) {
	des := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.a": {Schema: "public", Name: "a"},
	}}
	live := &schema.SchemaState{Tables: map[string]*schema.Table{
		"public.a": {Schema: "public", Name: "a"},
		"public.b": {Schema: "public", Name: "b"},
	}}
	r := Verify(des, live)
	require.Equal(t, []string{"public.b"}, r.Tables)
}

// TestVerify_triggerDeclaredInSourceIsClean confirms that a trigger that exists
// in both source and live does NOT appear in the verify report.
func TestVerify_triggerDeclaredInSourceIsClean(t *testing.T) {
	tg := &schema.Trigger{Schema: "public", Table: "posts", Name: "posts_set_updated_at",
		DefSQL: "CREATE TRIGGER posts_set_updated_at BEFORE UPDATE ON public.posts FOR EACH ROW EXECUTE FUNCTION public.set_updated_at()"}
	k := schema.TriggerKey("public", "posts", "posts_set_updated_at")

	des := &schema.SchemaState{Triggers: map[string]*schema.Trigger{k: tg}}
	live := &schema.SchemaState{Triggers: map[string]*schema.Trigger{k: tg}}

	r := Verify(des, live)
	require.Equal(t, 0, r.Count(), "declared trigger should not appear as undeclared")
	require.Empty(t, r.Triggers)
}

// TestVerify_outOfBandTriggerIsReported confirms that a trigger present in live
// but absent from source is flagged as an undeclared object — the primary CI
// gate for out-of-band trigger additions.
func TestVerify_outOfBandTriggerIsReported(t *testing.T) {
	declared := &schema.Trigger{Schema: "public", Table: "posts", Name: "posts_set_updated_at",
		DefSQL: "CREATE TRIGGER posts_set_updated_at BEFORE UPDATE ON public.posts FOR EACH ROW EXECUTE FUNCTION public.set_updated_at()"}
	outOfBand := &schema.Trigger{Schema: "public", Table: "posts", Name: "posts_audit",
		DefSQL: "CREATE TRIGGER posts_audit AFTER INSERT OR UPDATE OR DELETE ON public.posts FOR EACH ROW EXECUTE FUNCTION public.audit_log()"}

	declaredKey := schema.TriggerKey("public", "posts", "posts_set_updated_at")
	outOfBandKey := schema.TriggerKey("public", "posts", "posts_audit")

	des := &schema.SchemaState{Triggers: map[string]*schema.Trigger{
		declaredKey: declared,
	}}
	live := &schema.SchemaState{Triggers: map[string]*schema.Trigger{
		declaredKey: declared,
		outOfBandKey: outOfBand,
	}}

	r := Verify(des, live)
	require.Equal(t, 1, r.Count(), "one undeclared trigger expected")
	require.Equal(t, []string{outOfBandKey}, r.Triggers, "out-of-band trigger key must be reported")
	require.NotContains(t, r.Triggers, declaredKey, "declared trigger must not be flagged")
}
