package differ

import (
	"fmt"
	"sort"
	"strings"

	"github.com/nexg/pg-flux/pkg/plan"
	"github.com/nexg/pg-flux/pkg/schema"
)

// diffStatistics compares desired vs live pg_statistic_ext entries:
//   - new in desired → CREATE STATISTICS
//   - missing from desired → DROP STATISTICS
//   - definition changed → DROP + CREATE (PG only has ALTER STATISTICS for rename/owner/target)
func diffStatistics(d, l *schema.SchemaState) []change {
	var out []change
	if d == nil {
		d = &schema.SchemaState{}
	}
	if l == nil {
		l = &schema.SchemaState{}
	}
	for k, ds := range d.Statistics {
		if ds == nil {
			continue
		}
		ls := l.Statistics[k]
		if ls == nil {
			out = append(out, change{
				kind:   plan.ChangeRawSQL,
				rawSQL: renderCreateStatisticsSQL(ds),
				tbl:    "statistics/" + k,
			})
			continue
		}
		if !statisticsEqual(ds, ls) {
			out = append(out, change{
				kind:   plan.ChangeRawSQL,
				rawSQL: fmt.Sprintf("DROP STATISTICS IF EXISTS %s.%s", ident(ds.Schema), ident(ds.Name)),
				tbl:    "statistics/" + k,
			})
			out = append(out, change{
				kind:   plan.ChangeRawSQL,
				rawSQL: renderCreateStatisticsSQL(ds),
				tbl:    "statistics/" + k,
			})
		}
	}
	for k, ls := range l.Statistics {
		if ls == nil {
			continue
		}
		if _, ok := d.Statistics[k]; ok {
			continue
		}
		out = append(out, change{
			kind:   plan.ChangeRawSQL,
			rawSQL: fmt.Sprintf("DROP STATISTICS IF EXISTS %s.%s", ident(ls.Schema), ident(ls.Name)),
			tbl:    "statistics/" + k,
		})
	}
	return out
}

func renderCreateStatisticsSQL(s *schema.Statistics) string {
	var b strings.Builder
	b.WriteString("CREATE STATISTICS ")
	fmt.Fprintf(&b, "%s.%s", ident(s.Schema), ident(s.Name))
	if len(s.Kinds) > 0 {
		fmt.Fprintf(&b, " (%s)", strings.Join(s.Kinds, ", "))
	}
	fmt.Fprintf(&b, " ON %s FROM %s.%s",
		strings.Join(s.Columns, ", "), ident(s.TableSchema), ident(s.TableName))
	return b.String()
}

func statisticsEqual(a, b *schema.Statistics) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.TableSchema != b.TableSchema || a.TableName != b.TableName {
		return false
	}
	ak := append([]string(nil), a.Kinds...)
	bk := append([]string(nil), b.Kinds...)
	sort.Strings(ak)
	sort.Strings(bk)
	if !stringSliceEq(ak, bk) {
		return false
	}
	// Column lists are order-sensitive in PG (statistics object stores them in order),
	// so compare as-is.
	return stringSliceEq(a.Columns, b.Columns)
}

func stringSliceEq(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
