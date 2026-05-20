package differ

import (
	"fmt"
	"strings"

	"github.com/nexg/pg-flux/pkg/plan"
	"github.com/nexg/pg-flux/pkg/schema"
)

// diffOwners emits ALTER ... OWNER TO statements when the desired Owner differs from
// the live Owner across each object kind. Owners are only diffed when both sides have
// a non-empty Owner — that way unit tests / sources that don't specify ownership don't
// trigger spurious churn.
func diffOwners(d, l *schema.SchemaState) []change {
	var out []change
	if d == nil {
		d = &schema.SchemaState{}
	}
	if l == nil {
		l = &schema.SchemaState{}
	}
	for k, dt := range d.Tables {
		if dt == nil {
			continue
		}
		lt := l.Tables[k]
		if dt.Owner != "" && lt != nil && lt.Owner != "" && !ownerEqual(dt.Owner, lt.Owner) {
			out = append(out, ownerChange(
				fmt.Sprintf("ALTER TABLE %s.%s OWNER TO %s", ident(dt.Schema), ident(dt.Name), ident(dt.Owner)),
				schema.TableKey(dt.Schema, dt.Name),
			))
		}
	}
	for k, dv := range d.Views {
		if dv == nil {
			continue
		}
		lv := l.Views[k]
		if dv.Owner != "" && lv != nil && lv.Owner != "" && !ownerEqual(dv.Owner, lv.Owner) {
			kw := "VIEW"
			if dv.Materialized {
				kw = "MATERIALIZED VIEW"
			}
			out = append(out, ownerChange(
				fmt.Sprintf("ALTER %s %s.%s OWNER TO %s", kw, ident(dv.Schema), ident(dv.Name), ident(dv.Owner)),
				schema.ViewKey(dv.Schema, dv.Name),
			))
		}
	}
	for k, ds := range d.Sequences {
		if ds == nil {
			continue
		}
		ls := l.Sequences[k]
		if ds.Owner != "" && ls != nil && ls.Owner != "" && !ownerEqual(ds.Owner, ls.Owner) {
			out = append(out, ownerChange(
				fmt.Sprintf("ALTER SEQUENCE %s.%s OWNER TO %s", ident(ds.Schema), ident(ds.Name), ident(ds.Owner)),
				schema.SeqKey(ds.Schema, ds.Name),
			))
		}
	}
	for k, df := range d.Functions {
		if df == nil {
			continue
		}
		lf := l.Functions[k]
		if df.Owner != "" && lf != nil && lf.Owner != "" && !ownerEqual(df.Owner, lf.Owner) {
			kw := "FUNCTION"
			switch df.Kind {
			case "a":
				kw = "AGGREGATE"
			case "p":
				kw = "PROCEDURE"
			}
			out = append(out, ownerChange(
				fmt.Sprintf("ALTER %s %s OWNER TO %s", kw, df.Identity, ident(df.Owner)),
				df.Identity,
			))
		}
	}
	// Domains (CREATE DOMAIN), composite types, range types, foreign tables, foreign servers.
	// Each kind uses ALTER <kind> ... OWNER TO ... and is only emitted when both sides have
	// a non-empty Owner (avoids spurious diff in unit tests / partial source files).
	for k, dd := range d.Domains {
		if dd == nil {
			continue
		}
		ld := l.Domains[k]
		if dd.Owner != "" && ld != nil && ld.Owner != "" && !ownerEqual(dd.Owner, ld.Owner) {
			out = append(out, ownerChange(
				fmt.Sprintf("ALTER DOMAIN %s.%s OWNER TO %s", ident(dd.Schema), ident(dd.Name), ident(dd.Owner)),
				"DOMAIN "+k,
			))
		}
	}
	for k, dc := range d.CompositeTypes {
		if dc == nil {
			continue
		}
		lc := l.CompositeTypes[k]
		if dc.Owner != "" && lc != nil && lc.Owner != "" && !ownerEqual(dc.Owner, lc.Owner) {
			out = append(out, ownerChange(
				fmt.Sprintf("ALTER TYPE %s.%s OWNER TO %s", ident(dc.Schema), ident(dc.Name), ident(dc.Owner)),
				"TYPE "+k,
			))
		}
	}
	for k, dr := range d.RangeTypes {
		if dr == nil {
			continue
		}
		lr := l.RangeTypes[k]
		if dr.Owner != "" && lr != nil && lr.Owner != "" && !ownerEqual(dr.Owner, lr.Owner) {
			out = append(out, ownerChange(
				fmt.Sprintf("ALTER TYPE %s.%s OWNER TO %s", ident(dr.Schema), ident(dr.Name), ident(dr.Owner)),
				"TYPE "+k,
			))
		}
	}
	for k, dft := range d.ForeignTables {
		if dft == nil {
			continue
		}
		lft := l.ForeignTables[k]
		if dft.Owner != "" && lft != nil && lft.Owner != "" && !ownerEqual(dft.Owner, lft.Owner) {
			out = append(out, ownerChange(
				fmt.Sprintf("ALTER FOREIGN TABLE %s.%s OWNER TO %s", ident(dft.Schema), ident(dft.Name), ident(dft.Owner)),
				"FOREIGN TABLE "+k,
			))
		}
	}
	for k, dfs := range d.ForeignServers {
		if dfs == nil {
			continue
		}
		lfs := l.ForeignServers[k]
		if dfs.Owner != "" && lfs != nil && lfs.Owner != "" && !ownerEqual(dfs.Owner, lfs.Owner) {
			out = append(out, ownerChange(
				fmt.Sprintf("ALTER SERVER %s OWNER TO %s", ident(dfs.Name), ident(dfs.Owner)),
				"SERVER "+k,
			))
		}
	}
	return out
}

// ownerEqual compares role names case-insensitively (PostgreSQL is case-folded for
// unquoted identifiers).
func ownerEqual(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}

func ownerChange(ddl, object string) change {
	return change{
		kind:   plan.ChangeRawSQL,
		rawSQL: ddl,
		sch:    "",
		tbl:    object,
	}
}
