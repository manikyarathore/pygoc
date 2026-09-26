package optimizer

import "github.com/manikyarathore/pygoc/internal/ir"

// CommonSubexpressionElimination reuses a previously computed value
// instead of recomputing an identical expression: if `t1 = a + b` has
// already been computed earlier in the SAME block, a later `t2 = a + b`
// becomes `t2 = t1`.
//
// SUBMISSION SCOPE NOTE: the "seen expressions" map resets at each
// block boundary rather than tracking dominance across the whole CFG —
// reusing a value computed in a block that doesn't always execute
// before the current one would be unsound. Restricting to
// within-a-block reuse is the safe, simple version of this pass; a full
// implementation would use dominator-tree-aware value numbering.
func CommonSubexpressionElimination(fn *ir.Function) *ir.Function {
	newFn := &ir.Function{Name: fn.Name, Params: append([]ir.Param{}, fn.Params...), ReturnType: fn.ReturnType}
	for _, blk := range fn.Blocks {
		seen := map[string]string{} // "left op right" -> first Dst that computed it
		nb := &ir.Block{Label: blk.Label}
		for _, instr := range blk.Instrs {
			in, ok := instr.(ir.BinOp)
			if !ok {
				nb.Instrs = append(nb.Instrs, instr)
				continue
			}
			key := in.Left.String() + " " + in.Op + " " + in.Right.String()
			if firstDst, ok := seen[key]; ok {
				nb.Instrs = append(nb.Instrs, ir.Assign{Dst: in.Dst, Src: ir.VarOperand(firstDst, in.Type), Type: in.Type})
				continue
			}
			seen[key] = in.Dst
			nb.Instrs = append(nb.Instrs, instr)
		}
		newFn.Blocks = append(newFn.Blocks, nb)
	}
	return newFn
}
