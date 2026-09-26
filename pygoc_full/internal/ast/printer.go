package ast

import (
	"fmt"
	"io"
	"strings"
)

// Print writes an indented, human-readable tree representation of prog
// to w. This exists purely as a debugging/inspection tool — it's what
// the `pygoc ast <file>` CLI command will call — and has no effect on
// compilation itself.
func Print(w io.Writer, prog *Program) {
	p := &printer{w: w}
	for _, s := range prog.Statements {
		p.stmt(s, 0)
	}
}

// Sprint is a convenience wrapper for tests and quick debugging, where
// building an io.Writer would just be extra ceremony.
func Sprint(prog *Program) string {
	var sb strings.Builder
	Print(&sb, prog)
	return sb.String()
}

type printer struct {
	w io.Writer
}

func (p *printer) line(depth int, format string, args ...any) {
	fmt.Fprintf(p.w, "%s%s\n", strings.Repeat("  ", depth), fmt.Sprintf(format, args...))
}

// ---- statements -------------------------------------------------

func (p *printer) stmtList(stmts []Stmt, depth int) {
	for _, s := range stmts {
		p.stmt(s, depth)
	}
}

func (p *printer) stmt(s Stmt, depth int) {
	switch n := s.(type) {

	case *ExprStmt:
		p.line(depth, "ExprStmt")
		p.expr(n.X, depth+1)

	case *AssignStmt:
		p.line(depth, "AssignStmt op=%s", n.Op)
		p.line(depth+1, "Targets:")
		for _, t := range n.Targets {
			p.expr(t, depth+2)
		}
		p.line(depth+1, "Value:")
		p.expr(n.Value, depth+2)

	case *ReturnStmt:
		p.line(depth, "ReturnStmt")
		for _, v := range n.Values {
			p.expr(v, depth+1)
		}

	case *BreakStmt:
		p.line(depth, "BreakStmt")

	case *ContinueStmt:
		p.line(depth, "ContinueStmt")

	case *IfStmt:
		p.line(depth, "IfStmt")
		p.line(depth+1, "Cond:")
		p.expr(n.Cond, depth+2)
		p.line(depth+1, "Then:")
		p.stmtList(n.Then, depth+2)
		for _, elif := range n.Elifs {
			p.line(depth+1, "Elif:")
			p.expr(elif.Cond, depth+2)
			p.stmtList(elif.Body, depth+2)
		}
		if n.Else != nil {
			p.line(depth+1, "Else:")
			p.stmtList(n.Else, depth+2)
		}

	case *WhileStmt:
		p.line(depth, "WhileStmt")
		p.line(depth+1, "Cond:")
		p.expr(n.Cond, depth+2)
		p.line(depth+1, "Body:")
		p.stmtList(n.Body, depth+2)

	case *ForStmt:
		p.line(depth, "ForStmt var=%s", n.VarName)
		p.line(depth+1, "Start:")
		p.expr(n.Start, depth+2)
		if n.Stop != nil {
			p.line(depth+1, "Stop:")
			p.expr(n.Stop, depth+2)
		}
		p.line(depth+1, "Body:")
		p.stmtList(n.Body, depth+2)

	case *FuncDecl:
		p.line(depth, "FuncDecl name=%s", n.Name)
		p.line(depth+1, "Params:")
		for _, param := range n.Params {
			p.line(depth+2, "%s: %s", param.Name, typeExprString(param.Type))
		}
		if len(n.ReturnTypes) > 0 {
			names := make([]string, len(n.ReturnTypes))
			for i, rt := range n.ReturnTypes {
				names[i] = typeExprString(rt)
			}
			p.line(depth+1, "Returns: %s", strings.Join(names, ", "))
		}
		p.line(depth+1, "Body:")
		p.stmtList(n.Body, depth+2)

	default:
		p.line(depth, "<unknown stmt %T>", n)
	}
}

// ---- expressions -------------------------------------------------

func (p *printer) expr(e Expr, depth int) {
	switch n := e.(type) {

	case *Identifier:
		p.line(depth, "Identifier(%s)", n.Name)

	case *IntLiteral:
		p.line(depth, "IntLiteral(%d)", n.Value)

	case *FloatLiteral:
		p.line(depth, "FloatLiteral(%g)", n.Value)

	case *StringLiteral:
		p.line(depth, "StringLiteral(%q)", n.Value)

	case *BoolLiteral:
		p.line(depth, "BoolLiteral(%t)", n.Value)

	case *ListLiteral:
		p.line(depth, "ListLiteral")
		for _, el := range n.Elements {
			p.expr(el, depth+1)
		}

	case *DictLiteral:
		p.line(depth, "DictLiteral")
		for i := range n.Keys {
			p.line(depth+1, "entry:")
			p.expr(n.Keys[i], depth+2)
			p.expr(n.Values[i], depth+2)
		}

	case *TupleLiteral:
		p.line(depth, "TupleLiteral")
		for _, el := range n.Elements {
			p.expr(el, depth+1)
		}

	case *UnaryExpr:
		p.line(depth, "UnaryExpr op=%s", n.Op)
		p.expr(n.Operand, depth+1)

	case *BinaryExpr:
		p.line(depth, "BinaryExpr op=%s", n.Op)
		p.expr(n.Left, depth+1)
		p.expr(n.Right, depth+1)

	case *TernaryExpr:
		p.line(depth, "TernaryExpr")
		p.line(depth+1, "Cond:")
		p.expr(n.Cond, depth+2)
		p.line(depth+1, "Then:")
		p.expr(n.Then, depth+2)
		p.line(depth+1, "Else:")
		p.expr(n.Else, depth+2)

	case *CallExpr:
		p.line(depth, "CallExpr")
		p.line(depth+1, "Callee:")
		p.expr(n.Callee, depth+2)
		p.line(depth+1, "Args:")
		for _, a := range n.Args {
			p.expr(a, depth+2)
		}

	case *IndexExpr:
		p.line(depth, "IndexExpr")
		p.expr(n.Object, depth+1)
		p.expr(n.Index, depth+1)

	case *SliceExpr:
		p.line(depth, "SliceExpr")
		p.expr(n.Object, depth+1)
		if n.Low != nil {
			p.line(depth+1, "Low:")
			p.expr(n.Low, depth+2)
		}
		if n.High != nil {
			p.line(depth+1, "High:")
			p.expr(n.High, depth+2)
		}

	case *AttributeExpr:
		p.line(depth, "AttributeExpr .%s", n.Attr)
		p.expr(n.Object, depth+1)

	default:
		p.line(depth, "<unknown expr %T>", n)
	}
}

// typeExprString renders a TypeExpr back to PyGo-ish syntax, e.g.
// list[int], dict[string, int] — used only for debug printing.
func typeExprString(t *TypeExpr) string {
	if t == nil {
		return "?"
	}
	if len(t.Params) == 0 {
		return t.Name
	}
	parts := make([]string, len(t.Params))
	for i, p := range t.Params {
		parts[i] = typeExprString(p)
	}
	return fmt.Sprintf("%s[%s]", t.Name, strings.Join(parts, ", "))
}
