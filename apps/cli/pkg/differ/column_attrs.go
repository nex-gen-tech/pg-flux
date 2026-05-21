package differ

import (
	"fmt"
	"strings"

	"github.com/nex-gen-tech/pg-flux/pkg/plan"
	"github.com/nex-gen-tech/pg-flux/pkg/schema"
)

// diffColumnAttrs walks columns and emits cheap ALTER COLUMN statements when
// STORAGE, COMPRESSION, or COLLATE differs. These are catalog-only updates (no
// table rewrite) on existing data, with one wrinkle: changing COLLATE on a
// non-empty column requires ALTER COLUMN SET DATA TYPE, which IS a rewrite —
// we mark it with a hazard so the user sees the impact.
func diffColumnAttrs(d, l *schema.SchemaState) []change {
	var out []change
	if d == nil || l == nil {
		return out
	}
	for k, dt := range d.Tables {
		if dt == nil {
			continue
		}
		lt := l.Tables[k]
		if lt == nil {
			continue
		}
		for _, dc := range dt.Columns {
			if dc == nil {
				continue
			}
			var lc *schema.Column
			for _, c := range lt.Columns {
				if c != nil && c.Name == dc.Name {
					lc = c
					break
				}
			}
			if lc == nil {
				continue
			}
			base := fmt.Sprintf("ALTER TABLE %s.%s ALTER COLUMN %s",
				ident(dt.Schema), ident(dt.Name), ident(dc.Name))
			// STORAGE
			if dc.Storage != "" && !strings.EqualFold(dc.Storage, lc.Storage) {
				out = append(out, change{
					kind:   plan.ChangeRawSQL,
					rawSQL: fmt.Sprintf("%s SET STORAGE %s", base, strings.ToUpper(dc.Storage)),
					tbl:    schema.TableKey(dt.Schema, dt.Name) + "." + dc.Name,
				})
			}
			// COMPRESSION (PG14+)
			if dc.Compression != "" && !strings.EqualFold(dc.Compression, lc.Compression) {
				out = append(out, change{
					kind:   plan.ChangeRawSQL,
					rawSQL: fmt.Sprintf("%s SET COMPRESSION %s", base, dc.Compression),
					tbl:    schema.TableKey(dt.Schema, dt.Name) + "." + dc.Name,
				})
			}
			// COLLATE — requires SET DATA TYPE because PG doesn't support a standalone
			// ALTER COLUMN SET COLLATE; we wrap the existing type in COLLATE.
			if dc.Collation != "" && !strings.EqualFold(dc.Collation, lc.Collation) {
				out = append(out, change{
					kind: plan.ChangeRawSQL,
					rawSQL: fmt.Sprintf("%s SET DATA TYPE %s COLLATE %s",
						base, dc.TypeSQL, ident(dc.Collation)),
					tbl: schema.TableKey(dt.Schema, dt.Name) + "." + dc.Name,
				})
			}
		}
	}
	return out
}
