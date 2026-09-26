package codegen

import (
	"sort"

	"github.com/manikyarathore/pygoc/internal/ir"
	"github.com/manikyarathore/pygoc/internal/types"
)

// ResolveImports scans the whole module and returns exactly the Go
// standard-library packages the generated code actually needs — never
// more. In this build's supported subset that's only ever "fmt"
// (triggered by any print(...) call) and "math" (triggered by a "//"
// floor-division between floats, which needs math.Floor).
func ResolveImports(mod *ir.Module) []string {
	need := map[string]bool{}
	for _, fn := range mod.Functions {
		for _, blk := range fn.Blocks {
			for _, instr := range blk.Instrs {
				switch in := instr.(type) {
				case ir.Call:
					if in.Func == "print" {
						need["fmt"] = true
					}
				case ir.BinOp:
					if in.Op == "//" && in.Type.Kind == types.Float {
						need["math"] = true
					}
				}
			}
		}
	}
	var out []string
	for pkg := range need {
		out = append(out, pkg)
	}
	sort.Strings(out)
	return out
}
