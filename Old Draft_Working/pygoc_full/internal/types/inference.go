package types

import (
	"fmt"

	"github.com/manikyarathore/pygoc/internal/ast"
	"github.com/manikyarathore/pygoc/internal/symbol"
)

// Infer computes the type of an expression, resolving identifiers
// through tbl. This is deliberately local/statement-by-statement
// inference (not full Hindley-Milner) — PyGo doesn't need polymorphism,
// so every expression's type is determined directly from its own shape
// and its subexpressions' already-known types.
//
// SUBMISSION SCOPE NOTE: only the core expression forms are supported —
// identifiers, int/float/bool/string literals, unary/binary operators,
// and calls to `print` or a user-defined function. List/dict/tuple
// literals, ternary expressions, indexing, slicing, and attribute
// access are explicitly out of scope for this build's codegen target
// and return a clear error here rather than being silently mishandled
// later in the pipeline.
func Infer(e ast.Expr, tbl *symbol.Table) (Type, error) {
	switch n := e.(type) {

	case *ast.IntLiteral:
		return IntT, nil
	case *ast.FloatLiteral:
		return FloatT, nil
	case *ast.BoolLiteral:
		return BoolT, nil
	case *ast.StringLiteral:
		return StringT, nil

	case *ast.Identifier:
		sym, ok := tbl.Resolve(n.Name)
		if !ok {
			return UnknownT, &TypeError{Msg: fmt.Sprintf("undefined variable '%s'", n.Name)}
		}
		t, ok := sym.Type.(Type)
		if !ok {
			return UnknownT, &TypeError{Msg: fmt.Sprintf("'%s' has no resolved type yet", n.Name)}
		}
		return t, nil

	case *ast.UnaryExpr:
		operand, err := Infer(n.Operand, tbl)
		if err != nil {
			return UnknownT, err
		}
		return UnaryOpResult(n.Op, operand)

	case *ast.BinaryExpr:
		left, err := Infer(n.Left, tbl)
		if err != nil {
			return UnknownT, err
		}
		right, err := Infer(n.Right, tbl)
		if err != nil {
			return UnknownT, err
		}
		return BinaryOpResult(n.Op, left, right)

	case *ast.CallExpr:
		callee, ok := n.Callee.(*ast.Identifier)
		if !ok {
			return UnknownT, &TypeError{Msg: "method calls are not supported in this build"}
		}
		// type-check the arguments even though print/void calls don't
		// use the result, so a bad argument still surfaces as an error
		for _, arg := range n.Args {
			if _, err := Infer(arg, tbl); err != nil {
				return UnknownT, err
			}
		}
		if callee.Name == "print" {
			return VoidT, nil
		}
		sym, ok := tbl.Resolve(callee.Name)
		if !ok {
			return UnknownT, &TypeError{Msg: fmt.Sprintf("undefined function '%s'", callee.Name)}
		}
		if sym.Kind != symbol.FuncSymbol {
			return UnknownT, &TypeError{Msg: fmt.Sprintf("'%s' is not a function", callee.Name)}
		}
		t, _ := sym.Type.(Type) // defaults to UnknownT{Kind:Unknown} zero value if unset
		return t, nil

	default:
		return UnknownT, &TypeError{Msg: fmt.Sprintf("%T is not supported in this build (list/dict/tuple/ternary/indexing/attribute access are out of scope)", e)}
	}
}
