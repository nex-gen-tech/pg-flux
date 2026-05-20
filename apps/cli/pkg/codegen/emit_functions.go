package codegen

import (
	"fmt"
	"strings"

	"github.com/nexg/pg-flux/pkg/schema"
)

// shouldEmitFn reports whether codegen should emit types for a given function.
// Aggregates ('a') and window functions ('w') are call-from-SQL constructs and
// don't fit the params/result shape; they're skipped. Procedures ('p') and
// regular functions ('f') are emitted.
func shouldEmitFn(f *schema.Function) bool {
	if f == nil {
		return false
	}
	return f.Kind == "" || f.Kind == "f" || f.Kind == "p"
}

// fnTypeName returns the PascalCase identifier for a function's generated type
// family — e.g. "calculate_score" → "CalculateScore". The user-facing types
// are "<Name>Params" and "<Name>Result" / "<Name>Row".
func fnTypeName(f *schema.Function, overrides map[string]string) string {
	if overrides != nil {
		full := f.Schema + "." + f.Name
		if v, ok := overrides[full]; ok {
			return v
		}
		if v, ok := overrides[f.Name]; ok {
			return v
		}
	}
	return PascalCaseInit(f.Name)
}

// --- Go ---

// emitGoFunctions writes one chunk per function: a Params struct (when args
// exist), and either a Result struct (RETURNS TABLE / OUT args) or a Row alias
// (scalar return). Procedures only emit Params. Aggregate / window are skipped.
func (g *GoGenerator) emitGoFunctions(s *schema.SchemaState, tm TypeMap, opts Options) (string, []string, bool) {
	if !opts.Emit.Functions || len(s.Functions) == 0 {
		return "", nil, false
	}
	var b strings.Builder
	var imports []string
	any := false
	for _, k := range SortedKeys(s.Functions) {
		f := s.Functions[k]
		if !shouldEmitFn(f) {
			continue
		}
		fnName := fnTypeName(f, g.NameOverrides)
		// Params struct (only when the function takes input args).
		if len(f.Args) > 0 {
			fmt.Fprintf(&b, "// %sParams are the input parameters for %s.%s.\n", fnName, f.Schema, f.Name)
			fmt.Fprintf(&b, "type %sParams struct {\n", fnName)
			for i, a := range f.Args {
				name := paramName(a, i)
				field := PascalCaseInit(name)
				typeExpr, imps := tm.Map(a.Type, false)
				imports = append(imports, imps...)
				tagKey := opts.Emit.ApplyColumnCase(name)
				comment := paramComment(a)
				if comment != "" {
					fmt.Fprintf(&b, "\t// %s\n", comment)
				}
				fmt.Fprintf(&b, "\t%s %s `db:%q json:%q`\n", field, typeExpr, tagKey, tagKey)
			}
			b.WriteString("}\n\n")
			any = true
		}
		// Result type. Procedures (Kind == "p") don't return anything.
		if f.Kind == "p" {
			continue
		}
		if len(f.ReturnsTable) > 0 {
			fmt.Fprintf(&b, "// %sResult is one row returned by %s.%s.\n", fnName, f.Schema, f.Name)
			fmt.Fprintf(&b, "type %sResult struct {\n", fnName)
			for _, a := range f.ReturnsTable {
				name := paramName(a, 0)
				field := PascalCaseInit(name)
				typeExpr, imps := tm.Map(a.Type, false)
				imports = append(imports, imps...)
				tagKey := opts.Emit.ApplyColumnCase(name)
				fmt.Fprintf(&b, "\t%s %s `db:%q json:%q`\n", field, typeExpr, tagKey, tagKey)
			}
			b.WriteString("}\n\n")
			any = true
		} else if f.ReturnType != "" && !isVoidLike(f.ReturnType) {
			// Scalar return → type alias for readability.
			typeExpr, imps := tm.Map(f.ReturnType, false)
			imports = append(imports, imps...)
			fmt.Fprintf(&b, "// %sRow is the scalar value returned by %s.%s.\n", fnName, f.Schema, f.Name)
			fmt.Fprintf(&b, "type %sRow = %s\n\n", fnName, typeExpr)
			any = true
		}
	}
	if !any {
		return "", nil, false
	}
	return b.String(), imports, true
}

// --- TypeScript ---

// emitTSFunctions is the TS analog of emitGoFunctions.
func (g *TSGenerator) emitTSFunctions(s *schema.SchemaState, opts Options) (string, bool) {
	if !opts.Emit.Functions || len(s.Functions) == 0 {
		return "", false
	}
	var b strings.Builder
	any := false
	for _, k := range SortedKeys(s.Functions) {
		f := s.Functions[k]
		if !shouldEmitFn(f) {
			continue
		}
		fnName := fnTypeName(f, g.NameOverrides)
		if len(f.Args) > 0 {
			fmt.Fprintf(&b, "/** Input parameters for %s.%s. */\n", f.Schema, f.Name)
			fmt.Fprintf(&b, "export interface %sParams {\n", fnName)
			for i, a := range f.Args {
				name := paramName(a, i)
				key := opts.Emit.ApplyColumnCase(name)
				typeExpr, _ := opts.TypeMap.Map(a.Type, false)
				// HasDefault → optional, regardless of NullStyle: the parameter
				// is optional because the function provides a default, not
				// because it's nullable.
				suffix := ":"
				if a.HasDefault {
					suffix = "?:"
				}
				fmt.Fprintf(&b, "  %s%s %s;\n", key, suffix, typeExpr)
			}
			b.WriteString("}\n\n")
			any = true
		}
		if f.Kind == "p" {
			continue
		}
		if len(f.ReturnsTable) > 0 {
			fmt.Fprintf(&b, "/** One row returned by %s.%s. */\n", f.Schema, f.Name)
			fmt.Fprintf(&b, "export interface %sResult {\n", fnName)
			for _, a := range f.ReturnsTable {
				name := paramName(a, 0)
				key := opts.Emit.ApplyColumnCase(name)
				typeExpr, _ := opts.TypeMap.Map(a.Type, false)
				fmt.Fprintf(&b, "  %s: %s;\n", key, typeExpr)
			}
			b.WriteString("}\n\n")
			any = true
		} else if f.ReturnType != "" && !isVoidLike(f.ReturnType) {
			typeExpr, _ := opts.TypeMap.Map(f.ReturnType, false)
			fmt.Fprintf(&b, "/** Scalar value returned by %s.%s. */\n", f.Schema, f.Name)
			fmt.Fprintf(&b, "export type %sRow = %s;\n\n", fnName, typeExpr)
			any = true
		}
	}
	return b.String(), any
}

// paramName produces a stable identifier for an unnamed parameter. PG lets you
// omit names; positional fallbacks are "arg1", "arg2", ... matching pg_get_function_arguments.
func paramName(a schema.FunctionArg, idx int) string {
	if a.Name != "" {
		return a.Name
	}
	return fmt.Sprintf("arg%d", idx+1)
}

// paramComment surfaces the variadic / inout mode as a brief documentation hint.
func paramComment(a schema.FunctionArg) string {
	switch a.Mode {
	case "v":
		return "VARIADIC"
	case "b":
		return "INOUT"
	}
	return ""
}

// isVoidLike reports whether a return type means "no result row" — void or
// trigger or event_trigger (trigger functions don't return data the caller reads).
func isVoidLike(t string) bool {
	t = strings.ToLower(strings.TrimSpace(t))
	switch t {
	case "void", "trigger", "event_trigger":
		return true
	}
	return false
}
