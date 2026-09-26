// Package semantic implements Phase 3c: it walks the parsed AST, builds
// the scoped symbol table (via internal/symbol), infers and checks
// types (via internal/types), and reports every semantic error it
// finds rather than stopping at the first (matching the rest of the
// compiler's error-recovery philosophy).
//
// SUBMISSION SCOPE NOTE: this build supports the CORE PyGo subset only
// — see internal/types' package doc for the exact list. Anything
// outside that subset (list/dict/tuple literals, tuple-unpacking
// assignment, multiple return values, method calls) is rejected here
// with a clear "not supported in this build" error, so the rest of the
// pipeline never has to guess what an unsupported construct means.
package semantic

import (
	"fmt"

	"github.com/manikyarathore/pygoc/internal/ast"
	"github.com/manikyarathore/pygoc/internal/symbol"
	"github.com/manikyarathore/pygoc/internal/types"
)

// Analyzer holds all state for one analysis run: the symbol table being
// built and the errors accumulated so far.
type Analyzer struct {
	Table           *symbol.Table
	errors          []error
	returnTypeStack []types.Type
	// definedOrder records the names defined at GLOBAL scope, in
	// declaration order — symbol.Scope deliberately doesn't expose its
	// internal map for iteration (that's an implementation detail), so
	// the analyzer keeps its own ordered record for `pygoc sema`'s
	// top-level symbol table printout.
	definedOrder []string
}

// New creates an Analyzer with a fresh global-scope symbol table.
func New() *Analyzer {
	return &Analyzer{Table: symbol.NewTable()}
}

// Analyze walks the whole program and returns every semantic error
// found. A nil/empty return means the program is valid and Table now
// holds every declared symbol with its resolved type.
func (a *Analyzer) Analyze(prog *ast.Program) []error {
	for _, s := range prog.Statements {
		a.stmt(s)
	}
	return a.errors
}

func (a *Analyzer) errorAt(line, col int, format string, args ...any) {
	a.errors = append(a.errors, &SemanticError{Line: line, Col: col, Msg: fmt.Sprintf(format, args...)})
}

// ---- statements -------------------------------------------------

func (a *Analyzer) stmt(s ast.Stmt) {
	switch n := s.(type) {
	case *ast.AssignStmt:
		a.assignStmt(n)
	case *ast.ExprStmt:
		if _, err := types.Infer(n.X, a.Table); err != nil {
			line, col := n.Pos()
			a.errorAt(line, col, "%s", err)
		}
	case *ast.IfStmt:
		a.ifStmt(n)
	case *ast.WhileStmt:
		a.whileStmt(n)
	case *ast.ForStmt:
		a.forStmt(n)
	case *ast.FuncDecl:
		a.funcDecl(n)
	case *ast.ReturnStmt:
		a.returnStmt(n)
	case *ast.BreakStmt, *ast.ContinueStmt:
		// no loop-context enforcement in this build — see checks.go note
	default:
		line, col := s.Pos()
		a.errorAt(line, col, "%T is not supported in this build", s)
	}
}

func (a *Analyzer) stmtList(stmts []ast.Stmt) {
	for _, s := range stmts {
		a.stmt(s)
	}
}

func (a *Analyzer) assignStmt(n *ast.AssignStmt) {
	line, col := n.Pos()

	if len(n.Targets) != 1 {
		a.errorAt(line, col, "tuple-unpacking assignment (multiple targets) is not supported in this build")
		return
	}
	target, ok := n.Targets[0].(*ast.Identifier)
	if !ok {
		a.errorAt(line, col, "only a plain variable name is supported as an assignment target in this build")
		return
	}

	valType, err := types.Infer(n.Value, a.Table)
	if err != nil {
		a.errorAt(line, col, "%s", err)
		return
	}
	if valType.Kind == types.Void {
		a.errorAt(line, col, "cannot assign a void expression (e.g. print(...)) to a variable")
		return
	}

	if !a.equalsAssign(n) {
		// compound assignment: x += 1 type-checks as x = x + 1
		existing, ok := a.Table.Resolve(target.Name)
		if !ok {
			a.errorAt(line, col, "undefined variable '%s'", target.Name)
			return
		}
		existingType, _ := existing.Type.(types.Type)
		result, err := types.BinaryOpResult(compoundOpToBinary(n.Op), existingType, valType)
		if err != nil {
			a.errorAt(line, col, "%s", err)
			return
		}
		if !result.Equals(existingType) {
			a.errorAt(line, col, "cannot apply compound assignment: %s produces %s, but '%s' is %s",
				n.Op, result, target.Name, existingType)
		}
		return
	}

	if a.Table.IsLocal(target.Name) {
		existing, _ := a.Table.Resolve(target.Name)
		existingType, _ := existing.Type.(types.Type)
		if !existingType.Equals(valType) {
			a.errorAt(line, col, "cannot assign %s to variable '%s' of type %s", valType, target.Name, existingType)
		}
		return
	}
	if a.Table.IsCaptured(target.Name) {
		a.errorAt(line, col, "cannot reassign '%s' — it is captured (read-only) from an enclosing function", target.Name)
		return
	}

	wasGlobal := a.Table.InGlobalScope()
	if err := a.Table.Define(&symbol.Symbol{Name: target.Name, Type: valType, Kind: symbol.VarSymbol, Line: line, Col: col}); err != nil {
		a.errorAt(line, col, "%s", err)
		return
	}
	if wasGlobal {
		a.definedOrder = append(a.definedOrder, target.Name)
	}
}

func (a *Analyzer) equalsAssign(n *ast.AssignStmt) bool {
	return n.Op.String() == "="
}

func (a *Analyzer) ifStmt(n *ast.IfStmt) {
	a.checkBoolCond(n.Cond)
	a.stmtList(n.Then)
	for _, elif := range n.Elifs {
		a.checkBoolCond(elif.Cond)
		a.stmtList(elif.Body)
	}
	a.stmtList(n.Else)
}

func (a *Analyzer) whileStmt(n *ast.WhileStmt) {
	a.checkBoolCond(n.Cond)
	a.stmtList(n.Body)
}

func (a *Analyzer) forStmt(n *ast.ForStmt) {
	line, col := n.Pos()
	a.checkIntExpr(n.Start)
	if n.Stop != nil {
		a.checkIntExpr(n.Stop)
	}

	// define/validate the loop variable as int in the CURRENT scope —
	// re-entering the same for-loop variable name across two separate
	// loops in the same function must not be treated as a redefinition
	// error, so we check IsLocal first rather than calling Define blindly.
	if a.Table.IsLocal(n.VarName) {
		existing, _ := a.Table.Resolve(n.VarName)
		existingType, _ := existing.Type.(types.Type)
		if existingType.Kind != types.Int {
			a.errorAt(line, col, "loop variable '%s' was previously used as %s, not int", n.VarName, existingType)
		}
	} else {
		a.Table.Define(&symbol.Symbol{Name: n.VarName, Type: types.IntT, Kind: symbol.VarSymbol, Line: line, Col: col})
	}

	a.stmtList(n.Body)
}

func (a *Analyzer) checkBoolCond(e ast.Expr) {
	t, err := types.Infer(e, a.Table)
	if err != nil {
		line, col := e.Pos()
		a.errorAt(line, col, "%s", err)
		return
	}
	if t.Kind != types.Bool {
		line, col := e.Pos()
		a.errorAt(line, col, "condition must be bool, got %s", t)
	}
}

func (a *Analyzer) checkIntExpr(e ast.Expr) {
	t, err := types.Infer(e, a.Table)
	if err != nil {
		line, col := e.Pos()
		a.errorAt(line, col, "%s", err)
		return
	}
	if t.Kind != types.Int {
		line, col := e.Pos()
		a.errorAt(line, col, "expected int, got %s", t)
	}
}

func (a *Analyzer) funcDecl(n *ast.FuncDecl) {
	line, col := n.Pos()

	retType := types.VoidT
	if len(n.ReturnTypes) > 1 {
		a.errorAt(line, col, "multiple return values are not supported in this build")
	} else if len(n.ReturnTypes) == 1 {
		rt, ok := types.FromTypeName(n.ReturnTypes[0].Name)
		if !ok {
			a.errorAt(line, col, "unknown return type '%s'", n.ReturnTypes[0].Name)
		} else {
			retType = rt
		}
	}

	// Defined in the ENCLOSING scope, before entering the function body,
	// so recursive and forward calls resolve correctly.
	wasGlobal := a.Table.InGlobalScope()
	if err := a.Table.Define(&symbol.Symbol{Name: n.Name, Type: retType, Kind: symbol.FuncSymbol, Line: line, Col: col}); err != nil {
		a.errorAt(line, col, "%s", err)
	} else if wasGlobal {
		a.definedOrder = append(a.definedOrder, n.Name)
	}

	a.Table.EnterFunctionScope()
	for _, p := range n.Params {
		pt, ok := types.FromTypeName(p.Type.Name)
		if !ok {
			a.errorAt(line, col, "unknown parameter type '%s' for '%s'", p.Type.Name, p.Name)
			pt = types.UnknownT
		}
		a.Table.Define(&symbol.Symbol{Name: p.Name, Type: pt, Kind: symbol.ParamSymbol, Line: line, Col: col})
	}

	a.returnTypeStack = append(a.returnTypeStack, retType)
	a.stmtList(n.Body)
	if retType.Kind != types.Void && !hasReturn(n.Body) {
		a.errorAt(line, col, "function '%s' must return %s but has no return statement", n.Name, retType)
	}
	a.returnTypeStack = a.returnTypeStack[:len(a.returnTypeStack)-1]

	a.Table.ExitScope()
}

func (a *Analyzer) returnStmt(n *ast.ReturnStmt) {
	line, col := n.Pos()
	if len(a.returnTypeStack) == 0 {
		a.errorAt(line, col, "'return' outside of a function")
		return
	}
	want := a.returnTypeStack[len(a.returnTypeStack)-1]

	if len(n.Values) > 1 {
		a.errorAt(line, col, "multiple return values are not supported in this build")
		return
	}
	if len(n.Values) == 0 {
		if want.Kind != types.Void {
			a.errorAt(line, col, "function must return %s but this 'return' has no value", want)
		}
		return
	}
	got, err := types.Infer(n.Values[0], a.Table)
	if err != nil {
		a.errorAt(line, col, "%s", err)
		return
	}
	if !got.Equals(want) {
		a.errorAt(line, col, "return type mismatch: function declares %s, this returns %s", want, got)
	}
}
