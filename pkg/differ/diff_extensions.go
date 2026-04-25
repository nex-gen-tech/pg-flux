package differ

import (
	"strings"

	"github.com/nexg/pg-flux/pkg/plan"
	"github.com/nexg/pg-flux/pkg/schema"
)

func ensureExtMaps(s *schema.SchemaState) {
	if s == nil {
		return
	}
	if s.Extensions == nil {
		s.Extensions = make(map[string]*schema.Extension)
	}
}

func diffExtensions(d, l *schema.SchemaState) []change {
	if d == nil || d.Extensions == nil {
		// Desired schema does not declare extensions; never auto-DROP from live.
		return nil
	}
	var out []change
	ensureExtMaps(d)
	ensureExtMaps(l)
	for k, de := range d.Extensions {
		if de == nil {
			continue
		}
		le := l.Extensions[k]
		if le == nil {
			out = append(out, change{kind: plan.ChangeCreateExtension, ext: de})
			continue
		}
		// Version / upgrade path: prefer ALTER EXTENSION ... UPDATE TO over DROP+CREATE.
		if strings.TrimSpace(de.Version) != "" && strings.TrimSpace(le.Version) != "" && de.Version != le.Version {
			out = append(out, change{kind: plan.ChangeUpdateExtension, ext: de, extLiveVer: le.Version})
			continue
		}
		if strings.TrimSpace(de.DefSQL) != "" && strings.TrimSpace(le.DefSQL) != "" {
			if fpGenericSQL(de.DefSQL) != fpGenericSQL(le.DefSQL) {
				out = append(out, change{kind: plan.ChangeDropExtension, dropExt: k, ext: le})
				out = append(out, change{kind: plan.ChangeCreateExtension, ext: de})
			}
		}
	}
	for k, le := range l.Extensions {
		if le == nil {
			continue
		}
		if d.Extensions[k] == nil {
			out = append(out, change{kind: plan.ChangeDropExtension, dropExt: k, ext: le})
		}
	}
	return out
}

func diffExtraDDL(d *schema.SchemaState) []change {
	if d == nil || len(d.ExtraDDL) == 0 {
		return nil
	}
	var out []change
	for _, sql := range d.ExtraDDL {
		s := strings.TrimSpace(sql)
		if s == "" {
			continue
		}
		out = append(out, change{kind: plan.ChangeRawSQL, rawSQL: s})
	}
	return out
}
