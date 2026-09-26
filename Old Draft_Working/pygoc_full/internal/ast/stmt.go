package ast

import "github.com/manikyarathore/pygoc/internal/token"

// Stmt is any node that represents an action rather than a value:
// an assignment, an if-block, a function definition, and so on.
type Stmt interface {
	Node
	stmtNode() // marker method — restricts implementers to this package's intent
}

// ---- Type annotations -------------------------------------------------

// TypeExpr is a parsed type annotation, e.g. `int`, `list[int]`,
// `dict[string, int]`. Params is non-empty only for parameterized types
// (list/dict/tuple); a plain type like `int` has Params == nil.
//
// NOTE: this is the PARSED SHAPE of a type annotation only. Turning it
// into an actual internal/types.Type (with real equality/inference
// rules) happens in Phase 3b — the parser's job stops at "here is the
// syntax the programmer wrote", not "here is what it resolves to".
type TypeExpr struct {
	Position
	Name   string // "int", "float", "list", "dict", "tuple", ...
	Params []*TypeExpr
}

// ---- Program root -------------------------------------------------

// Program is the root node: a PyGo source file is just a flat sequence
// of top-level statements (function definitions and/or top-level code).
type Program struct {
	Statements []Stmt
}

// ---- Simple statements -------------------------------------------------

// ExprStmt is an expression used on its own as a statement, most
// commonly a call: `print(x)` on its own line.
type ExprStmt struct {
	Position
	X Expr
}

func (*ExprStmt) stmtNode() {}

// AssignStmt covers `=`, `+=`, `-=`, `*=`, `/=`. Targets holds one or
// more assignment targets — more than one only for tuple unpacking,
// e.g. `a, b = t`, where Targets == [a, b].
type AssignStmt struct {
	Position
	Targets []Expr // Identifier, IndexExpr, or AttributeExpr
	Op      token.Type // ASSIGN, PLUSEQ, MINUSEQ, STAREQ, or SLASHEQ
	Value   Expr
}

func (*AssignStmt) stmtNode() {}

// ReturnStmt is `return`, `return x`, or `return a, b` (multiple
// return values — Values holds every returned expression in order).
type ReturnStmt struct {
	Position
	Values []Expr // empty for a bare `return`
}

func (*ReturnStmt) stmtNode() {}

// BreakStmt is `break`.
type BreakStmt struct {
	Position
}

func (*BreakStmt) stmtNode() {}

// ContinueStmt is `continue`.
type ContinueStmt struct {
	Position
}

func (*ContinueStmt) stmtNode() {}

// ---- Compound statements -------------------------------------------------

// ElifClause is one `elif cond: body` branch inside an IfStmt.
type ElifClause struct {
	Position
	Cond Expr
	Body []Stmt
}

// IfStmt is `if cond: ... elif cond: ... else: ...`. Elifs and Else
// are both optional (nil/empty when absent).
type IfStmt struct {
	Position
	Cond  Expr
	Then  []Stmt
	Elifs []ElifClause
	Else  []Stmt // nil if there's no else branch
}

func (*IfStmt) stmtNode() {}

// WhileStmt is `while cond: body`.
type WhileStmt struct {
	Position
	Cond Expr
	Body []Stmt
}

func (*WhileStmt) stmtNode() {}

// ForStmt is PyGo's restricted for-loop form: `for x in range(a, b):`.
// Start is always present; Stop is nil for the one-argument form
// `range(n)` (meaning "0 to n"), matching Python's own range() rules.
type ForStmt struct {
	Position
	VarName string
	Start   Expr
	Stop    Expr // nil for the one-argument range(n) form
	Body    []Stmt
}

func (*ForStmt) stmtNode() {}

// Param is one function parameter: `a: int`.
type Param struct {
	Position
	Name string
	Type *TypeExpr
}

// FuncDecl is a function definition:
//
//	def add(a: int, b: int) -> int:
//	    return a + b
//
// ReturnTypes holds more than one entry only for multiple-return-value
// functions (`-> (int, int)`); a function with no return annotation has
// ReturnTypes == nil (inferred/void).
type FuncDecl struct {
	Position
	Name        string
	Params      []Param
	ReturnTypes []*TypeExpr
	Body        []Stmt
}

func (*FuncDecl) stmtNode() {}
