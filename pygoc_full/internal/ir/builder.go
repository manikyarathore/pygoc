package ir

import (
	"fmt"

	"github.com/manikyarathore/pygoc/internal/ast"
	"github.com/manikyarathore/pygoc/internal/symbol"
	"github.com/manikyarathore/pygoc/internal/token"
	"github.com/manikyarathore/pygoc/internal/types"
)

// Builder lowers a validated AST into IR.
//
// DESIGN NOTE: the semantic analyzer's symbol.Table has already exited
// every function scope by the time analysis finishes (scopes it popped
// are no longer reachable), so the builder cannot reuse that table to
// resolve function-local variable types. Instead it keeps its OWN
// symbol.Table and re-walks the same Define/EnterFunctionScope/
// ExitScope choreography the analyzer used. This is deliberate, not
// wasted work: the program is already known-valid at this point (no
// errors), so this second walk only needs to redo the bookkeeping (what
// type does each name have here), not the validation.
type Builder struct {
	tempCount  int
	blockCount int
	table      *symbol.Table
	module     *Module
	fn         *Function
	block      *Block
	loopStack  []loopLabels
}

type loopLabels struct {
	ContinueLabel string
	BreakLabel    string
}

// NewBuilder creates a Builder with a fresh global-scope symbol table.
func NewBuilder() *Builder {
	return &Builder{table: symbol.NewTable()}
}

// Build lowers an entire program to a Module: one Function per PyGo
// function, plus a synthetic "main" function holding the top-level
// statements.
func Build(prog *ast.Program) *Module {
	b := NewBuilder()
	module := &Module{}
	b.module = module

	mainFn := &Function{Name: "main", ReturnType: types.VoidT}
	module.Functions = append(module.Functions, mainFn)
	b.fn = mainFn
	b.block = b.newBlock()

	for _, s := range prog.Statements {
		b.buildStmt(s)
	}
	return module
}

// ---- block/temp bookkeeping -------------------------------------------------

func (b *Builder) newTemp() string {
	b.tempCount++
	return fmt.Sprintf("t%d", b.tempCount)
}

func (b *Builder) newBlock() *Block {
	b.blockCount++
	blk := &Block{Label: fmt.Sprintf("B%d", b.blockCount)}
	b.fn.Blocks = append(b.fn.Blocks, blk)
	return blk
}

func (b *Builder) emit(i Instr) {
	b.block.Instrs = append(b.block.Instrs, i)
}

// endsInJump reports whether blk's last instruction already
// unconditionally transfers control away (a Goto or a Return) — used to
// avoid emitting a redundant, unreachable jump right after one (e.g.
// `return 1` followed by a pointless `goto End`). Purely a cleanliness
// improvement: Go tolerates unreachable straight-line code just fine,
// but a compiler whose whole point is transparent, inspectable IR
// shouldn't litter its own output with dead instructions it could
// easily avoid emitting in the first place.
func endsInJump(blk *Block) bool {
	if len(blk.Instrs) == 0 {
		return false
	}
	switch blk.Instrs[len(blk.Instrs)-1].(type) {
	case Goto, Return:
		return true
	default:
		return false
	}
}

// emitJumpUnlessTerminated emits `goto label` UNLESS the current block
// already ends in an unconditional jump or a return.
func (b *Builder) emitJumpUnlessTerminated(label string) {
	if !endsInJump(b.block) {
		b.emit(Goto{Label: label})
	}
}

// ---- statements -------------------------------------------------

func (b *Builder) buildStmt(s ast.Stmt) {
	switch n := s.(type) {
	case *ast.AssignStmt:
		b.buildAssign(n)
	case *ast.ExprStmt:
		b.buildExpr(n.X)
	case *ast.IfStmt:
		b.buildIf(n)
	case *ast.WhileStmt:
		b.buildWhile(n)
	case *ast.ForStmt:
		b.buildFor(n)
	case *ast.ReturnStmt:
		b.buildReturn(n)
	case *ast.BreakStmt:
		if len(b.loopStack) > 0 {
			b.emit(Goto{Label: b.loopStack[len(b.loopStack)-1].BreakLabel})
		}
	case *ast.ContinueStmt:
		if len(b.loopStack) > 0 {
			b.emit(Goto{Label: b.loopStack[len(b.loopStack)-1].ContinueLabel})
		}
	case *ast.FuncDecl:
		b.buildFuncDecl(n)
	}
}

func (b *Builder) buildAssign(n *ast.AssignStmt) {
	target := n.Targets[0].(*ast.Identifier) // semantic analysis guaranteed this shape

	if n.Op == token.ASSIGN {
		val := b.buildExpr(n.Value)
		b.emit(Assign{Dst: target.Name, Src: val, Type: val.Type})
		if !b.table.IsLocal(target.Name) {
			b.table.Define(&symbol.Symbol{Name: target.Name, Type: val.Type, Kind: symbol.VarSymbol})
		}
		return
	}

	// Compound assignment: `x += value` lowers as `x = x + value`.
	existing, _ := b.table.Resolve(target.Name)
	existingType, _ := existing.Type.(types.Type)
	left := VarOperand(target.Name, existingType)
	right := b.buildExpr(n.Value)
	resultType, _ := types.BinaryOpResult(compoundBinaryToken(n.Op), existingType, right.Type)
	b.emit(BinOp{Dst: target.Name, Op: opTokenToTACString(compoundBinaryToken(n.Op)), Left: left, Right: right, Type: resultType})
}

func (b *Builder) buildFuncDecl(n *ast.FuncDecl) {
	retType := types.VoidT
	if len(n.ReturnTypes) == 1 {
		if rt, ok := types.FromTypeName(n.ReturnTypes[0].Name); ok {
			retType = rt
		}
	}

	b.table.Define(&symbol.Symbol{Name: n.Name, Type: retType, Kind: symbol.FuncSymbol})

	var params []Param
	b.table.EnterFunctionScope()
	for _, p := range n.Params {
		pt, _ := types.FromTypeName(p.Type.Name)
		b.table.Define(&symbol.Symbol{Name: p.Name, Type: pt, Kind: symbol.ParamSymbol})
		params = append(params, Param{Name: p.Name, Type: pt})
	}

	newFn := &Function{Name: n.Name, Params: params, ReturnType: retType}
	b.module.Functions = append(b.module.Functions, newFn)

	savedFn, savedBlock := b.fn, b.block
	b.fn = newFn
	b.block = b.newBlock()

	for _, s := range n.Body {
		b.buildStmt(s)
	}

	b.table.ExitScope()
	b.fn, b.block = savedFn, savedBlock
}

func (b *Builder) buildReturn(n *ast.ReturnStmt) {
	if len(n.Values) == 0 {
		b.emit(Return{Value: nil})
		return
	}
	// SUBMISSION SCOPE: only the first return value lowers here — the
	// semantic analyzer already rejects any function that declares or
	// attempts multiple return values, so this is never reached with
	// more than one value in a validated program.
	v := b.buildExpr(n.Values[0])
	b.emit(Return{Value: &v})
}

// buildIf lowers if/elif/else into an IfGoto-based diamond, desugaring
// any elif chain via buildElifChain.
func (b *Builder) buildIf(n *ast.IfStmt) {
	cond := b.buildExpr(n.Cond)
	thenBlk := b.newBlock()
	hasElse := len(n.Elifs) > 0 || n.Else != nil
	var elseBlk *Block
	if hasElse {
		elseBlk = b.newBlock()
	}
	endBlk := b.newBlock()

	falseLabel := endBlk.Label
	if hasElse {
		falseLabel = elseBlk.Label
	}
	b.emit(IfGoto{Cond: cond, TrueLabel: thenBlk.Label, FalseLabel: falseLabel})

	b.block = thenBlk
	for _, s := range n.Then {
		b.buildStmt(s)
	}
	b.emitJumpUnlessTerminated(endBlk.Label)

	if hasElse {
		b.block = elseBlk
		if len(n.Elifs) > 0 {
			b.buildElifChain(n.Elifs, n.Else, endBlk.Label)
		} else {
			for _, s := range n.Else {
				b.buildStmt(s)
			}
			b.emitJumpUnlessTerminated(endBlk.Label)
		}
	}
	b.block = endBlk
}

func (b *Builder) buildElifChain(elifs []ast.ElifClause, els []ast.Stmt, endLabel string) {
	first := elifs[0]
	cond := b.buildExpr(first.Cond)
	thenBlk := b.newBlock()
	hasMore := len(elifs) > 1 || els != nil
	var elseBlk *Block
	if hasMore {
		elseBlk = b.newBlock()
	}
	falseLabel := endLabel
	if hasMore {
		falseLabel = elseBlk.Label
	}
	b.emit(IfGoto{Cond: cond, TrueLabel: thenBlk.Label, FalseLabel: falseLabel})

	b.block = thenBlk
	for _, s := range first.Body {
		b.buildStmt(s)
	}
	b.emitJumpUnlessTerminated(endLabel)

	if hasMore {
		b.block = elseBlk
		if len(elifs) > 1 {
			b.buildElifChain(elifs[1:], els, endLabel)
		} else {
			for _, s := range els {
				b.buildStmt(s)
			}
			b.emitJumpUnlessTerminated(endLabel)
		}
	}
}

func (b *Builder) buildWhile(n *ast.WhileStmt) {
	condBlk := b.newBlock()
	b.emit(Goto{Label: condBlk.Label})

	b.block = condBlk
	cond := b.buildExpr(n.Cond)
	bodyBlk := b.newBlock()
	endBlk := b.newBlock()
	b.emit(IfGoto{Cond: cond, TrueLabel: bodyBlk.Label, FalseLabel: endBlk.Label})

	b.loopStack = append(b.loopStack, loopLabels{ContinueLabel: condBlk.Label, BreakLabel: endBlk.Label})
	b.block = bodyBlk
	for _, s := range n.Body {
		b.buildStmt(s)
	}
	b.emitJumpUnlessTerminated(condBlk.Label)
	b.loopStack = b.loopStack[:len(b.loopStack)-1]

	b.block = endBlk
}

// buildFor desugars `for i in range(a[, b]):` into the equivalent of:
//
//	i = a (or 0);  bound = b (or a)
//	while i < bound:
//	    <body>
//	    i = i + 1
func (b *Builder) buildFor(n *ast.ForStmt) {
	var startOperand, boundOperand Operand
	if n.Stop == nil {
		startOperand = ConstOperand(int64(0), types.IntT)
		boundOperand = b.buildExpr(n.Start)
	} else {
		startOperand = b.buildExpr(n.Start)
		boundOperand = b.buildExpr(n.Stop)
	}
	b.emit(Assign{Dst: n.VarName, Src: startOperand, Type: types.IntT})
	if !b.table.IsLocal(n.VarName) {
		b.table.Define(&symbol.Symbol{Name: n.VarName, Type: types.IntT, Kind: symbol.VarSymbol})
	}
	boundTemp := b.newTemp()
	b.emit(Assign{Dst: boundTemp, Src: boundOperand, Type: types.IntT})

	condBlk := b.newBlock()
	b.emit(Goto{Label: condBlk.Label})

	b.block = condBlk
	condDst := b.newTemp()
	b.emit(BinOp{Dst: condDst, Op: "<", Left: VarOperand(n.VarName, types.IntT), Right: VarOperand(boundTemp, types.IntT), Type: types.BoolT})
	bodyBlk := b.newBlock()
	incBlk := b.newBlock()
	endBlk := b.newBlock()
	b.emit(IfGoto{Cond: VarOperand(condDst, types.BoolT), TrueLabel: bodyBlk.Label, FalseLabel: endBlk.Label})

	// `continue` must jump to the INCREMENT block, never straight back to
	// the condition test — otherwise the loop variable would never
	// advance and `continue` would hang the generated program in an
	// infinite loop. Same "guarantee forward progress" lesson as the
	// lexer's EOF-ordering fix, applied here at IR-construction time.
	b.loopStack = append(b.loopStack, loopLabels{ContinueLabel: incBlk.Label, BreakLabel: endBlk.Label})
	b.block = bodyBlk
	for _, s := range n.Body {
		b.buildStmt(s)
	}
	b.emitJumpUnlessTerminated(incBlk.Label)
	b.loopStack = b.loopStack[:len(b.loopStack)-1]

	b.block = incBlk
	incDst := b.newTemp()
	b.emit(BinOp{Dst: incDst, Op: "+", Left: VarOperand(n.VarName, types.IntT), Right: ConstOperand(int64(1), types.IntT), Type: types.IntT})
	b.emit(Assign{Dst: n.VarName, Src: VarOperand(incDst, types.IntT), Type: types.IntT})
	b.emit(Goto{Label: condBlk.Label})

	b.block = endBlk
}

// ---- expressions -------------------------------------------------

func (b *Builder) buildExpr(e ast.Expr) Operand {
	switch n := e.(type) {
	case *ast.IntLiteral:
		return ConstOperand(n.Value, types.IntT)
	case *ast.FloatLiteral:
		return ConstOperand(n.Value, types.FloatT)
	case *ast.BoolLiteral:
		return ConstOperand(n.Value, types.BoolT)
	case *ast.StringLiteral:
		return ConstOperand(n.Value, types.StringT)

	case *ast.Identifier:
		t := types.UnknownT
		if sym, ok := b.table.Resolve(n.Name); ok {
			t, _ = sym.Type.(types.Type)
		}
		return VarOperand(n.Name, t)

	case *ast.UnaryExpr:
		x := b.buildExpr(n.Operand)
		resultType, _ := types.UnaryOpResult(n.Op, x.Type)
		dst := b.newTemp()
		b.emit(UnOp{Dst: dst, Op: unaryOpToTACString(n.Op), X: x, Type: resultType})
		return VarOperand(dst, resultType)

	case *ast.BinaryExpr:
		left := b.buildExpr(n.Left)
		right := b.buildExpr(n.Right)
		resultType, _ := types.BinaryOpResult(n.Op, left.Type, right.Type)
		dst := b.newTemp()
		b.emit(BinOp{Dst: dst, Op: opTokenToTACString(n.Op), Left: left, Right: right, Type: resultType})
		return VarOperand(dst, resultType)

	case *ast.CallExpr:
		callee, _ := n.Callee.(*ast.Identifier) // semantic analysis guaranteed this shape
		var args []Operand
		for _, a := range n.Args {
			args = append(args, b.buildExpr(a))
		}
		if callee.Name == "print" {
			b.emit(Call{Dst: "", Func: "print", Args: args, Type: types.VoidT})
			return Operand{}
		}
		retType := types.UnknownT
		if sym, ok := b.table.Resolve(callee.Name); ok {
			retType, _ = sym.Type.(types.Type)
		}
		dst := ""
		if retType.Kind != types.Void {
			dst = b.newTemp()
		}
		b.emit(Call{Dst: dst, Func: callee.Name, Args: args, Type: retType})
		return VarOperand(dst, retType)

	default:
		// The semantic analyzer already rejects every unsupported
		// expression form before the pipeline ever reaches IR
		// construction, so this path is unreachable for a validated
		// program — it exists only as a defensive fallback.
		return ConstOperand("<unsupported>", types.UnknownT)
	}
}

// ---- small operator-string helpers -------------------------------------------------

func opTokenToTACString(t token.Type) string {
	switch t {
	case token.PLUS:
		return "+"
	case token.MINUS:
		return "-"
	case token.STAR:
		return "*"
	case token.SLASH:
		return "/"
	case token.PERCENT:
		return "%"
	case token.DSLASH:
		return "//"
	case token.EQ:
		return "=="
	case token.NEQ:
		return "!="
	case token.LT:
		return "<"
	case token.GT:
		return ">"
	case token.LE:
		return "<="
	case token.GE:
		return ">="
	case token.AND:
		return "and"
	case token.OR:
		return "or"
	default:
		return t.String()
	}
}

func unaryOpToTACString(t token.Type) string {
	switch t {
	case token.MINUS:
		return "-"
	case token.NOT:
		return "not "
	default:
		return t.String()
	}
}

// compoundBinaryToken maps a compound-assignment operator to the plain
// binary operator it implies. Small, deliberate duplication of
// internal/semantic's identically-shaped helper — see that package's
// doc comment on why the builder keeps its own bookkeeping rather than
// reaching into semantic's (already-exited) scopes.
func compoundBinaryToken(op token.Type) token.Type {
	switch op {
	case token.PLUSEQ:
		return token.PLUS
	case token.MINUSEQ:
		return token.MINUS
	case token.STAREQ:
		return token.STAR
	case token.SLASHEQ:
		return token.SLASH
	default:
		return op
	}
}
