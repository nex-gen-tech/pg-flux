package hashstate

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/nex-gen-tech/pg-flux/pkg/schema"
)

// OfSchemaState returns a stable hex digest of the model (for JSON metadata).
func OfSchemaState(s *schema.SchemaState) string {
	if s == nil {
		return "0"
	}
	var keys []string
	for k := range s.Tables {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		t := s.Tables[k]
		if t == nil {
			continue
		}
		fmt.Fprintf(&b, "T:%s:%s:%s|", k, t.Schema, t.Name)
		for _, c := range t.Columns {
			if c == nil {
				continue
			}
			fmt.Fprintf(&b, "C:%s:%s:%v:%s:%s|", c.Name, c.TypeSQL, c.NotNull, c.DefaultSQL, c.RenameFrom)
		}
		for _, c := range t.Checks {
			if c == nil {
				continue
			}
			fmt.Fprintf(&b, "K:%s:%s|", c.Name, c.DefSQL)
		}
		for _, c := range t.Uniques {
			if c == nil {
				continue
			}
			fmt.Fprintf(&b, "U:%s:%s|", c.Name, c.DefSQL)
		}
		for _, c := range t.Excludes {
			if c == nil {
				continue
			}
			fmt.Fprintf(&b, "X:%s:%s|", c.Name, c.DefSQL)
		}
		for _, c := range t.ForeignKeys {
			if c == nil {
				continue
			}
			fmt.Fprintf(&b, "F:%s:%s|", c.Name, c.DefSQL)
		}
	}
	if s.Indexes != nil {
		keys = keys[:0]
		for k := range s.Indexes {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			ix := s.Indexes[k]
			if ix == nil {
				continue
			}
			fmt.Fprintf(&b, "I:%s:%s|", k, ix.CreateSQL)
		}
	}
	if s.Functions != nil {
		keys = keys[:0]
		for k := range s.Functions {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			f := s.Functions[k]
			if f == nil {
				continue
			}
			fmt.Fprintf(&b, "F:%s:%s:%s|", k, f.Kind, f.DefSQL)
		}
	}
	if s.Extensions != nil {
		keys = keys[:0]
		for k := range s.Extensions {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			e := s.Extensions[k]
			if e == nil {
				continue
			}
			fmt.Fprintf(&b, "E:%s:%s:%s|", k, e.Version, e.DefSQL)
		}
	}
	for _, x := range s.ExtraDDL {
		fmt.Fprintf(&b, "XDDL:%s|", x)
	}
	if s.MiscObjects != nil {
		for _, m := range s.MiscObjects {
			if m == nil {
				continue
			}
			fmt.Fprintf(&b, "M:%s:%s|", m.Kind, m.DefSQL)
		}
	}
	if s.Policies != nil {
		keys = keys[:0]
		for k := range s.Policies {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			p := s.Policies[k]
			if p == nil {
				continue
			}
			fmt.Fprintf(&b, "P:%s:%s:%v|", k, p.DefSQL, p.Roles)
		}
	}
	if s.Views != nil {
		keys = keys[:0]
		for k := range s.Views {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			v := s.Views[k]
			if v == nil {
				continue
			}
			fmt.Fprintf(&b, "V:%s:%s|", k, v.DefSQL)
		}
	}
	if s.Sequences != nil {
		keys = keys[:0]
		for k := range s.Sequences {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			sq := s.Sequences[k]
			if sq == nil {
				continue
			}
			fmt.Fprintf(&b, "S:%s:%s|", k, sq.DefSQL)
		}
	}
	if s.Triggers != nil {
		keys = keys[:0]
		for k := range s.Triggers {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			tg := s.Triggers[k]
			if tg == nil {
				continue
			}
			fmt.Fprintf(&b, "G:%s:%s|", k, tg.DefSQL)
		}
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}
