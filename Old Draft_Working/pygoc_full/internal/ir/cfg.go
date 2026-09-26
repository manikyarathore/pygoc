package ir

import (
	"strings"

	"github.com/manikyarathore/pygoc/internal/types"
)

// Block is one basic block: a label and a straight-line run of
// instructions with no internal branches (any branch/jump is always
// the LAST instruction in a block, by construction in builder.go).
type Block struct {
	Label  string
	Instrs []Instr
}

// String renders just this block's own label and instructions —
// convenient for debug output and test failure messages that only want
// to show one block rather than a whole function.
func (b *Block) String() string {
	var sb strings.Builder
	sb.WriteString(b.Label + ":\n")
	for _, instr := range b.Instrs {
		sb.WriteString("    " + instr.String() + "\n")
	}
	return sb.String()
}

// Param describes one function parameter for codegen purposes (name +
// its Go-mappable type).
type Param struct {
	Name string
	Type types.Type
}

// Function is one PyGo function (or the synthetic "main" function
// holding top-level statements) lowered to a sequence of basic blocks.
type Function struct {
	Name       string
	Params     []Param
	ReturnType types.Type // types.VoidT for a function with no declared return
	Blocks     []*Block
}

// Module is the whole program's IR: one Function per PyGo function,
// plus the synthetic "main" function for top-level statements.
type Module struct {
	Functions []*Function
}

// String renders the whole module as TAC + labeled blocks, exactly
// matching the format used throughout docs/ROADMAP.md's IR examples.
func (m *Module) String() string {
	var sb strings.Builder
	for i, fn := range m.Functions {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(fn.String())
	}
	return sb.String()
}

func (fn *Function) String() string {
	var sb strings.Builder
	sb.WriteString("func " + fn.Name + ":\n")
	for _, b := range fn.Blocks {
		sb.WriteString(b.Label + ":\n")
		for _, instr := range b.Instrs {
			sb.WriteString("    " + instr.String() + "\n")
		}
	}
	return sb.String()
}

// Clone deep-copies a Function. Every optimizer pass returns a fresh
// Function built from scratch rather than mutating its input in place
// (see internal/optimizer), so Clone exists mainly for tests/tools that
// want an independent snapshot to compare against; the passes
// themselves don't need to call it.
func (fn *Function) Clone() *Function {
	out := &Function{Name: fn.Name, Params: append([]Param{}, fn.Params...), ReturnType: fn.ReturnType}
	for _, b := range fn.Blocks {
		nb := &Block{Label: b.Label, Instrs: append([]Instr{}, b.Instrs...)}
		out.Blocks = append(out.Blocks, nb)
	}
	return out
}
