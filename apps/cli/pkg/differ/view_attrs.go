package differ

import (
	"fmt"
	"sort"
	"strings"

	"github.com/nex-gen-tech/pg-flux/pkg/pgver"
	"github.com/nex-gen-tech/pg-flux/pkg/plan"
	"github.com/nex-gen-tech/pg-flux/pkg/schema"
)

// diffViewAttrs emits ALTER VIEW SET (option = value) / RESET (option) statements
// for changes to check_option, security_barrier, security_invoker (PG15+) that
// don't require rebuilding the view. Body changes are still handled by diffViews
// (DROP+CREATE because PG can't ALTER a view body in place beyond CREATE OR REPLACE
// which has its own restrictions).
func diffViewAttrs(d, l *schema.SchemaState) []change {
	var out []change
	if d == nil || l == nil {
		return out
	}
	for k, dv := range d.Views {
		if dv == nil {
			continue
		}
		lv := l.Views[k]
		if lv == nil {
			continue
		}
		var setPairs, resetKeys []string
		if !strings.EqualFold(dv.CheckOption, lv.CheckOption) {
			if dv.CheckOption == "" {
				resetKeys = append(resetKeys, "check_option")
			} else {
				setPairs = append(setPairs, "check_option = "+dv.CheckOption)
			}
		}
		if dv.SecurityBarrier != lv.SecurityBarrier {
			setPairs = append(setPairs, fmt.Sprintf("security_barrier = %t", dv.SecurityBarrier))
		}
		if dv.SecurityInvoker != lv.SecurityInvoker {
			// PG15+ — fail-loud is registered in checkServerCompat below.
			setPairs = append(setPairs, fmt.Sprintf("security_invoker = %t", dv.SecurityInvoker))
		}
		sort.Strings(setPairs)
		sort.Strings(resetKeys)
		kw := "VIEW"
		if dv.Materialized {
			kw = "MATERIALIZED VIEW"
		}
		if len(setPairs) > 0 {
			out = append(out, change{
				kind: plan.ChangeRawSQL,
				rawSQL: fmt.Sprintf("ALTER %s %s.%s SET (%s)",
					kw, ident(dv.Schema), ident(dv.Name), strings.Join(setPairs, ", ")),
				tbl: schema.ViewKey(dv.Schema, dv.Name),
			})
		}
		if len(resetKeys) > 0 {
			out = append(out, change{
				kind: plan.ChangeRawSQL,
				rawSQL: fmt.Sprintf("ALTER %s %s.%s RESET (%s)",
					kw, ident(dv.Schema), ident(dv.Name), strings.Join(resetKeys, ", ")),
				tbl: schema.ViewKey(dv.Schema, dv.Name),
			})
		}
	}
	return out
}

// hasViewSecurityInvoker reports whether any desired view sets security_invoker = true.
// security_invoker on views is a PG15+ feature.
func hasViewSecurityInvoker(s *schema.SchemaState) bool {
	if s == nil {
		return false
	}
	for _, v := range s.Views {
		if v != nil && v.SecurityInvoker {
			return true
		}
	}
	return false
}

// init registers view security_invoker as PG15-gated. Done from a small init to
// avoid touching compat.go's check table list directly (which is var-scope).
func init() {
	registerCompat(pgver.FeatureSecurityInvokerView, hasViewSecurityInvoker)
}
