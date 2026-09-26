package optimizer

import "github.com/manikyarathore/pygoc/internal/ir"

// transformFunction is the shared skeleton for any pass that rewrites
// instructions one at a time with no cross-instruction state (constant
// folding, algebraic simplification). Passes that DO need state across
// instructions (constant propagation, CSE, dead code elimination) build
// their own Function/Block copies directly — see their own files — but
// still follow the same "always return a fresh Function, never mutate
// the input" rule this helper enforces.
func transformFunction(fn *ir.Function, transform func(ir.Instr) ir.Instr) *ir.Function {
	newFn := &ir.Function{Name: fn.Name, Params: append([]ir.Param{}, fn.Params...), ReturnType: fn.ReturnType}
	for _, blk := range fn.Blocks {
		nb := &ir.Block{Label: blk.Label}
		for _, instr := range blk.Instrs {
			nb.Instrs = append(nb.Instrs, transform(instr))
		}
		newFn.Blocks = append(newFn.Blocks, nb)
	}
	return newFn
}

// Result retains the IR snapshot after EVERY pass, in order — this is
// deliberately not a black box: `pygoc optimize` prints every one of
// these fields under its own labeled header.
type Result struct {
	Original       *ir.Function
	AfterConstFold *ir.Function
	AfterConstProp *ir.Function
	AfterAlgebraic *ir.Function
	AfterCSE       *ir.Function
	AfterDeadCode  *ir.Function
}

// Final is the fully optimized function — what Phase 6 (codegen)
// actually consumes.
func (r *Result) Final() *ir.Function { return r.AfterDeadCode }

// Run executes all five passes, in the fixed order specified by the
// roadmap, retaining every intermediate result.
func Run(fn *ir.Function) *Result {
	r := &Result{Original: fn}
	r.AfterConstFold = ConstantFold(fn)
	r.AfterConstProp = ConstantPropagate(r.AfterConstFold)
	r.AfterAlgebraic = AlgebraicSimplify(r.AfterConstProp)
	r.AfterCSE = CommonSubexpressionElimination(r.AfterAlgebraic)
	r.AfterDeadCode = DeadCodeElimination(r.AfterCSE)
	return r
}

// ModuleResult is Run applied across every function in a Module.
type ModuleResult struct {
	PerFunction map[string]*Result
	Final       *ir.Module
}

func RunModule(mod *ir.Module) *ModuleResult {
	mr := &ModuleResult{PerFunction: map[string]*Result{}}
	finalMod := &ir.Module{}
	for _, fn := range mod.Functions {
		res := Run(fn)
		mr.PerFunction[fn.Name] = res
		finalMod.Functions = append(finalMod.Functions, res.Final())
	}
	mr.Final = finalMod
	return mr
}
