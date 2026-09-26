// Package parser turns a PyGo token stream into an AST.
//
// This is Phase 2 of the pipeline:
//
//	[]token.Token  --[parser]-->  *ast.Program  --[symbol/types/semantic]--> ...
//
// Like the lexer, this package is deliberately split by concern:
//   - parser.go       (this file) the Parser struct, low-level token
//     helpers, and every STATEMENT-level grammar rule (if/while/for/def/
//     return/assignment), plus type-annotation and parameter parsing
//     since those only ever appear attached to a function declaration.
//   - expr_parser.go  the EXPRESSION grammar only (precedence climbing),
//     isolated because it's a genuinely separate concern from statement
//     structure and is easier to reason about on its own.
//   - recovery.go     panic-mode error recovery, shared by both.
package parser

import (
	"fmt"

	"github.com/manikyarathore/pygoc/internal/ast"
	"github.com/manikyarathore/pygoc/internal/token"
)

// Parser holds all parsing state: the full token slice (produced by the
// lexer) and a cursor into it. Like the lexer, we work over the whole
// token slice rather than streaming — PyGo source files are small enough
// that this isn't a real cost, and it makes lookahead trivial.
type Parser struct {
	tokens []token.Token
	pos    int // index of the current (not-yet-consumed) token
	errors []error
}

// New creates a Parser over a token stream (as produced by
// lexer.Tokenize()).
func New(tokens []token.Token) *Parser {
	return &Parser{tokens: tokens, pos: 0}
}

// ParseProgram is the entry point: parses the entire token stream into
// an *ast.Program, returning every syntax error encountered along the
// way (parsing continues past errors — see recovery.go).
func (p *Parser) ParseProgram() (*ast.Program, []error) {
	var stmts []ast.Stmt
	for !p.isAtEnd() {
		p.skipStrayNewlines()
		if p.isAtEnd() {
			break
		}
		stmts = append(stmts, p.parseStatementGuarded())
	}
	return &ast.Program{Statements: stmts}, p.errors
}

// ---- low-level token helpers -------------------------------------------------

func (p *Parser) current() token.Token {
	if p.pos >= len(p.tokens) {
		// Defensive fallback — should never happen, since the lexer
		// always emits a trailing EOF token, but guards against an
		// out-of-bounds panic if that invariant is ever broken.
		return token.Token{Type: token.EOF}
	}
	return p.tokens[p.pos]
}

func (p *Parser) previous() token.Token {
	if p.pos == 0 {
		return token.Token{}
	}
	return p.tokens[p.pos-1]
}

func (p *Parser) isAtEnd() bool {
	return p.current().Type == token.EOF
}

func (p *Parser) advance() token.Token {
	tok := p.current()
	if !p.isAtEnd() {
		p.pos++
	}
	return tok
}

func (p *Parser) check(tt token.Type) bool {
	return p.current().Type == tt
}

// match advances and returns true if the current token is one of the
// given types; otherwise it leaves the cursor untouched and returns
// false.
func (p *Parser) match(types ...token.Type) bool {
	for _, tt := range types {
		if p.check(tt) {
			p.advance()
			return true
		}
	}
	return false
}

// expect consumes the current token if it matches tt, or records a
// syntax error if not.
//
// Design decision (deliberate simplification, documented rather than
// accidental): expect() ALWAYS advances past the current token, even on
// a mismatch. This guarantees every call makes forward progress, which
// is the single most important property for avoiding an infinite loop
// on malformed input — we hit exactly this class of bug in the lexer's
// EOF handling earlier in the project, so every parsing function here is
// held to the same "must always progress" standard. The tradeoff is that
// a missing token can occasionally cause one extra cascading error
// before synchronize() re-aligns at the next statement boundary — an
// acceptable cost for guaranteed termination.
func (p *Parser) expect(tt token.Type, what string) token.Token {
	if p.check(tt) {
		return p.advance()
	}
	tok := p.current()
	p.errorAt(tok, fmt.Sprintf("expected %s, but got %s", what, tok.Type))
	return p.advance() // consume the unexpected token anyway — guarantees progress
}

// skipStrayNewlines defensively skips any NEWLINE tokens sitting at the
// current position. In well-formed input this should rarely fire (the
// lexer already collapses blank/comment-only lines to nothing), but it
// protects the statement-dispatch loops from stalling if a NEWLINE ever
// ends up here after error recovery skips past a malformed line.
func (p *Parser) skipStrayNewlines() {
	for p.check(token.NEWLINE) {
		p.advance()
	}
}

// parseStatementGuarded wraps parseStatement with two safety nets:
//  1. if parsing this statement produced any new errors, resynchronize
//     afterward so the next statement starts clean;
//  2. if, despite everything, the cursor didn't move at all (a bug we
//     haven't anticipated), force one token of progress rather than
//     hang forever. This mirrors the lexer's own EOF-ordering lesson:
//     never trust a loop to terminate without an explicit guarantee.
func (p *Parser) parseStatementGuarded() ast.Stmt {
	startPos := p.pos
	startErrs := len(p.errors)

	stmt := p.parseStatement()

	if len(p.errors) > startErrs {
		p.synchronize()
	}
	if p.pos == startPos {
		p.advance()
	}
	return stmt
}

// ---- statement dispatch -------------------------------------------------

func (p *Parser) parseStatement() ast.Stmt {
	switch p.current().Type {
	case token.IF:
		return p.parseIfStmt()
	case token.WHILE:
		return p.parseWhileStmt()
	case token.FOR:
		return p.parseForStmt()
	case token.DEF:
		return p.parseFuncDecl()
	case token.RETURN:
		return p.parseReturnStmt()
	case token.BREAK:
		return p.parseBreakStmt()
	case token.CONTINUE:
		return p.parseContinueStmt()
	default:
		return p.parseSimpleStmt()
	}
}

// parseBlock parses `: NEWLINE INDENT statement+ DEDENT` — the body of
// any compound statement (if/elif/else/while/for/def).
func (p *Parser) parseBlock() []ast.Stmt {
	p.expect(token.COLON, "':' to start a block")
	p.expect(token.NEWLINE, "newline after ':'")
	p.expect(token.INDENT, "an indented block")

	var stmts []ast.Stmt
	for !p.check(token.DEDENT) && !p.isAtEnd() {
		p.skipStrayNewlines()
		if p.check(token.DEDENT) || p.isAtEnd() {
			break
		}
		stmts = append(stmts, p.parseStatementGuarded())
	}
	p.expect(token.DEDENT, "end of indented block")
	return stmts
}

// ---- simple statements -------------------------------------------------

// parseSimpleStmt handles both plain expression statements (`print(x)`)
// and assignment statements (`x = 1`, `x += 1`, `a, b = pair`).
//
// The approach: parse one expression, then keep collecting more
// comma-separated expressions (these are candidate assignment targets
// for tuple unpacking). If an assignment operator follows, this is an
// AssignStmt. Otherwise, if we collected more than one expression it's a
// bare tuple expression statement; if just one, it's a plain ExprStmt.
func (p *Parser) parseSimpleStmt() ast.Stmt {
	first := p.parseExpr()
	line, col := first.Pos()

	targets := []ast.Expr{first}
	for p.match(token.COMMA) {
		targets = append(targets, p.parseExpr())
	}

	if p.match(token.ASSIGN, token.PLUSEQ, token.MINUSEQ, token.STAREQ, token.SLASHEQ) {
		op := p.previous().Type
		value := p.parseExpr()
		p.expect(token.NEWLINE, "newline after assignment")
		return &ast.AssignStmt{
			Position: ast.Position{Line: line, Col: col},
			Targets:  targets,
			Op:       op,
			Value:    value,
		}
	}

	if len(targets) > 1 {
		tup := &ast.TupleLiteral{Position: ast.Position{Line: line, Col: col}, Elements: targets}
		p.expect(token.NEWLINE, "newline after expression")
		return &ast.ExprStmt{Position: ast.Position{Line: line, Col: col}, X: tup}
	}

	p.expect(token.NEWLINE, "newline after expression")
	return &ast.ExprStmt{Position: ast.Position{Line: line, Col: col}, X: first}
}

func (p *Parser) parseReturnStmt() ast.Stmt {
	tok := p.advance() // consume RETURN
	var values []ast.Expr
	if !p.check(token.NEWLINE) {
		values = append(values, p.parseExpr())
		for p.match(token.COMMA) {
			values = append(values, p.parseExpr())
		}
	}
	p.expect(token.NEWLINE, "newline after return")
	return &ast.ReturnStmt{Position: ast.Position{Line: tok.Line, Col: tok.Col}, Values: values}
}

func (p *Parser) parseBreakStmt() ast.Stmt {
	tok := p.advance() // consume BREAK
	p.expect(token.NEWLINE, "newline after break")
	return &ast.BreakStmt{Position: ast.Position{Line: tok.Line, Col: tok.Col}}
}

func (p *Parser) parseContinueStmt() ast.Stmt {
	tok := p.advance() // consume CONTINUE
	p.expect(token.NEWLINE, "newline after continue")
	return &ast.ContinueStmt{Position: ast.Position{Line: tok.Line, Col: tok.Col}}
}

// ---- compound statements -------------------------------------------------

func (p *Parser) parseIfStmt() ast.Stmt {
	tok := p.advance() // consume IF
	cond := p.parseExpr()
	then := p.parseBlock()

	var elifs []ast.ElifClause
	for p.check(token.ELIF) {
		elifTok := p.advance()
		econd := p.parseExpr()
		ebody := p.parseBlock()
		elifs = append(elifs, ast.ElifClause{
			Position: ast.Position{Line: elifTok.Line, Col: elifTok.Col},
			Cond:     econd,
			Body:     ebody,
		})
	}

	var elseBody []ast.Stmt
	if p.match(token.ELSE) {
		elseBody = p.parseBlock()
	}

	return &ast.IfStmt{
		Position: ast.Position{Line: tok.Line, Col: tok.Col},
		Cond:     cond,
		Then:     then,
		Elifs:    elifs,
		Else:     elseBody,
	}
}

func (p *Parser) parseWhileStmt() ast.Stmt {
	tok := p.advance() // consume WHILE
	cond := p.parseExpr()
	body := p.parseBlock()
	return &ast.WhileStmt{Position: ast.Position{Line: tok.Line, Col: tok.Col}, Cond: cond, Body: body}
}

// parseForStmt handles PyGo's restricted for-loop form:
//
//	for x in range(n):        # Start=n,    Stop=nil  -> iterates 0..n
//	for x in range(a, b):     # Start=a,    Stop=b    -> iterates a..b
func (p *Parser) parseForStmt() ast.Stmt {
	tok := p.advance() // consume FOR
	nameTok := p.expect(token.IDENT, "loop variable name")
	p.expect(token.IN, "'in'")
	p.expect(token.RANGE, "'range'")
	p.expect(token.LPAREN, "'(' after range")

	start := p.parseExpr()
	var stop ast.Expr
	if p.match(token.COMMA) {
		stop = p.parseExpr()
	}
	p.expect(token.RPAREN, "')' to close range(...)")

	body := p.parseBlock()
	return &ast.ForStmt{
		Position: ast.Position{Line: tok.Line, Col: tok.Col},
		VarName:  nameTok.Lexeme,
		Start:    start,
		Stop:     stop,
		Body:     body,
	}
}

func (p *Parser) parseFuncDecl() ast.Stmt {
	tok := p.advance() // consume DEF
	nameTok := p.expect(token.IDENT, "function name")
	p.expect(token.LPAREN, "'(' after function name")
	params := p.parseParamList()

	var returnTypes []*ast.TypeExpr
	if p.match(token.ARROW) {
		returnTypes = p.parseReturnTypeAnnotation()
	}

	body := p.parseBlock()
	return &ast.FuncDecl{
		Position:    ast.Position{Line: tok.Line, Col: tok.Col},
		Name:        nameTok.Lexeme,
		Params:      params,
		ReturnTypes: returnTypes,
		Body:        body,
	}
}

// ---- parameters & type annotations -------------------------------------------------
// (Grouped here, not in expr_parser.go, because they only ever appear
// as part of a function declaration — this is statement-shape syntax,
// not expression syntax.)

// parseParamList parses zero or more `name: Type` parameters and
// consumes the closing ')'. The opening '(' is consumed by the caller
// (parseFuncDecl) before this is called.
func (p *Parser) parseParamList() []ast.Param {
	var params []ast.Param
	if !p.check(token.RPAREN) {
		for {
			nameTok := p.expect(token.IDENT, "parameter name")
			p.expect(token.COLON, "':' before parameter type")
			typ := p.parseTypeExpr()
			params = append(params, ast.Param{
				Position: ast.Position{Line: nameTok.Line, Col: nameTok.Col},
				Name:     nameTok.Lexeme,
				Type:     typ,
			})
			if !p.match(token.COMMA) {
				break
			}
		}
	}
	p.expect(token.RPAREN, "')' to close parameter list")
	return params
}

// parseReturnTypeAnnotation handles both `-> int` (single return value)
// and `-> (int, int)` (multiple return values). The '->' itself is
// consumed by the caller.
func (p *Parser) parseReturnTypeAnnotation() []*ast.TypeExpr {
	if p.match(token.LPAREN) {
		var types []*ast.TypeExpr
		if !p.check(token.RPAREN) {
			for {
				types = append(types, p.parseTypeExpr())
				if !p.match(token.COMMA) {
					break
				}
			}
		}
		p.expect(token.RPAREN, "')' to close return type list")
		return types
	}
	return []*ast.TypeExpr{p.parseTypeExpr()}
}

// parseTypeExpr parses a type annotation: `int`, `list[int]`,
// `dict[string, int]`.
func (p *Parser) parseTypeExpr() *ast.TypeExpr {
	nameTok := p.expect(token.IDENT, "a type name")
	t := &ast.TypeExpr{
		Position: ast.Position{Line: nameTok.Line, Col: nameTok.Col},
		Name:     nameTok.Lexeme,
	}
	if p.match(token.LBRACKET) {
		for {
			t.Params = append(t.Params, p.parseTypeExpr())
			if !p.match(token.COMMA) {
				break
			}
		}
		p.expect(token.RBRACKET, "']' to close parameterized type")
	}
	return t
}
