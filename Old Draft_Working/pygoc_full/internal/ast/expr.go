// Package ast defines the Abstract Syntax Tree node types PyGoC's parser
// produces and every later phase (symbol table, type checker, IR builder)
// consumes.
//
// This file is DATA ONLY — no parsing logic lives here. That separation
// matters: it lets you understand "what shape does a PyGo program take"
// completely independently from "how do we build that shape from
// tokens" (that's internal/parser's job).
package ast

import "github.com/manikyarathore/pygoc/internal/token"

// pos is embedded in every node so Pos() comes for free. Every node
// remembers where in the source it started, so later phases (type
// errors, semantic errors) can report precise line:col locations
// without threading position info through every function signature.
type Position struct {
	Line, Col int
}

func (p Position) Pos() (int, int) { return p.Line, p.Col }

// Node is the common interface for every AST node, expression or
// statement.
type Node interface {
	Pos() (line, col int)
}

// Expr is any node that produces a value: a literal, a variable
// reference, a binary operation, a function call, and so on.
type Expr interface {
	Node
	exprNode() // marker method — restricts implementers to this package's intent
}

// ---- Identifiers & literals -------------------------------------------------

// Identifier is a bare name reference, e.g. `x` in `x + 1`.
type Identifier struct {
	Position
	Name string
}

func (*Identifier) exprNode() {}

// IntLiteral is an integer constant, e.g. `42`.
type IntLiteral struct {
	Position
	Value int64
}

func (*IntLiteral) exprNode() {}

// FloatLiteral is a floating-point constant, e.g. `3.14`.
type FloatLiteral struct {
	Position
	Value float64
}

func (*FloatLiteral) exprNode() {}

// StringLiteral is a string constant, e.g. `"hello"`. Value holds the
// already-decoded string (escape sequences resolved by the lexer).
type StringLiteral struct {
	Position
	Value string
}

func (*StringLiteral) exprNode() {}

// BoolLiteral is `True` or `False`.
type BoolLiteral struct {
	Position
	Value bool
}

func (*BoolLiteral) exprNode() {}

// ListLiteral is `[1, 2, 3]`.
type ListLiteral struct {
	Position
	Elements []Expr
}

func (*ListLiteral) exprNode() {}

// DictLiteral is `{"a": 1, "b": 2}`. Keys[i] pairs with Values[i].
type DictLiteral struct {
	Position
	Keys   []Expr
	Values []Expr
}

func (*DictLiteral) exprNode() {}

// TupleLiteral is `(1, "x")`.
type TupleLiteral struct {
	Position
	Elements []Expr
}

func (*TupleLiteral) exprNode() {}

// ---- Operators -------------------------------------------------

// UnaryExpr is a prefix operator applied to one operand: `-x`, `not x`.
type UnaryExpr struct {
	Position
	Op      token.Type
	Operand Expr
}

func (*UnaryExpr) exprNode() {}

// BinaryExpr is a two-operand operator: `a + b`, `a and b`, `a == b`.
// Op is one of the operator token types (PLUS, MINUS, AND, EQ, ...).
type BinaryExpr struct {
	Position
	Op    token.Type
	Left  Expr
	Right Expr
}

func (*BinaryExpr) exprNode() {}

// TernaryExpr is PyGo's conditional expression: `x if cond else y`.
// (This is a stretch-tier feature per the language spec — the AST
// supports it, but the parser may reject it until that phase is
// explicitly enabled.)
type TernaryExpr struct {
	Position
	Cond Expr
	Then Expr
	Else Expr
}

func (*TernaryExpr) exprNode() {}

// ---- Calls, indexing, attribute access -------------------------------------------------

// CallExpr is a function or method call: `add(1, 2)`, `s.upper()`.
// For a method call, Callee is an AttributeExpr (`s.upper`) and Args
// holds the call's arguments (not including the receiver).
type CallExpr struct {
	Position
	Callee Expr
	Args   []Expr
}

func (*CallExpr) exprNode() {}

// IndexExpr is subscript access: `nums[0]`, `d["key"]`.
type IndexExpr struct {
	Position
	Object Expr
	Index  Expr
}

func (*IndexExpr) exprNode() {}

// SliceExpr is basic slicing: `nums[1:3]`. Low or High may be nil,
// meaning "from the start" / "to the end" respectively (`nums[:3]`,
// `nums[1:]`).
type SliceExpr struct {
	Position
	Object Expr
	Low    Expr // nil if omitted
	High   Expr // nil if omitted
}

func (*SliceExpr) exprNode() {}

// AttributeExpr is member access: `s.upper`, `nums.append`. On its own
// it's just a reference; wrapped in a CallExpr it becomes a method call.
type AttributeExpr struct {
	Position
	Object Expr
	Attr   string
}

func (*AttributeExpr) exprNode() {}
