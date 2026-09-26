package optimizer

import "github.com/manikyarathore/pygoc/internal/ir"

// DeadCodeElimination removes any Assign/BinOp/UnOp instruction whose
// result is never used anywhere in the function. Instructions with
// possible side effects (Call, Goto, IfGoto, Return) are always kept —
// even a Call whose result is discarded (Dst == "") might be `print`,
// which has an externally visible effect and must never be removed.
//
// FIXED-POINT DESIGN NOTE (fixed after a real bug): removing one dead
// instruction can make ANOTHER instruction newly dead — e.g. `t1 = 1`
// followed by `x = t1` where `x` turns out to be unused: a single
// scan-then-filter pass correctly removes `x = t1` (x is unused) but,
// having already computed its "used" set before removing anything, does
// NOT notice that `t1` is now unused too, since `x = t1` (t1's only
// reader) was itself just deleted. Left as a single pass, this can
// leave an orphaned `t1 = 1` behind — which Go's compiler rejects
// outright as "declared and not used" (Go treats this as a hard error,
// unlike merely-unreachable code, which it tolerates). Running the
// scan-and-filter to a fixed point (repeat until a full pass removes
// nothing) closes that gap.
func DeadCodeElimination(fn *ir.Function) *ir.Function {
	current := fn
	const maxIterations = 10 // safety cap; real programs converge in 1-3 passes
	for i := 0; i < maxIterations; i++ {
		next, removed := deadCodePass(current)
		if !removed {
			return next
		}
		current = next
	}
	return current
}

// deadCodePass runs ONE scan-then-filter pass and reports whether it
// removed anything.
func deadCodePass(fn *ir.Function) (*ir.Function, bool) {
	used := map[string]bool{}
	mark := func(op ir.Operand) {
		if !op.IsConst && op.Name != "" {
			used[op.Name] = true
		}
	}

	for _, blk := range fn.Blocks {
		for _, instr := range blk.Instrs {
			switch in := instr.(type) {
			case ir.BinOp:
				mark(in.Left)
				mark(in.Right)
			case ir.UnOp:
				mark(in.X)
			case ir.Assign:
				mark(in.Src)
			case ir.Call:
				for _, a := range in.Args {
					mark(a)
				}
			case ir.IfGoto:
				mark(in.Cond)
			case ir.Return:
				if in.Value != nil {
					mark(*in.Value)
				}
			}
		}
	}

	removedAny := false
	newFn := &ir.Function{Name: fn.Name, Params: append([]ir.Param{}, fn.Params...), ReturnType: fn.ReturnType}
	for _, blk := range fn.Blocks {
		nb := &ir.Block{Label: blk.Label}
		for _, instr := range blk.Instrs {
			switch in := instr.(type) {
			case ir.Assign:
				if !used[in.Dst] {
					removedAny = true
					continue
				}
			case ir.BinOp:
				if !used[in.Dst] {
					removedAny = true
					continue
				}
			case ir.UnOp:
				if !used[in.Dst] {
					removedAny = true
					continue
				}
			}
			nb.Instrs = append(nb.Instrs, instr)
		}
		newFn.Blocks = append(newFn.Blocks, nb)
	}
	return newFn, removedAny
}
