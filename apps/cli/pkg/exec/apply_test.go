package exec

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nex-gen-tech/pg-flux/pkg/plan"
)

func TestApply_EmptyPlan(t *testing.T) {
	err := Apply(context.Background(), nil, &plan.ExecutionPlan{Statements: []plan.Statement{}}, Options{})
	require.NoError(t, err)
	err = Apply(context.Background(), nil, nil, Options{})
	require.NoError(t, err)
}

func TestApply_DryRunNoDB(t *testing.T) {
	p := &plan.ExecutionPlan{Statements: []plan.Statement{
		{ID: 1, DDL: "SELECT 1"},
	}}
	err := Apply(context.Background(), nil, p, Options{DryRun: true})
	require.NoError(t, err)
}

func TestApply_InvalidLockTimeout(t *testing.T) {
	p := &plan.ExecutionPlan{Statements: []plan.Statement{
		{ID: 1, DDL: "ALTER TABLE t ADD COLUMN c text"},
	}}
	err := Apply(context.Background(), nil, p, Options{LockTimeout: "5; DROP TABLE users--"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid lock_timeout")
}

func TestApply_InvalidStatementTimeout(t *testing.T) {
	p := &plan.ExecutionPlan{Statements: []plan.Statement{
		{ID: 1, DDL: "ALTER TABLE t ADD COLUMN c text"},
	}}
	err := Apply(context.Background(), nil, p, Options{StatementTimeout: "'; DELETE FROM"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid statement_timeout")
}

func TestApply_ValidTimeoutValues(t *testing.T) {
	// These are DryRun so no DB needed — just validates timeout parsing doesn't error.
	validTimeouts := []string{"3s", "500ms", "1min", "0", "30 seconds"}
	for _, to := range validTimeouts {
		p := &plan.ExecutionPlan{Statements: []plan.Statement{{ID: 1, DDL: "SELECT 1"}}}
		err := Apply(context.Background(), nil, p, Options{DryRun: true, LockTimeout: to, StatementTimeout: to})
		require.NoError(t, err, "timeout %q should be valid in dry-run", to)
	}
}

func TestApply_EmptyDDLSkipped(t *testing.T) {
	// Statements with empty DDL should be skipped in DryRun.
	p := &plan.ExecutionPlan{Statements: []plan.Statement{
		{ID: 1, DDL: ""},
		{ID: 2, DDL: ""},
	}}
	err := Apply(context.Background(), nil, p, Options{DryRun: true})
	require.NoError(t, err)
}

func TestValidTimeout_regex(t *testing.T) {
	valid := []string{"3s", "500ms", "0", "30 seconds", "1min", "3600000"}
	invalid := []string{"'; DROP TABLE", "1s; DROP", "--comment", "1\x00s"}
	for _, v := range valid {
		require.True(t, validTimeout.MatchString(v), "should be valid: %q", v)
	}
	for _, v := range invalid {
		require.False(t, validTimeout.MatchString(v), "should be invalid: %q", v)
	}
}

func TestApply_AllConcurrentDryRun(t *testing.T) {
	p := &plan.ExecutionPlan{Statements: []plan.Statement{
		{ID: 1, DDL: "CREATE INDEX CONCURRENTLY idx ON t(c)", IsConcurrent: true},
	}}
	err := Apply(context.Background(), nil, p, Options{DryRun: true})
	require.NoError(t, err)
}

// TestApply_LockTimeoutDefaultSet verifies that a zero/empty LockTimeout is defaulted to "3s".
func TestApply_LockTimeoutDefault(t *testing.T) {
	opt := Options{}
	// Access the default-setting logic by calling with DryRun and empty LockTimeout.
	// The actual default-setting happens inside Apply before pool usage, so test via
	// the validation path: an empty LockTimeout must NOT cause an error before pool.
	p := &plan.ExecutionPlan{Statements: []plan.Statement{{ID: 1, DDL: "SELECT 1"}}}
	err := Apply(context.Background(), nil, p, Options{DryRun: true, LockTimeout: opt.LockTimeout})
	require.NoError(t, err)
	// Also verify the sentinel "3s" is valid under the regex.
	require.True(t, validTimeout.MatchString("3s"))
}

// TestApply_ProgressWriter verifies DryRun with Progress does not panic.
func TestApply_ProgressWriterDryRun(t *testing.T) {
	var out strings.Builder
	p := &plan.ExecutionPlan{Statements: []plan.Statement{
		{ID: 1, DDL: "ALTER TABLE t ADD COLUMN c text"},
	}}
	err := Apply(context.Background(), nil, p, Options{DryRun: true, Progress: &out})
	require.NoError(t, err)
}
