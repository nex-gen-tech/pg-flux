package differ

import (
	"fmt"
	"sort"
	"strings"

	"github.com/nexg/pg-flux/pkg/plan"
	"github.com/nexg/pg-flux/pkg/schema"
)

// diffFunctionMetadata compares the cheap-to-alter metadata fields on each function
// (volatility, security, parallel, leakproof, cost, rows, SET config) and emits
// minimal ALTER FUNCTION statements when they differ. The function body itself is
// handled by the existing function diff (CREATE OR REPLACE); this pass only fires
// for already-existing pairs whose body fingerprints match.
//
// All these alterations are non-disruptive (catalog-only updates), so no hazards.
func diffFunctionMetadata(d, l *schema.SchemaState) []change {
	var out []change
	if d == nil || l == nil {
		return out
	}
	for k, df := range d.Functions {
		if df == nil {
			continue
		}
		lf := l.Functions[k]
		if lf == nil {
			continue // CREATE handled by diffFunctions
		}
		var alters []string
		if df.Volatility != "" && !strings.EqualFold(df.Volatility, lf.Volatility) {
			alters = append(alters, df.Volatility)
		}
		if df.Security != "" && !strings.EqualFold(df.Security, lf.Security) {
			alters = append(alters, "SECURITY "+df.Security)
		}
		if df.Parallel != "" && !strings.EqualFold(df.Parallel, lf.Parallel) {
			alters = append(alters, "PARALLEL "+df.Parallel)
		}
		if df.LeakProof != lf.LeakProof {
			if df.LeakProof {
				alters = append(alters, "LEAKPROOF")
			} else {
				alters = append(alters, "NOT LEAKPROOF")
			}
		}
		if df.Cost > 0 && df.Cost != lf.Cost {
			alters = append(alters, fmt.Sprintf("COST %g", df.Cost))
		}
		if df.Rows > 0 && df.Rows != lf.Rows {
			alters = append(alters, fmt.Sprintf("ROWS %g", df.Rows))
		}
		// SET config: emit individual SET <key> TO <val> ... RESET for removed keys.
		for _, kv := range configDiff(df.Config, lf.Config) {
			alters = append(alters, kv)
		}
		if len(alters) == 0 {
			continue
		}
		kw := "FUNCTION"
		switch df.Kind {
		case "p":
			kw = "PROCEDURE"
		}
		for _, a := range alters {
			out = append(out, change{
				kind:   plan.ChangeRawSQL,
				rawSQL: fmt.Sprintf("ALTER %s %s %s", kw, df.Identity, a),
				sch:    "",
				tbl:    df.Identity,
			})
		}
	}
	return out
}

// configDiff returns the set of SET/RESET clauses needed to bring live config to desired.
// Inputs are slices of "key=val" strings (pg_proc.proconfig format).
func configDiff(desired, live []string) []string {
	dm := parseConfig(desired)
	lm := parseConfig(live)
	var out []string
	dkeys := make([]string, 0, len(dm))
	for k := range dm {
		dkeys = append(dkeys, k)
	}
	sort.Strings(dkeys)
	for _, k := range dkeys {
		dv := dm[k]
		if lv, ok := lm[k]; !ok || lv != dv {
			out = append(out, fmt.Sprintf("SET %s TO %s", k, formatConfigValue(dv)))
		}
	}
	lkeys := make([]string, 0, len(lm))
	for k := range lm {
		lkeys = append(lkeys, k)
	}
	sort.Strings(lkeys)
	for _, k := range lkeys {
		if _, ok := dm[k]; !ok {
			out = append(out, fmt.Sprintf("RESET %s", k))
		}
	}
	return out
}

func parseConfig(cfg []string) map[string]string {
	m := make(map[string]string, len(cfg))
	for _, kv := range cfg {
		i := strings.IndexByte(kv, '=')
		if i <= 0 {
			continue
		}
		m[strings.ToLower(strings.TrimSpace(kv[:i]))] = strings.TrimSpace(kv[i+1:])
	}
	return m
}

func formatConfigValue(v string) string {
	if v == "" {
		return "''"
	}
	// search_path values like "public, pg_temp" need quoting/parens depending on form.
	// Simplest robust form: single-quote the whole value, escape internal quotes.
	return "'" + strings.ReplaceAll(v, "'", "''") + "'"
}
