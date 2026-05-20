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
		// If ANY argument is something we can't quickly render, bail out and let
		// the full deparser handle the entire expression. Silently dropping an
		// argument produces buggy output like nextval() with empty parens —
		// previously a real round-trip failure on serial-style defaults.
		for i, arg := range fn.GetArgs() {
			if i > 0 {
				b.WriteString(", ")
			}
			if arg == nil {
				return "NULL" // bail to deparser
			}
			inner := &pgq.Node{}
			switch {
			case arg.GetAConst() != nil:
				inner.Node = &pgq.Node_AConst{AConst: arg.GetAConst()}
				b.WriteString(tryQuickExprString(inner))
			case arg.GetFuncCall() != nil:
				inner.Node = &pgq.Node_FuncCall{FuncCall: arg.GetFuncCall()}
				b.WriteString(tryQuickExprString(inner))
			default:
				// TypeCast, ColumnRef, etc. — let the full deparser handle it.
				return "NULL"
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
