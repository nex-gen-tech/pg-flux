package differ

import (
	"regexp"
	"sort"
	"strings"

	"github.com/nex-gen-tech/pg-flux/pkg/schema"
)

var reViewFrom = regexp.MustCompile(`(?i)\b(?:from|join)\s+([a-z_][a-z0-9_]*\.[a-z_][a-z0-9_]*|[a-z_][a-z0-9_]*)`)

// viewRank returns a stable ordering index (lower = earlier) for CREATE VIEW steps
// using a best-effort dependency order on referenced relations.
func viewRank(des *schema.SchemaState) map[string]int {
	if des == nil || des.Views == nil {
		return nil
	}
	keys := make([]string, 0, len(des.Views))
	for k, v := range des.Views {
		if v == nil {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	// dep[v] = set of other view keys v depends on (by name)
	dep := make(map[string]map[string]struct{}, len(keys))
	for _, k := range keys {
		v := des.Views[k]
		if v == nil {
			continue
		}
		m := make(map[string]struct{})
		for _, sub := range reViewFrom.FindAllStringSubmatch(v.DefSQL, -1) {
			if len(sub) < 2 {
				continue
			}
			ref := strings.ToLower(strings.TrimSpace(sub[1]))
			var vk string
			if strings.Contains(ref, ".") {
				p := strings.SplitN(ref, ".", 2)
				vk = schema.TableKey(p[0], p[1])
			} else {
				// unqualified: assume public
				vk = schema.TableKey("public", ref)
			}
			if des.Views[vk] != nil {
				m[vk] = struct{}{}
			}
		}
		dep[k] = m
	}
	// Kahn-style level assignment (iterative for simple cycles: stay at 0)
	rank := make(map[string]int, len(keys))
	changed := true
	for i := 0; i < len(keys)+2 && changed; i++ {
		changed = false
		for _, k := range keys {
			maxp := 0
			for d := range dep[k] {
				if r, ok := rank[d]; ok && r+1 > maxp {
					maxp = r + 1
				}
			}
			if rank[k] != maxp {
				rank[k] = maxp
				changed = true
			}
		}
	}
	return rank
}
