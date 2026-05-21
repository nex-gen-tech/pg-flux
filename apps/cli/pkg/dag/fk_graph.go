package dag

import (
	"sort"
	"strings"

	"github.com/nex-gen-tech/pg-flux/pkg/schema"
)

// FKCircularDependencyError reports a cycle in the desired FOREIGN KEY graph
// (among tables in the schema state).
type FKCircularDependencyError struct {
	Tables []string
}

func (e *FKCircularDependencyError) Error() string {
	if len(e.Tables) == 0 {
		return "fk dependency cycle in desired schema"
	}
	return "fk dependency cycle in desired schema involving: " + strings.Join(e.Tables, " → ")
}

// TableCreationRank returns sort keys for creating tables: lower rank = earlier
// (referenced parents before children). Returns a cycle error if the FK graph has a loop.
func TableCreationRank(s *schema.SchemaState) (map[string]int, error) {
	if s == nil || s.Tables == nil {
		return map[string]int{}, nil
	}
	var keys []string
	for k := range s.Tables {
		if s.Tables[k] == nil {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	children := make(map[string][]string) // parent table key -> child keys that reference it
	indeg := make(map[string]int)
	for _, k := range keys {
		indeg[k] = 0
	}
	for childKey, t := range s.Tables {
		if t == nil {
			continue
		}
		seenP := make(map[string]struct{})
		for _, fk := range t.ForeignKeys {
			if fk == nil {
				continue
			}
			pk := schema.ReferenceTableKeyFromDefSQL(fk.DefSQL)
			if pk == "" || pk == childKey {
				continue
			}
			if _, ok := s.Tables[pk]; !ok {
				continue
			}
			if _, d := seenP[pk]; d {
				continue
			}
			seenP[pk] = struct{}{}
			children[pk] = append(children[pk], childKey)
			indeg[childKey]++
		}
	}
	var q []string
	for _, k := range keys {
		if indeg[k] == 0 {
			q = append(q, k)
		}
	}
	sort.Strings(q)
	rank := make(map[string]int)
	steps := 0
	for len(q) > 0 {
		u := q[0]
		q = q[1:]
		if _, done := rank[u]; done {
			continue
		}
		rank[u] = steps
		steps++
		succ := append([]string(nil), children[u]...)
		sort.Strings(succ)
		for _, v := range succ {
			indeg[v]--
			if indeg[v] == 0 {
				q = append(q, v)
			}
		}
		sort.Strings(q)
	}
	if len(rank) < len(keys) {
		var cyc []string
		for _, k := range keys {
			if _, ok := rank[k]; !ok {
				cyc = append(cyc, k)
			}
		}
		sort.Strings(cyc)
		if len(cyc) > 10 {
			cyc = cyc[:10]
		}
		return nil, &FKCircularDependencyError{Tables: cyc}
	}
	return rank, nil
}

// ValidateSchemaFKGraph returns a non-nil error if the desired schema has a cyclic FK graph.
func ValidateSchemaFKGraph(s *schema.SchemaState) error {
	_, err := TableCreationRank(s)
	return err
}
