package optimizer

import "github.com/manikyarathore/pygoc/internal/ir"

// ConstantPropagate carries known-constant variable values forward
// through the instruction stream, replacing later references to a
// variable with its known constant value.
//
// CORRECTNESS-CRITICAL DESIGN NOTE: the "known constants" map is reset
// at the start of EVERY BLOCK. This is deliberate and was fixed after
// a real bug: an earlier version carried one map across the whole
// function in block-STORAGE order. Since a loop's body/increment
// blocks are stored textually AFTER the entry block but actually
// execute repeatedly, that version treated a loop variable (e.g. a
// `for` loop's `i`, initialized to 0 in the entry block) as eternally
// constant-0 even inside the loop body — silently corrupting
// `total += i` into `total = 0` on every iteration. Basic blocks have
// no internal branches by definition, so a linear map that's valid for
// exactly one block, and only one block, is always sound — it never
// carries an assumption across a control-flow edge (including a loop
// back-edge) that might not actually hold.
func ConstantPropagate(fn *ir.Function) *ir.Function {
	newFn := &ir.Function{Name: fn.Name, Params: append([]ir.Param{}, fn.Params...), ReturnType: fn.ReturnType}
	for _, blk := range fn.Blocks {
		known := map[string]ir.Operand{} // reset every block — see note above
		subst := func(op ir.Operand) ir.Operand {
			if !op.IsConst && op.Name != "" {
				if c, ok := known[op.Name]; ok {
					return c
				}
			}
			return op
		}

		nb := &ir.Block{Label: blk.Label}
		for _, instr := range blk.Instrs {
			switch in := instr.(type) {
			case ir.Assign:
				newSrc := subst(in.Src)
				if newSrc.IsConst {
					known[in.Dst] = newSrc
				} else {
					delete(known, in.Dst)
				}
				nb.Instrs = append(nb.Instrs, ir.Assign{Dst: in.Dst, Src: newSrc, Type: in.Type})

			case ir.BinOp:
				nl, nr := subst(in.Left), subst(in.Right)
				delete(known, in.Dst)
				nb.Instrs = append(nb.Instrs, ir.BinOp{Dst: in.Dst, Op: in.Op, Left: nl, Right: nr, Type: in.Type})

			case ir.UnOp:
				nx := subst(in.X)
				delete(known, in.Dst)
				nb.Instrs = append(nb.Instrs, ir.UnOp{Dst: in.Dst, Op: in.Op, X: nx, Type: in.Type})

			case ir.Call:
				newArgs := make([]ir.Operand, len(in.Args))
				for i, a := range in.Args {
					newArgs[i] = subst(a)
				}
				if in.Dst != "" {
					delete(known, in.Dst)
				}
				nb.Instrs = append(nb.Instrs, ir.Call{Dst: in.Dst, Func: in.Func, Args: newArgs, Type: in.Type})

			case ir.IfGoto:
				nb.Instrs = append(nb.Instrs, ir.IfGoto{Cond: subst(in.Cond), TrueLabel: in.TrueLabel, FalseLabel: in.FalseLabel})

			case ir.Return:
				if in.Value != nil {
					v := subst(*in.Value)
					nb.Instrs = append(nb.Instrs, ir.Return{Value: &v})
				} else {
					nb.Instrs = append(nb.Instrs, in)
				}

			default:
				nb.Instrs = append(nb.Instrs, instr)
			}
		}
		newFn.Blocks = append(newFn.Blocks, nb)
	}
	return newFn
}
