// Package ir implements Phase 4: lowering the typed AST into
// Three-Address Code (TAC), organized into labeled basic blocks that
// together form a Control Flow Graph (CFG).
//
// SUBMISSION SCOPE NOTE: mirrors internal/types and internal/semantic —
// only the core subset lowers here. Multiple return values lower using
// only the first return expression (documented cut, not a silent bug).
package ir

import (
	"fmt"

	"github.com/manikyarathore/pygoc/internal/types"
)

// Operand is either a compile-time constant or a reference to a
// variable/temporary. Every Operand carries its own Type — this is
// what lets Phase 6 (codegen) emit correctly-typed Go `var` declarations
// without needing a second type-inference pass over the IR.
type Operand struct {
	IsConst bool
	Const   any // int64, float64, bool, or string
	Name    string
	Type    types.Type
}

func ConstOperand(v any, t types.Type) Operand {
	return Operand{IsConst: true, Const: v, Type: t}
}

func VarOperand(name string, t types.Type) Operand {
	return Operand{IsConst: false, Name: name, Type: t}
}

func (o Operand) String() string {
	if o.IsConst {
		if s, ok := o.Const.(string); ok {
			return fmt.Sprintf("%q", s)
		}
		return fmt.Sprintf("%v", o.Const)
	}
	return o.Name
}

// Instr is any single TAC instruction. Every concrete type below
// implements String() for the `pygoc ir` debug printer.
type Instr interface {
	String() string
}

// Assign is a plain copy: dst = src.
type Assign struct {
	Dst  string
	Src  Operand
	Type types.Type
}

func (i Assign) String() string { return fmt.Sprintf("%s = %s", i.Dst, i.Src) }

// BinOp is dst = left OP right.
type BinOp struct {
	Dst         string
	Op          string
	Left, Right Operand
	Type        types.Type
}

func (i BinOp) String() string { return fmt.Sprintf("%s = %s %s %s", i.Dst, i.Left, i.Op, i.Right) }

// UnOp is dst = OP x.
type UnOp struct {
	Dst  string
	Op   string
	X    Operand
	Type types.Type
}

func (i UnOp) String() string { return fmt.Sprintf("%s = %s%s", i.Dst, i.Op, i.X) }

// Call is `call func, args...` (Dst == "" when the result is unused,
// e.g. `print(...)`).
type Call struct {
	Dst  string
	Func string
	Args []Operand
	Type types.Type
}

func (i Call) String() string {
	if i.Dst == "" {
		return fmt.Sprintf("call %s, %s", i.Func, joinOperands(i.Args))
	}
	return fmt.Sprintf("%s = call %s, %s", i.Dst, i.Func, joinOperands(i.Args))
}

func joinOperands(ops []Operand) string {
	s := ""
	for i, o := range ops {
		if i > 0 {
			s += ", "
		}
		s += o.String()
	}
	return s
}

// Goto is an unconditional jump.
type Goto struct {
	Label string
}

func (i Goto) String() string { return fmt.Sprintf("goto %s", i.Label) }

// IfGoto is a conditional branch: if Cond goto TrueLabel else goto FalseLabel.
type IfGoto struct {
	Cond                 Operand
	TrueLabel, FalseLabel string
}

func (i IfGoto) String() string {
	return fmt.Sprintf("if %s goto %s else goto %s", i.Cond, i.TrueLabel, i.FalseLabel)
}

// Return is `return` (Value == nil) or `return value`.
type Return struct {
	Value *Operand
}

func (i Return) String() string {
	if i.Value == nil {
		return "return"
	}
	return fmt.Sprintf("return %s", *i.Value)
}
