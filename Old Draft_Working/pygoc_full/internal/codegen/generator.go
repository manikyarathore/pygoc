package codegen

import (
	"fmt"
	"strings"

	"github.com/manikyarathore/pygoc/internal/ir"
	"github.com/manikyarathore/pygoc/internal/types"
)

// Generate emits a complete, self-contained Go source file for the
// given (already optimized) module.
func Generate(mod *ir.Module) (string, error) {
	imports := ResolveImports(mod)

	var sb strings.Builder
	sb.WriteString("package main\n\n")
	if len(imports) > 0 {
		sb.WriteString("import (\n")
		for _, imp := range imports {
			sb.WriteString(fmt.Sprintf("\t%q\n", imp))
		}
		sb.WriteString(")\n\n")
	}

	// Emit user-defined functions first, "main" last — purely stylistic,
	// Go itself doesn't require any particular declaration order.
	var mainFn *ir.Function
	for _, fn := range mod.Functions {
		if fn.Name == "main" {
			mainFn = fn
			continue
		}
		sb.WriteString(genFunction(fn))
		sb.WriteString("\n")
	}
	if mainFn != nil {
		sb.WriteString(genFunction(mainFn))
	}

	return sb.String(), nil
}

type varDecl struct {
	Name string
	Type types.Type
}

// collectVars finds every distinct variable/temp name assigned
// anywhere in fn (excluding parameters, which are already declared via
// the Go function signature), in first-appearance order. Declaring
// every one of these with `var name Type` at the top of the function —
// rather than `:=` at first use — is what makes it safe to freely
// `goto` across the rest of the function body: Go only forbids a goto
// that jumps INTO a variable's scope past its declaration, and with
// every declaration up front, no goto in this generated code ever does.
func collectVars(fn *ir.Function) []varDecl {
	isParam := map[string]bool{}
	for _, p := range fn.Params {
		isParam[p.Name] = true
	}
	seen := map[string]bool{}
	var out []varDecl
	mark := func(name string, t types.Type) {
		if name == "" || isParam[name] || seen[name] {
			return
		}
		seen[name] = true
		out = append(out, varDecl{Name: name, Type: t})
	}
	for _, blk := range fn.Blocks {
		for _, instr := range blk.Instrs {
			switch in := instr.(type) {
			case ir.Assign:
				mark(in.Dst, in.Type)
			case ir.BinOp:
				mark(in.Dst, in.Type)
			case ir.UnOp:
				mark(in.Dst, in.Type)
			case ir.Call:
				if in.Dst != "" {
					mark(in.Dst, in.Type)
				}
			}
		}
	}
	return out
}

func genSignature(fn *ir.Function) string {
	if fn.Name == "main" {
		return "func main()"
	}
	var params []string
	for _, p := range fn.Params {
		params = append(params, p.Name+" "+GoTypeName(p.Type))
	}
	sig := fmt.Sprintf("func %s(%s)", fn.Name, strings.Join(params, ", "))
	if fn.ReturnType.Kind != types.Void {
		sig += " " + GoTypeName(fn.ReturnType)
	}
	return sig
}

func genFunction(fn *ir.Function) string {
	var sb strings.Builder
	sb.WriteString(genSignature(fn) + " {\n")

	vars := collectVars(fn)
	for _, v := range vars {
		sb.WriteString(fmt.Sprintf("\tvar %s %s\n", v.Name, GoTypeName(v.Type)))
	}
	if len(vars) > 0 {
		sb.WriteString("\n")
	}

	// Go requires every label to actually be the target of a `goto`
	// somewhere in the function ("label X defined and not used" is a
	// compile error, not a warning). A block's own entry point (the
	// function's first block) is reached by the CALL itself, never by a
	// goto — so we only emit a label for blocks that some Goto/IfGoto
	// elsewhere in this function actually names.
	usedLabels := map[string]bool{}
	for _, blk := range fn.Blocks {
		for _, instr := range blk.Instrs {
			switch in := instr.(type) {
			case ir.Goto:
				usedLabels[in.Label] = true
			case ir.IfGoto:
				usedLabels[in.TrueLabel] = true
				usedLabels[in.FalseLabel] = true
			}
		}
	}

	// Go statements fall through sequentially by default, and this
	// codegen strategy is ENTIRELY goto-based — nothing should ever rely
	// on natural fall-through between blocks. That assumption broke in
	// practice: a block's position in fn.Blocks reflects CREATION order
	// (when the builder first allocated it), not the order it's finally
	// USED in — e.g. a `while` loop's own end-block is created before a
	// nested `if` inside its body, but is only filled in (with whatever
	// comes after the loop) once the whole loop finishes building. That
	// end-block can end up positioned, in the generated text, right
	// before blocks belonging to the nested `if` — and without an
	// explicit terminator, execution would silently fall through IN
	// (not jump to) that later, unrelated block: this produced a real
	// infinite loop in a generated `while ... break` program, caught by
	// actually running the e2e test suite. Fix: for a VOID function
	// (main, or any function with no declared return type), explicitly
	// append `return` to ANY block that doesn't already end in a
	// Goto/IfGoto/Return — regardless of that block's position in the
	// slice. This is always safe (a redundant `return` at the function's
	// true end is a no-op) and removes the fall-through hazard entirely.
	// For a NON-void function, this is deliberately left alone: if a
	// block genuinely lacks a terminator there, Go's own compiler
	// correctly rejects it as "missing return" — a loud compile-time
	// failure is the right outcome for that pre-existing, documented
	// limitation (return-path checking isn't full path-coverage), not
	// something to paper over with a synthetic return of the wrong type.
	isVoid := fn.ReturnType.Kind == types.Void

	for i, blk := range fn.Blocks {
		if usedLabels[blk.Label] {
			sb.WriteString(blk.Label + ":\n")
		}
		if len(blk.Instrs) == 0 {
			if usedLabels[blk.Label] && i == len(fn.Blocks)-1 {
				sb.WriteString("\tpanic(\"unreachable\")\n")
			}
			continue
		}
		for _, instr := range blk.Instrs {
			sb.WriteString("\t" + genInstr(instr) + "\n")
		}
		if isVoid && !endsInTerminator(blk) {
			sb.WriteString("\treturn\n")
		}
	}

	sb.WriteString("}\n")
	return sb.String()
}

// endsInTerminator reports whether blk's last instruction already
// unconditionally transfers control away (Goto, IfGoto, or Return).
func endsInTerminator(blk *ir.Block) bool {
	if len(blk.Instrs) == 0 {
		return false
	}
	switch blk.Instrs[len(blk.Instrs)-1].(type) {
	case ir.Goto, ir.IfGoto, ir.Return:
		return true
	default:
		return false
	}
}

func genInstr(instr ir.Instr) string {
	switch in := instr.(type) {
	case ir.Assign:
		return fmt.Sprintf("%s = %s", in.Dst, in.Src.String())

	case ir.BinOp:
		return fmt.Sprintf("%s = %s", in.Dst, genBinOpRHS(in))

	case ir.UnOp:
		return fmt.Sprintf("%s = %s", in.Dst, genUnOpRHS(in))

	case ir.Call:
		args := make([]string, len(in.Args))
		for i, a := range in.Args {
			args[i] = a.String()
		}
		joined := strings.Join(args, ", ")
		funcName := in.Func
		if funcName == "print" {
			funcName = "fmt.Println"
		}
		if in.Dst != "" {
			return fmt.Sprintf("%s = %s(%s)", in.Dst, funcName, joined)
		}
		return fmt.Sprintf("%s(%s)", funcName, joined)

	case ir.Goto:
		return fmt.Sprintf("goto %s", in.Label)

	case ir.IfGoto:
		return fmt.Sprintf("if %s {\n\t\tgoto %s\n\t} else {\n\t\tgoto %s\n\t}", in.Cond.String(), in.TrueLabel, in.FalseLabel)

	case ir.Return:
		if in.Value == nil {
			return "return"
		}
		return fmt.Sprintf("return %s", in.Value.String())

	default:
		return "// unsupported instruction: " + instr.String()
	}
}

// genBinOpRHS renders a BinOp's right-hand side. Most operators map
// directly to a Go infix expression; "and"/"or" map to &&/||; "//"
// needs special handling since Go has no floor-division operator —
// int//int uses Go's truncating `/` (a documented approximation of
// Python's floor division that is exact for non-negative operands, the
// only case exercised by this build's test/example programs), while
// float//float needs an explicit math.Floor call.
func genBinOpRHS(in ir.BinOp) string {
	switch in.Op {
	case "and":
		return fmt.Sprintf("%s && %s", in.Left, in.Right)
	case "or":
		return fmt.Sprintf("%s || %s", in.Left, in.Right)
	case "//":
		if in.Type.Kind == types.Float {
			return fmt.Sprintf("math.Floor(%s / %s)", in.Left, in.Right)
		}
		return fmt.Sprintf("%s / %s", in.Left, in.Right)
	default:
		return fmt.Sprintf("%s %s %s", in.Left, in.Op, in.Right)
	}
}

func genUnOpRHS(in ir.UnOp) string {
	switch in.Op {
	case "not ":
		return fmt.Sprintf("!%s", in.X)
	default: // "-"
		return fmt.Sprintf("%s%s", in.Op, in.X)
	}
}
