package dag

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/nex-gen-tech/pg-flux/pkg/plan"
	"github.com/nex-gen-tech/pg-flux/pkg/schema"
)

// ErrDependencyCycle is returned when statement-level dependencies form a cycle.
var ErrDependencyCycle = fmt.Errorf("ddl dependency cycle: statements reference each other in a way that cannot be linearized")

var (
	reRefFK     = regexp.MustCompile(`(?i)REFERENCES\s+([a-z_][a-z0-9_]*(?:\.[a-z_][a-z0-9_]*)?)`)
	reRefOn     = regexp.MustCompile(`(?i)\bON\s+ONLY\s+([a-z_][a-z0-9_]*(?:\.[a-z_][a-z0-9_]*)?)\b`)
	reRefOn2    = regexp.MustCompile(`(?i)\bON\s+([a-z_][a-z0-9_]*(?:\.[a-z_][a-z0-9_]*)?)\s+(?:USING|FOR|TO)\b`)
	reFromJoin  = regexp.MustCompile(`(?i)\b(?:FROM|JOIN)\s+([a-z_][a-z0-9_]*(?:\.[a-z_][a-z0-9_]*)?)\b`)
	reTableQual = regexp.MustCompile(`^[a-z_][a-z0-9_]*\.[a-z_][a-z0-9_]*$`)
	// Index DDL: relation after ON (CONCURRENTLY may appear before INDEX; we scan full DDL).
	reIndexOnTable = regexp.MustCompile(`(?i)ON\s+(?:ONLY\s+)?([a-z_][a-z0-9_]*(?:\.[a-z_][a-z0-9_]*)?)(?:\s+USING|\s+WITH|\s*\(|\s+TABLESPACE)`)
	// Trigger: ... ON [schema.]rel before FOR EACH / EXECUTE / WHEN / FROM
	reTriggerOnTable = regexp.MustCompile(`(?i)\bON\s+([a-z_][a-z0-9_]*(?:\.[a-z_][a-z0-9_]*)?)(?:\s+FOR\s+EACH|\s+FROM|\s+WHEN|\s+EXECUTE)`)
	// EXECUTE { FUNCTION | PROCEDURE } [schema.]name
	reTriggerExec = regexp.MustCompile(`(?i)EXECUTE\s+(?:FUNCTION|PROCEDURE)\s+((?:[a-z_][a-z0-9_]*\.)?[a-z_][a-z0-9_]*)(?:\s*[\(;]|$)`)
)

// unquoteIdent strips double-quote delimiters from a (possibly quoted) PostgreSQL
// identifier or schema-qualified name and lowercases the result.
// E.g. `"My Type"` → `my type`, `"Pub"."My Type"` → `pub.my type`.
func unquoteIdent(s string) string {
	var b strings.Builder
	inQuote := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"' && inQuote && i+1 < len(s) && s[i+1] == '"':
			// escaped double-quote inside quoted ident
			b.WriteByte('"')
			i++
		case c == '"':
			inQuote = !inQuote
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

func normalizeObjKey(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return ""
	}
	s = unquoteIdent(s)
	if !strings.Contains(s, ".") {
		return "public." + s
	}
	return s
}

// statementProduces returns object keys this statement is treated as creating (for ordering).
func statementProduces(s plan.Statement) []string {
	if s.Object == "" {
		return nil
	}
	o := strings.TrimSpace(s.Object)
	switch s.OpType {
	case "CREATE_TABLE", "CREATE_VIEW", "CREATE_MATERIALIZED_VIEW", "CREATE_SEQUENCE",
		"CREATE_FUNCTION", "CREATE_AGGREGATE", "CREATE_WINDOW_FUNCTION",
		"CREATE_INDEX", "CREATE_EXTENSION", "RENAME_TABLE":
		return []string{normalizeObjKey(o)}
	case "TOGGLE_RLS", "TOGGLE_RLS_FORCE", "TOGGLE_RLS_NOFORCE":
		// alters existing table; table must already exist — not a new producer key
		return nil
	default:
		// ADD_COLUMN / constraints / etc. do not introduce a new top-level object key for the graph
		if strings.HasPrefix(s.OpType, "CREATE_") {
			return []string{normalizeObjKey(o)}
		}
		return nil
	}
}

// statementRequires parses DDL for relation references this step may need to exist first.
func statementRequires(s plan.Statement) []string {
	ddl := s.DDL
	if strings.TrimSpace(ddl) == "" {
		return nil
	}
	seen := map[string]struct{}{}
	add := func(k string) {
		k = normalizeObjKey(k)
		if k != "" {
			seen[k] = struct{}{}
		}
	}
	for _, re := range []*regexp.Regexp{reRefFK, reRefOn, reRefOn2, reFromJoin} {
		for _, m := range re.FindAllStringSubmatch(ddl, -1) {
			if len(m) > 1 {
				add(m[1])
			}
		}
	}
	// Index (incl. CONCURRENTLY): must run after the heap relation exists.
	if s.OpType == "CREATE_INDEX" {
		if m := reIndexOnTable.FindStringSubmatch(ddl); len(m) > 1 {
			add(m[1])
		}
	}
	// Triggers: after table and after referenced function/ procedure.
	if s.OpType == "CREATE_TRIGGER" {
		if m := reTriggerOnTable.FindStringSubmatch(ddl); len(m) > 1 {
			add(m[1])
		}
		for _, m := range reTriggerExec.FindAllStringSubmatch(ddl, -1) {
			if len(m) < 2 {
				continue
			}
			b := schema.FunctionDependencyKey(m[1])
			if b != "" {
				add(b)
			}
		}
	}
	// Object may be schema.table for ALTER / POLICY — table must exist
	if s.Object != "" && strings.Contains(s.Object, ".") && reTableQual.MatchString(strings.ToLower(s.Object)) {
		if strings.HasPrefix(s.OpType, "CREATE_POLICY") || s.OpType == "CREATE_POLICY" ||
			strings.Contains(s.OpType, "TOGGLE_RLS") || s.OpType == "SET_NOT_NULL" {
			p := strings.SplitN(s.Object, ".", 2)
			if len(p) == 2 {
				add(p[0] + "." + p[1])
			}
		}
	}
	for _, k := range ddlTypeRequires(s) {
		add(k)
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// registerProducers records object keys a statement materializes, including a coarse
// function key (schema.name) so trigger EXECUTE references can order after the function.
func registerProducers(producer map[string]int, i int, s plan.Statement) {
	for _, k := range statementProduces(s) {
		if k == "" {
			continue
		}
		producer[normalizeObjKey(k)] = i
	}
	switch s.OpType {
	case "CREATE_FUNCTION", "CREATE_AGGREGATE", "CREATE_WINDOW_FUNCTION":
		id := strings.TrimSpace(s.Object)
		if id == "" {
			return
		}
		b := schema.FunctionDependencyKey(id)
		if b == "" {
			return
		}
		producer[normalizeObjKey(b)] = i
	}
	registerDDLProducers(producer, i, s)
}

// TopologicalSortStatements orders statements by a dependency graph ( producer[i] before consumer[j] ),
// then by previous OpType score and original index. Returns ErrDependencyCycle if a cycle is detected.
func TopologicalSortStatements(in []plan.Statement) ([]plan.Statement, error) {
	if len(in) == 0 {
		return nil, nil
	}
	// producer[key] = last index that creates key (wins for stability)
	producer := make(map[string]int)
	for i := range in {
		registerProducers(producer, i, in[i])
	}
	n := len(in)
	adj := make([][]int, n)
	indeg := make([]int, n)
	for j := range in {
		for _, need := range statementRequires(in[j]) {
			if i, ok := producer[need]; ok && i != j {
				// i must come before j
				adj[i] = append(adj[i], j)
				indeg[j]++
			}
		}
	}
	type node struct {
		idx   int
		score int
		ord   int
	}
	ready := make([]node, 0, n)
	for i := range in {
		if indeg[i] == 0 {
			ready = append(ready, node{idx: i, score: score(in[i].OpType), ord: i})
		}
	}
	sort.Slice(ready, func(a, b int) bool {
		if ready[a].score != ready[b].score {
			return ready[a].score < ready[b].score
		}
		return ready[a].ord < ready[b].ord
	})
	out := make([]plan.Statement, 0, n)
	for len(ready) > 0 {
		u := ready[0]
		ready = ready[1:]
		out = append(out, in[u.idx])
		for _, v := range adj[u.idx] {
			indeg[v]--
			if indeg[v] == 0 {
				ready = append(ready, node{idx: v, score: score(in[v].OpType), ord: v})
			}
		}
		sort.Slice(ready, func(a, b int) bool {
			if ready[a].score != ready[b].score {
				return ready[a].score < ready[b].score
			}
			return ready[a].ord < ready[b].ord
		})
	}
	if len(out) != n {
		return nil, ErrDependencyCycle
	}
	return out, nil
}
