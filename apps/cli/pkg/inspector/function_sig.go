package inspector

import "github.com/nex-gen-tech/pg-flux/pkg/schema"

// buildFunctionSignature populates fn.Args / ReturnType / ReturnsSet / ReturnsTable
// from the per-position arrays the pg_proc query returns.
//
// Arrays semantics (per PG docs):
//   - argNames    : []string per arg; "" when unnamed
//   - argTypesFmt : formatted PG type per arg (already format_type'd)
//   - argModes    : "i"/"o"/"b"/"v"/"t" per arg; empty when every arg is IN
//   - nargDefaults: count of TRAILING args that have a DEFAULT
//   - returnType  : format_type(prorettype) — "void", "integer", "TABLE", "trigger", ...
//   - returnsSet  : true for SETOF / RETURNS TABLE
//
// Convention used downstream by codegen:
//   - IN / INOUT / VARIADIC → fn.Args
//   - OUT / TABLE          → fn.ReturnsTable (the synthesised row type)
//   - Bare scalar return    → fn.ReturnType, ReturnsSet preserved
func buildFunctionSignature(fn *schema.Function, argNames, argTypesFmt, argModes []string, nargDefaults int, returnType string, returnsSet bool) {
	fn.ReturnType = returnType
	fn.ReturnsSet = returnsSet
	n := len(argTypesFmt)
	if n == 0 {
		return
	}
	hasModes := len(argModes) > 0
	// Count input-eligible args from the end so we can flag the trailing
	// nargDefaults of them as HasDefault. Only "i" / "b" / "v" args take part
	// in the defaults trail; PG enforces this.
	inputIdx := []int{}
	for i := 0; i < n; i++ {
		m := "i"
		if hasModes && i < len(argModes) {
			m = argModes[i]
		}
		if m == "i" || m == "b" || m == "v" {
			inputIdx = append(inputIdx, i)
		}
	}
	defaultFrom := len(inputIdx) - nargDefaults
	defaultsSet := map[int]bool{}
	if nargDefaults > 0 && defaultFrom >= 0 {
		for j := defaultFrom; j < len(inputIdx); j++ {
			defaultsSet[inputIdx[j]] = true
		}
	}
	for i := 0; i < n; i++ {
		mode := "i"
		if hasModes && i < len(argModes) {
			mode = argModes[i]
		}
		name := ""
		if i < len(argNames) {
			name = argNames[i]
		}
		arg := schema.FunctionArg{
			Name:       name,
			Type:       argTypesFmt[i],
			Mode:       mode,
			HasDefault: defaultsSet[i],
		}
		switch mode {
		case "o", "t":
			fn.ReturnsTable = append(fn.ReturnsTable, arg)
		default:
			fn.Args = append(fn.Args, arg)
		}
	}
}
