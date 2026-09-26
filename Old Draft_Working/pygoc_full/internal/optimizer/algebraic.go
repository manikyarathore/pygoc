package optimizer

import (
	"github.com/manikyarathore/pygoc/internal/ir"
	"github.com/manikyarathore/pygoc/internal/types"
)

// AlgebraicSimplify rewrites BinOp instructions matching a known
// identity — x*1, 1*x, x+0, 0+x, x-0, x*0, 0*x — into a plain Assign,
// eliminating the arithmetic entirely.
func AlgebraicSimplify(fn *ir.Function) *ir.Function {
	return transformFunction(fn, func(instr ir.Instr) ir.Instr {
		in, ok := instr.(ir.BinOp)
		if !ok {
			return instr
		}
		switch {
		case in.Op == "*" && isOne(in.Right):
			return ir.Assign{Dst: in.Dst, Src: in.Left, Type: in.Type}
		case in.Op == "*" && isOne(in.Left):
			return ir.Assign{Dst: in.Dst, Src: in.Right, Type: in.Type}
		case in.Op == "+" && isZero(in.Right):
			return ir.Assign{Dst: in.Dst, Src: in.Left, Type: in.Type}
		case in.Op == "+" && isZero(in.Left):
			return ir.Assign{Dst: in.Dst, Src: in.Right, Type: in.Type}
		case in.Op == "-" && isZero(in.Right):
			return ir.Assign{Dst: in.Dst, Src: in.Left, Type: in.Type}
		case in.Op == "*" && (isZero(in.Right) || isZero(in.Left)):
			zero := any(int64(0))
			if in.Type.Kind == types.Float {
				zero = float64(0)
			}
			return ir.Assign{Dst: in.Dst, Src: ir.ConstOperand(zero, in.Type), Type: in.Type}
		}
		return instr
	})
}

func isZero(op ir.Operand) bool {
	if !op.IsConst {
		return false
	}
	return toFloat64(op.Const) == 0
}

func isOne(op ir.Operand) bool {
	if !op.IsConst {
		return false
	}
	return toFloat64(op.Const) == 1
}
