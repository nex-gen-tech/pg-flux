package src

import (
	"fmt"
	"strings"

	pgq "github.com/pganalyze/pg_query_go/v6"
)

// typeNameToSQL renders a best-effort SQL type from pg_query TypeName.
func typeNameToSQL(tn *pgq.TypeName) (string, error) {
	if tn == nil {
		return "", nil
	}
	var b strings.Builder
	if tn.GetSetof() {
		b.WriteString("SETOF ")
	}
	// names: schema . type
	for i, n := range tn.GetNames() {
		if i > 0 {
			b.WriteString(".")
		}
		s, err := nodeStringLiteral(n)
		if err != nil {
			return "", err
		}
		b.WriteString(s)
	}
	if len(tn.GetTypmods()) > 0 {
		b.WriteString("(")
		for i, tm := range tn.GetTypmods() {
			if i > 0 {
				b.WriteString(", ")
			}
			if tm == nil {
				continue
			}
			// A_Const for integers etc.
			if a := tm.GetAConst(); a != nil {
				if v := a.GetIval(); v != nil {
					fmt.Fprintf(&b, "%d", v.GetIval())
					continue
				}
				if s := a.GetSval(); s != nil {
					b.WriteString(quoteString(s.GetSval()))
				}
			}
		}
		b.WriteString(")")
	}
	if len(tn.GetArrayBounds()) > 0 {
		b.WriteString("[]")
	}
	return b.String(), nil
}

func nodeStringLiteral(n *pgq.Node) (string, error) {
	if n == nil {
		return "", nil
	}
	switch t := n.GetNode().(type) {
	case *pgq.Node_String_:
		if t.String_ == nil {
			return "", nil
		}
		return quoteIdentIfNeeded(t.String_.GetSval()), nil
	default:
		return "", fmt.Errorf("type name: unexpected node %T", t)
	}
}

func quoteIdentIfNeeded(name string) string {
	if name == "" {
		return `""`
	}
	// Already quoted in AST?
	if strings.ContainsAny(name, `"'`) {
		return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
	}
	needQuote := false
	for _, c := range name {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_') {
			needQuote = true
			break
		}
	}
	if needQuote {
		return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
	}
	return name
}

func quoteString(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func defaultToSQL(d *pgq.Node) (string, error) {
	if d == nil {
		return "", nil
	}
	pr := &pgq.ParseResult{Stmts: []*pgq.RawStmt{{Stmt: d}}}
	return pgq.Deparse(pr)
}
