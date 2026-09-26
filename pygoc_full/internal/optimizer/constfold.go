// Package optimizer implements Phase 5: five independently visible,
// independently testable passes over the IR. Every pass has the same
// shape — func(fn *ir.Function) *ir.Function — and returns a FRESH
// Function built from scratch rather than mutating its input. That
// design choice is what makes the before/after snapshots in
// pipeline.go trivial: nothing needs a separate Clone() step, because
// nothing is ever mutated in place.
package optimizer

import (
	"math"

	"github.com/manikyarathore/pygoc/internal/ir"
	"github.com/manikyarathore/pygoc/internal/types"
)

// ConstantFold replaces any BinOp/UnOp instruction whose operands are
// ALL compile-time constants with a plain Assign of the computed
// result, e.g. `t1 = 2 + 3` becomes `t1 = 5`.
func ConstantFold(fn *ir.Function) *ir.Function {
	return transformFunction(fn, func(instr ir.Instr) ir.Instr {
		switch in := instr.(type) {
		case ir.BinOp:
			if in.Left.IsConst && in.Right.IsConst {
				if v, ok := foldBinary(in.Op, in.Left, in.Right, in.Type); ok {
					return ir.Assign{Dst: in.Dst, Src: ir.ConstOperand(v, in.Type), Type: in.Type}
				}
			}
		case ir.UnOp:
			if in.X.IsConst {
				if v, ok := foldUnary(in.Op, in.X, in.Type); ok {
					return ir.Assign{Dst: in.Dst, Src: ir.ConstOperand(v, in.Type), Type: in.Type}
				}
			}
		}
		return instr
	})
}

func toInt64(v any) int64 {
	switch x := v.(type) {
	case int64:
		return x
	case float64:
		return int64(x)
	}
	return 0
}

func toFloat64(v any) float64 {
	switch x := v.(type) {
	case int64:
		return float64(x)
	case float64:
		return x
	}
	return 0
}

func foldBinary(op string, l, r ir.Operand, resultType types.Type) (any, bool) {
	switch resultType.Kind {
	case types.Int:
		a, b := toInt64(l.Const), toInt64(r.Const)
		switch op {
		case "+":
			return a + b, true
		case "-":
			return a - b, true
		case "*":
			return a * b, true
		case "//":
			if b == 0 {
				return nil, false // never fold a compile-time division by zero; let it surface at runtime
			}
			return int64(math.Floor(float64(a) / float64(b))), true
		case "%":
			if b == 0 {
				return nil, false
			}
			return a % b, true
		}
	case types.Float:
		a, b := toFloat64(l.Const), toFloat64(r.Const)
		switch op {
		case "+":
			return a + b, true
		case "-":
			return a - b, true
		case "*":
			return a * b, true
		case "/":
			if b == 0 {
				return nil, false
			}
			return a / b, true
		case "//":
			if b == 0 {
				return nil, false
			}
			return math.Floor(a / b), true
		}
	case types.Bool:
		switch op {
		case "==", "!=", "<", ">", "<=", ">=":
			return compareConsts(op, l, r), true
		case "and":
			lb, lok := l.Const.(bool)
			rb, rok := r.Const.(bool)
			if lok && rok {
				return lb && rb, true
			}
		case "or":
			lb, lok := l.Const.(bool)
			rb, rok := r.Const.(bool)
			if lok && rok {
				return lb || rb, true
			}
		}
	case types.String:
		if op == "+" {
			ls, lok := l.Const.(string)
			rs, rok := r.Const.(string)
			if lok && rok {
				return ls + rs, true
			}
		}
	}
	return nil, false
}

func compareConsts(op string, l, r ir.Operand) bool {
	if l.Type.Kind == types.String {
		ls, _ := l.Const.(string)
		rs, _ := r.Const.(string)
		switch op {
		case "==":
			return ls == rs
		case "!=":
			return ls != rs
		case "<":
			return ls < rs
		case ">":
			return ls > rs
		case "<=":
			return ls <= rs
		case ">=":
			return ls >= rs
		}
		return false
	}
	lf, rf := toFloat64(l.Const), toFloat64(r.Const)
	switch op {
	case "==":
		return lf == rf
	case "!=":
		return lf != rf
	case "<":
		return lf < rf
	case ">":
		return lf > rf
	case "<=":
		return lf <= rf
	case ">=":
		return lf >= rf
	}
	return false
}

func foldUnary(op string, x ir.Operand, resultType types.Type) (any, bool) {
	switch op {
	case "-":
		if resultType.Kind == types.Int {
			return -toInt64(x.Const), true
		}
		return -toFloat64(x.Const), true
	case "not ":
		b, ok := x.Const.(bool)
		if ok {
			return !b, true
		}
	}
	return nil, false
}
