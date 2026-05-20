package shadow

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nexg/pg-flux/pkg/differ"
	"github.com/nexg/pg-flux/pkg/inspector"
	"github.com/nexg/pg-flux/pkg/plan"
	"github.com/nexg/pg-flux/pkg/schema"
)

// ValidateStructuralEquivalence applies the full plan to an empty disposable database (semantic apply),
// inspects the resulting catalog, and fails if desired vs live still differs structurally.
//
// This is not a formal proof of equivalence with arbitrary production state; it is a strong
// check that the generated DDL matches the desired model when applied from scratch (same as
// semantic shadow apply, plus a second Diff against inspected state).
//
// Requires a disposable DSN; the database will be mutated.
func ValidateStructuralEquivalence(ctx context.Context, shadowDSN string, des *schema.SchemaState, p *plan.ExecutionPlan, opt differ.Options) error {
	if shadowDSN == "" {
		return fmt.Errorf("shadow: empty connection string")
	}
	if des == nil {
		return fmt.Errorf("equivalence: nil desired state")
	}
	if p == nil {
		return fmt.Errorf("equivalence: nil plan")
	}
	if err := ValidateSemanticOnDatabase(ctx, shadowDSN, p); err != nil {
		return fmt.Errorf("structural equivalence: apply failed: %w", err)
	}
	pool, err := pgxpool.New(ctx, shadowDSN)
	if err != nil {
		return err
	}
	defer pool.Close()
	live, err := inspector.Inspect(ctx, pool, inspector.Options{Schemas: targetSchemasFromDesired(des)})
	if err != nil {
		return fmt.Errorf("structural equivalence: inspect failed: %w", err)
	}
	// Shadow empty-apply path: avoid reltuple-driven hints against the shadow catalog.
	shadowOpt := opt
	shadowOpt.Reltuples = nil
	shadowOpt.SetNotNullReltupleThreshold = 0
	dr, err := differ.Diff(des, live, shadowOpt)
	if err != nil {
		return fmt.Errorf("structural equivalence: diff failed: %w", err)
	}
	if dr.Plan == nil || len(dr.Plan.Statements) == 0 {
		return nil
	}
	var b strings.Builder
	for _, s := range dr.Plan.Statements {
		if s.DDL == "" {
			continue
		}
		fmt.Fprintf(&b, "[%d] %s %s\n", s.ID, s.OpType, s.DDL)
		if b.Len() > 4000 {
			b.WriteString("…\n")
			break
		}
	}
	return fmt.Errorf("structural equivalence: shadow catalog still differs from desired after apply (%d steps remain):\n%s", len(dr.Plan.Statements), b.String())
}

func targetSchemasFromDesired(s *schema.SchemaState) []string {
	if s == nil || s.Tables == nil {
		return []string{"public"}
	}
	seen := map[string]struct{}{"public": {}}
	for k := range s.Tables {
		p := strings.SplitN(k, ".", 2)
		if len(p) < 1 {
			continue
		}
		seen[strings.ToLower(strings.TrimSpace(p[0]))] = struct{}{}
	}
	for k := range s.Views {
		p := strings.SplitN(k, ".", 2)
		if len(p) < 1 {
			continue
		}
		seen[strings.ToLower(strings.TrimSpace(p[0]))] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for sch := range seen {
		out = append(out, sch)
	}
	sort.Strings(out)
	return out
}
