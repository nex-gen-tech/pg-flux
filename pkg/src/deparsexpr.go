package src

import (
	"fmt"
	"strings"

	pgq "github.com/pganalyze/pg_query_go/v6"
)

// deparseExprToSQL turns a pg_query expression node into SQL (no leading SELECT).
func deparseExprToSQL(n *pgq.Node) (string, error) {
	if n == nil {
		return "", nil
	}
	sel := &pgq.SelectStmt{
		TargetList: []*pgq.Node{{
			Node: &pgq.Node_ResTarget{ResTarget: &pgq.ResTarget{Val: n}},
		}},
	}
	pr := &pgq.ParseResult{Stmts: []*pgq.RawStmt{{
		Stmt: &pgq.Node{Node: &pgq.Node_SelectStmt{SelectStmt: sel}},
	}}}
	out, err := pgq.Deparse(pr)
	if err != nil {
		return "", err
	}
	out = strings.TrimSpace(out)
	// "select only" or "select expr"
	low := strings.ToLower(out)
	if strings.HasPrefix(low, "select ") {
		return strings.TrimSpace(out[7:]), nil
	}
	return out, nil
}

// constraintToTableSQL returns the fragment that appears after CONSTRAINT name in CREATE/ALTER, for diffing.
func constraintToTableSQL(c *pgq.Constraint) (name string, sql string, err error) {
	if c == nil {
		return "", "", nil
	}
	name = strings.ToLower(strings.TrimSpace(c.GetConname()))
	switch c.GetContype() {
	case pgq.ConstrType_CONSTR_CHECK:
		expr, e := deparseExprToSQL(c.GetRawExpr())
		if e != nil {
			if c.GetCookedExpr() != "" {
				return name, "CHECK (" + c.GetCookedExpr() + ")", nil
			}
			return name, "", e
		}
		if expr == "" && c.GetCookedExpr() != "" {
			expr = c.GetCookedExpr()
		}
		return name, "CHECK (" + strings.TrimSpace(expr) + ")", nil
	case pgq.ConstrType_CONSTR_FOREIGN:
		return name, buildForeignKeySQL(c), nil
	case pgq.ConstrType_CONSTR_UNIQUE:
		return name, buildUniqueSQL(c), nil
	case pgq.ConstrType_CONSTR_EXCLUSION:
		return name, buildExclusionSQL(c), nil
	default:
		return name, "", fmt.Errorf("unsupported table constraint type %v", c.GetContype())
	}
}

func buildUniqueSQL(c *pgq.Constraint) string {
	if c == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("UNIQUE")
	if c.GetNullsNotDistinct() {
		b.WriteString(" NULLS NOT DISTINCT")
	}
	b.WriteString(" (")
	first := true
	for _, k := range c.GetKeys() {
		if k == nil {
			continue
		}
		if s := k.GetString_(); s != nil {
			if !first {
				b.WriteString(", ")
			}
			first = false
			b.WriteString(quoteIdentIfNeeded(s.GetSval()))
		}
	}
	b.WriteString(")")
	return b.String()
}

// buildExclusionSQL builds a fragment comparable to pg_get_constraintdef (EXCLUDE [USING ...] (...)).
func buildExclusionSQL(c *pgq.Constraint) string {
	if c == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("EXCLUDE")
	if am := strings.TrimSpace(c.GetAccessMethod()); am != "" {
		b.WriteString(" USING ")
		b.WriteString(am)
	}
	b.WriteString(" (")
	for i, exn := range c.GetExclusions() {
		if exn == nil {
			continue
		}
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(deparseExclusionSpec(exn))
	}
	b.WriteString(")")
	if c.GetWhereClause() != nil {
		w, err := deparseExprToSQL(c.GetWhereClause())
		if err == nil && w != "" {
			b.WriteString(" WHERE (")
			b.WriteString(w)
			b.WriteString(")")
		}
	}
	return b.String()
}

// deparseExclusionSpec turns one Exclusion spec (IndexElem + operator) into "col WITH op".
func deparseExclusionSpec(n *pgq.Node) string {
	if n == nil {
		return ""
	}
	if l := n.GetList(); l != nil {
		items := l.GetItems()
		if len(items) == 0 {
			return ""
		}
		col := ""
		if ie := items[0].GetIndexElem(); ie != nil {
			if nm := strings.TrimSpace(ie.GetName()); nm != "" {
				col = quoteIdentIfNeeded(nm)
			} else if ie.GetExpr() != nil {
				if s, err := deparseExprToSQL(ie.GetExpr()); err == nil {
					col = s
				}
			}
		}
		var op string
		if len(items) > 1 {
			op = deparseExclusionOp(items[1])
		}
		if op == "" {
			op = "="
		}
		if col == "" {
			return ""
		}
		return col + " WITH " + op
	}
	if ie := n.GetIndexElem(); ie != nil {
		col := quoteIdentIfNeeded(ie.GetName())
		if col == "" && ie.GetExpr() != nil {
			if s, err := deparseExprToSQL(ie.GetExpr()); err == nil {
				col = s
			}
		}
		if col == "" {
			return ""
		}
		return col
	}
	return ""
}

// deparseExclusionOp recreates the operator in EXCLUDE (col WITH <op>).
func deparseExclusionOp(n *pgq.Node) string {
	if n == nil {
		return ""
	}
	if l := n.GetList(); l != nil {
		var parts []string
		for _, it := range l.GetItems() {
			if it == nil {
				continue
			}
			if s := it.GetString_(); s != nil {
				parts = append(parts, s.GetSval())
			}
		}
		if len(parts) == 0 {
			return ""
		}
		return strings.Join(parts, " ")
	}
	if s := n.GetString_(); s != nil {
		return s.GetSval()
	}
	return ""
}

func buildForeignKeySQL(c *pgq.Constraint) string {
	var cols []string
	for _, k := range c.GetFkAttrs() {
		if k == nil {
			continue
		}
		if s := k.GetString_(); s != nil {
			cols = append(cols, s.GetSval())
		}
	}
	return buildForeignKeySQLWithCols(cols, c)
}

// buildInlineForeignKeySQL builds an FK SQL fragment for a column-level (inline) REFERENCES
// clause where FkAttrs is empty and the referencing column is supplied via colName.
func buildInlineForeignKeySQL(colName string, c *pgq.Constraint) string {
	return buildForeignKeySQLWithCols([]string{colName}, c)
}

func buildForeignKeySQLWithCols(cols []string, c *pgq.Constraint) string {
	var refCols []string
	for _, k := range c.GetPkAttrs() {
		if k == nil {
			continue
		}
		if s := k.GetString_(); s != nil {
			refCols = append(refCols, s.GetSval())
		}
	}
	pk := c.GetPktable()
	pkSchema := "public"
	pkTable := ""
	if pk != nil {
		if s := strings.TrimSpace(pk.GetSchemaname()); s != "" {
			pkSchema = s
		}
		pkTable = pk.GetRelname()
	}
	pkRef := quoteIdentIfNeeded(pkSchema) + "." + quoteIdentIfNeeded(pkTable)
	var b strings.Builder
	b.WriteString("FOREIGN KEY (")
	for i, col := range cols {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(quoteIdentIfNeeded(col))
	}
	b.WriteString(") REFERENCES ")
	b.WriteString(pkRef)
	if len(refCols) > 0 {
		b.WriteString(" (")
		for i, col := range refCols {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(quoteIdentIfNeeded(col))
		}
		b.WriteString(")")
	}
	if a := c.GetFkUpdAction(); a != "" {
		if up := mapFKAction(a); up != "" {
			b.WriteString(" ON UPDATE " + up)
		}
	}
	if a := c.GetFkDelAction(); a != "" {
		if d := mapFKAction(a); d != "" {
			b.WriteString(" ON DELETE " + d)
		}
	}
	return b.String()
}

// mapFKAction turns pg parsenode character codes to SQL keywords. Empty means omit (NO ACTION or default).
func mapFKAction(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	// first char of encoded action, or a full keyword from deparsed SQL
	low := strings.ToLower(s)
	if len(low) == 1 {
		switch low[0] {
		case 'a', 'e': // NO ACTION, equivalent
			return ""
		case 'r':
			return "RESTRICT"
		case 'c':
			return "CASCADE"
		case 'n':
			return "SET NULL"
		case 'd':
			return "SET DEFAULT"
		}
	}
	switch low {
	case "cascade":
		return "CASCADE"
	case "restrict":
		return "RESTRICT"
	case "set null":
		return "SET NULL"
	case "set default":
		return "SET DEFAULT"
	case "no action":
		return "NO ACTION"
	}
	if len(s) > 2 {
		return s
	}
	return ""
}
