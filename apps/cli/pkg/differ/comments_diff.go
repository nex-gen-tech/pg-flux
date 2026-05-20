package differ

import (
	"fmt"
	"strings"

	"github.com/nexg/pg-flux/pkg/plan"
	"github.com/nexg/pg-flux/pkg/schema"
)

// diffComments walks every object kind that supports COMMENT ON and emits
// COMMENT ON ... IS '...' (or IS NULL to clear) when desired vs live differ.
// Comments are non-blocking metadata so all emitted statements are advisory.
func diffComments(d, l *schema.SchemaState) []change {
	var out []change
	if d == nil {
		d = &schema.SchemaState{}
	}
	if l == nil {
		l = &schema.SchemaState{}
	}
	// Tables + columns
	for k, dt := range d.Tables {
		if dt == nil {
			continue
		}
		lt := l.Tables[k]
		// Table-level comment
		if normComment(commentOf(dt)) != normComment(commentOf(lt)) {
			out = append(out, commentChange(
				fmt.Sprintf("COMMENT ON TABLE %s.%s IS %s", ident(dt.Schema), ident(dt.Name), commentLiteral(dt.Comment)),
				schema.TableKey(dt.Schema, dt.Name),
			))
		}
		// Column-level comments
		for _, dc := range dt.Columns {
			if dc == nil {
				continue
			}
			var lc *schema.Column
			if lt != nil {
				for _, c := range lt.Columns {
					if c != nil && c.Name == dc.Name {
						lc = c
						break
					}
				}
			}
			if normComment(commentOfCol(dc)) != normComment(commentOfCol(lc)) {
				out = append(out, commentChange(
					fmt.Sprintf("COMMENT ON COLUMN %s.%s.%s IS %s",
						ident(dt.Schema), ident(dt.Name), ident(dc.Name), commentLiteral(dc.Comment)),
					schema.TableKey(dt.Schema, dt.Name)+"."+dc.Name,
				))
			}
		}
	}
	// Views (including materialized)
	for k, dv := range d.Views {
		if dv == nil {
			continue
		}
		lv := l.Views[k]
		var dDesc, lDesc string
		if dv != nil {
			dDesc = dv.Comment
		}
		if lv != nil {
			lDesc = lv.Comment
		}
		if normComment(dDesc) != normComment(lDesc) {
			kw := "VIEW"
			if dv.Materialized {
				kw = "MATERIALIZED VIEW"
			}
			out = append(out, commentChange(
				fmt.Sprintf("COMMENT ON %s %s.%s IS %s", kw, ident(dv.Schema), ident(dv.Name), commentLiteral(dv.Comment)),
				schema.ViewKey(dv.Schema, dv.Name),
			))
		}
	}
	// Sequences
	for k, ds := range d.Sequences {
		if ds == nil {
			continue
		}
		ls := l.Sequences[k]
		var dDesc, lDesc string
		if ds != nil {
			dDesc = ds.Comment
		}
		if ls != nil {
			lDesc = ls.Comment
		}
		if normComment(dDesc) != normComment(lDesc) {
			out = append(out, commentChange(
				fmt.Sprintf("COMMENT ON SEQUENCE %s.%s IS %s", ident(ds.Schema), ident(ds.Name), commentLiteral(ds.Comment)),
				schema.SeqKey(ds.Schema, ds.Name),
			))
		}
	}
	// Indexes — COMMENT ON INDEX must run AFTER the index exists. Indexes are
	// created CONCURRENTLY (outside the main transaction), so we mark these as
	// rawConcurrent so they land in the post-COMMIT section.
	for k, di := range d.Indexes {
		if di == nil {
			continue
		}
		li := l.Indexes[k]
		var dDesc, lDesc string
		if di != nil {
			dDesc = di.Comment
		}
		if li != nil {
			lDesc = li.Comment
		}
		if normComment(dDesc) != normComment(lDesc) {
			c := commentChange(
				fmt.Sprintf("COMMENT ON INDEX %s.%s IS %s", ident(di.Schema), ident(di.Name), commentLiteral(di.Comment)),
				schema.IndexKey(di.Schema, di.Name),
			)
			c.rawConcurrent = true
			out = append(out, c)
		}
	}
	// Functions: identity already encodes schema/name/args
	for k, df := range d.Functions {
		if df == nil {
			continue
		}
		lf := l.Functions[k]
		var dDesc, lDesc string
		if df != nil {
			dDesc = df.Comment
		}
		if lf != nil {
			lDesc = lf.Comment
		}
		if normComment(dDesc) != normComment(lDesc) {
			kw := "FUNCTION"
			switch df.Kind {
			case "a":
				kw = "AGGREGATE"
			case "p":
				kw = "PROCEDURE"
			}
			out = append(out, commentChange(
				fmt.Sprintf("COMMENT ON %s %s IS %s", kw, df.Identity, commentLiteral(df.Comment)),
				df.Identity,
			))
		}
	}
	// Triggers and policies
	for k, dt := range d.Triggers {
		if dt == nil {
			continue
		}
		lt := l.Triggers[k]
		var dDesc, lDesc string
		if dt != nil {
			dDesc = dt.Comment
		}
		if lt != nil {
			lDesc = lt.Comment
		}
		if normComment(dDesc) != normComment(lDesc) {
			out = append(out, commentChange(
				fmt.Sprintf("COMMENT ON TRIGGER %s ON %s.%s IS %s",
					ident(dt.Name), ident(dt.Schema), ident(dt.Table), commentLiteral(dt.Comment)),
				schema.TriggerKey(dt.Schema, dt.Table, dt.Name),
			))
		}
	}
	for k, dp := range d.Policies {
		if dp == nil {
			continue
		}
		lp := l.Policies[k]
		var dDesc, lDesc string
		if dp != nil {
			dDesc = dp.Comment
		}
		if lp != nil {
			lDesc = lp.Comment
		}
		if normComment(dDesc) != normComment(lDesc) {
			out = append(out, commentChange(
				fmt.Sprintf("COMMENT ON POLICY %s ON %s.%s IS %s",
					ident(dp.Name), ident(dp.Schema), ident(dp.Table), commentLiteral(dp.Comment)),
				schema.PolicyKey(dp.Schema, dp.Table, dp.Name),
			))
		}
	}
	return out
}

func commentOf(t *schema.Table) string {
	if t == nil {
		return ""
	}
	return t.Comment
}

func commentOfCol(c *schema.Column) string {
	if c == nil {
		return ""
	}
	return c.Comment
}

// normComment canonicalizes a comment for equality comparison — trims surrounding
// whitespace but preserves internal content (including embedded newlines).
func normComment(s string) string {
	return strings.TrimSpace(s)
}

// commentLiteral renders the SQL literal form of a comment payload. An empty
// payload becomes NULL (which clears the comment in PG); non-empty values are
// single-quoted with embedded quotes doubled.
func commentLiteral(s string) string {
	if strings.TrimSpace(s) == "" {
		return "NULL"
	}
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// commentChange wraps a COMMENT ON … statement as a RAW_DDL change so it
// flows through the existing emit pipeline.
func commentChange(ddl, object string) change {
	return change{
		kind:   plan.ChangeRawSQL,
		rawSQL: ddl,
		sch:    "",
		tbl:    object,
	}
}
