package src

import (
	"fmt"
	"strings"

	pgq "github.com/pganalyze/pg_query_go/v6"
)

// defaultExprToSQL returns a best-effort SQL fragment for a default expression.
func defaultExprToSQL(n *pgq.Node) (string, error) {
	if n == nil {
		return "", nil
	}
	q := tryQuickExprString(n)
	if q != "NULL" {
		return q, nil
	}
	// tryQuickExprString couldn't handle this node; fall back to the full pg_query deparser.
	return deparseExprToSQL(n)
}

func tryQuickExprString(n *pgq.Node) string {
	if n == nil {
		return ""
	}
	if c := n.GetAConst(); c != nil {
		if v := c.GetIval(); v != nil {
			return fmt.Sprint(v.GetIval())
		}
		if s := c.GetSval(); s != nil {
			return formatString(s.GetSval())
		}
		if f := c.GetFval(); f != nil {
			return f.GetFval()
		}
		if b := c.GetBoolval(); b != nil {
			if b.GetBoolval() {
				return "true"
			}
			return "false"
		}
	}
	if fn := n.GetFuncCall(); fn != nil {
		var b strings.Builder
		for i, a := range fn.GetFuncname() {
			if i > 0 {
				b.WriteString(".")
			}
			if str := a.GetString_(); str != nil {
				p := str.GetSval()
				b.WriteString(p)
			}
		}
		b.WriteString("(")
		for i, arg := range fn.GetArgs() {
			if i > 0 {
				b.WriteString(", ")
			}
			if arg == nil {
				continue
			}
			// Reconstruct minimal node for const/func
			inner := &pgq.Node{}
			if ac := arg.GetAConst(); ac != nil {
				inner.Node = &pgq.Node_AConst{AConst: ac}
				b.WriteString(tryQuickExprString(inner))
				continue
			}
			if sub := arg.GetFuncCall(); sub != nil {
				inner.Node = &pgq.Node_FuncCall{FuncCall: sub}
				b.WriteString(tryQuickExprString(inner))
			}
		}
		b.WriteString(")")
		return b.String()
	}
	return "NULL"
}

func formatString(s string) string {
	return `'` + strings.ReplaceAll(s, `'`, `''`) + `'`
}
