package parser

// This file implements ONLY the expression grammar, via classic
// precedence climbing (a.k.a. operator-precedence parsing). Each
// function handles one precedence level and calls down into the next
// tighter-binding level, so precedence and associativity fall directly
// out of the call structure rather than needing an explicit precedence
// table:
//
//	parseExpr        (ternary — loosest binding)
//	  parseOr           or
//	    parseAnd           and
//	      parseNot           not (prefix)
//	        parseComparison    == != < > <= >=
//	          parseArith         + -
//	            parseTerm          * / % //
//	              parseUnary         - (prefix)
//	                parsePostfix       call / index / slice / attribute
//	                  parsePrimary       literals, identifiers, ( ), [ ], { }
//
// This shape is exactly the EBNF in docs/grammar.ebnf — each grammar
// rule maps to exactly one function here, so the two documents should
// always be read side by side when changing either.

import (
	"strconv"

	"github.com/manikyarathore/pygoc/internal/ast"
	"github.com/manikyarathore/pygoc/internal/token"
)

// parseExpr is the single entry point every other part of the parser
// calls to parse "an expression". It starts at the loosest-binding
// level (ternary) and lets precedence climbing handle the rest.
func (p *Parser) parseExpr() ast.Expr {
	return p.parseTernary()
}

// parseTernary handles `x if cond else y`. Right-associative: the else
// branch recurses back into parseTernary, so `a if c1 else b if c2 else
// c` groups as `a if c1 else (b if c2 else c)`, matching Python.
func (p *Parser) parseTernary() ast.Expr {
	thenExpr := p.parseOr()

	if p.check(token.IF) {
		tok := p.advance() // consume IF
		cond := p.parseOr()
		p.expect(token.ELSE, "'else' to complete ternary expression")
		elseExpr := p.parseTernary()
		return &ast.TernaryExpr{
			Position: ast.Position{Line: tok.Line, Col: tok.Col},
			Cond:     cond,
			Then:     thenExpr,
			Else:     elseExpr,
		}
	}
	return thenExpr
}

func (p *Parser) parseOr() ast.Expr {
	left := p.parseAnd()
	for p.check(token.OR) {
		tok := p.advance()
		right := p.parseAnd()
		left = &ast.BinaryExpr{Position: ast.Position{Line: tok.Line, Col: tok.Col}, Op: token.OR, Left: left, Right: right}
	}
	return left
}

func (p *Parser) parseAnd() ast.Expr {
	left := p.parseNot()
	for p.check(token.AND) {
		tok := p.advance()
		right := p.parseNot()
		left = &ast.BinaryExpr{Position: ast.Position{Line: tok.Line, Col: tok.Col}, Op: token.AND, Left: left, Right: right}
	}
	return left
}

// parseNot handles the boolean prefix operator `not x`. It's kept as
// its own precedence level (rather than folded into parseUnary)
// because in Python-family grammars `not` binds looser than comparison
// operators: `not x == y` means `not (x == y)`, not `(not x) == y`.
func (p *Parser) parseNot() ast.Expr {
	if p.check(token.NOT) {
		tok := p.advance()
		operand := p.parseNot() // right-associative: `not not x` works
		return &ast.UnaryExpr{Position: ast.Position{Line: tok.Line, Col: tok.Col}, Op: token.NOT, Operand: operand}
	}
	return p.parseComparison()
}

func (p *Parser) parseComparison() ast.Expr {
	left := p.parseArith()
	for p.check(token.EQ) || p.check(token.NEQ) || p.check(token.LT) ||
		p.check(token.GT) || p.check(token.LE) || p.check(token.GE) {
		tok := p.advance()
		right := p.parseArith()
		left = &ast.BinaryExpr{Position: ast.Position{Line: tok.Line, Col: tok.Col}, Op: tok.Type, Left: left, Right: right}
	}
	return left
}

func (p *Parser) parseArith() ast.Expr {
	left := p.parseTerm()
	for p.check(token.PLUS) || p.check(token.MINUS) {
		tok := p.advance()
		right := p.parseTerm()
		left = &ast.BinaryExpr{Position: ast.Position{Line: tok.Line, Col: tok.Col}, Op: tok.Type, Left: left, Right: right}
	}
	return left
}

func (p *Parser) parseTerm() ast.Expr {
	left := p.parseUnary()
	for p.check(token.STAR) || p.check(token.SLASH) || p.check(token.PERCENT) || p.check(token.DSLASH) {
		tok := p.advance()
		right := p.parseUnary()
		left = &ast.BinaryExpr{Position: ast.Position{Line: tok.Line, Col: tok.Col}, Op: tok.Type, Left: left, Right: right}
	}
	return left
}

// parseUnary handles arithmetic negation: `-x`. (Boolean `not` is its
// own precedence level above — see parseNot.)
func (p *Parser) parseUnary() ast.Expr {
	if p.check(token.MINUS) {
		tok := p.advance()
		operand := p.parseUnary() // right-associative: `--x` parses (though semantically odd, that's a semantic-phase concern, not a parse error)
		return &ast.UnaryExpr{Position: ast.Position{Line: tok.Line, Col: tok.Col}, Op: token.MINUS, Operand: operand}
	}
	return p.parsePostfix()
}

// parsePostfix handles the tightest-binding operators: function/method
// calls, indexing, slicing, and attribute access — all of which can
// chain arbitrarily, e.g. `nums.get(0).upper()[1:3]`.
func (p *Parser) parsePostfix() ast.Expr {
	expr := p.parsePrimary()

	for {
		switch {
		case p.check(token.LPAREN):
			tok := p.advance()
			args := p.parseArgList()
			expr = &ast.CallExpr{Position: ast.Position{Line: tok.Line, Col: tok.Col}, Callee: expr, Args: args}

		case p.check(token.LBRACKET):
			tok := p.advance()
			expr = p.parseIndexOrSlice(expr, tok)

		case p.check(token.DOT):
			p.advance()
			attrTok := p.expect(token.IDENT, "attribute or method name after '.'")
			expr = &ast.AttributeExpr{Position: ast.Position{Line: attrTok.Line, Col: attrTok.Col}, Object: expr, Attr: attrTok.Lexeme}

		default:
			return expr
		}
	}
}

// parseIndexOrSlice handles both `obj[i]` and `obj[low:high]` (with
// either bound optional: `obj[:3]`, `obj[1:]`, `obj[:]`). The opening
// '[' has already been consumed by the caller.
func (p *Parser) parseIndexOrSlice(object ast.Expr, openBracket token.Token) ast.Expr {
	pos := ast.Position{Line: openBracket.Line, Col: openBracket.Col}

	// `obj[:high]` — slice with an omitted low bound.
	if p.check(token.COLON) {
		p.advance()
		var high ast.Expr
		if !p.check(token.RBRACKET) {
			high = p.parseExpr()
		}
		p.expect(token.RBRACKET, "']' to close slice")
		return &ast.SliceExpr{Position: pos, Object: object, Low: nil, High: high}
	}

	first := p.parseExpr()

	if p.match(token.COLON) {
		var high ast.Expr
		if !p.check(token.RBRACKET) {
			high = p.parseExpr()
		}
		p.expect(token.RBRACKET, "']' to close slice")
		return &ast.SliceExpr{Position: pos, Object: object, Low: first, High: high}
	}

	p.expect(token.RBRACKET, "']' to close index")
	return &ast.IndexExpr{Position: pos, Object: object, Index: first}
}

// parseArgList parses zero or more comma-separated call arguments and
// consumes the closing ')'. The opening '(' is consumed by the caller.
func (p *Parser) parseArgList() []ast.Expr {
	var args []ast.Expr
	if !p.check(token.RPAREN) {
		for {
			args = append(args, p.parseExpr())
			if !p.match(token.COMMA) {
				break
			}
		}
	}
	p.expect(token.RPAREN, "')' to close argument list")
	return args
}

// parsePrimary handles the base cases of the expression grammar:
// literals, identifiers, parenthesized/tuple expressions, and list/dict
// literals.
func (p *Parser) parsePrimary() ast.Expr {
	tok := p.current()

	switch tok.Type {
	case token.INT:
		p.advance()
		v, err := strconv.ParseInt(tok.Lexeme, 10, 64)
		if err != nil {
			p.errorAt(tok, "invalid integer literal '"+tok.Lexeme+"'")
		}
		return &ast.IntLiteral{Position: ast.Position{Line: tok.Line, Col: tok.Col}, Value: v}

	case token.FLOAT:
		p.advance()
		v, err := strconv.ParseFloat(tok.Lexeme, 64)
		if err != nil {
			p.errorAt(tok, "invalid float literal '"+tok.Lexeme+"'")
		}
		return &ast.FloatLiteral{Position: ast.Position{Line: tok.Line, Col: tok.Col}, Value: v}

	case token.STRING:
		p.advance()
		s, _ := tok.Literal.(string)
		return &ast.StringLiteral{Position: ast.Position{Line: tok.Line, Col: tok.Col}, Value: s}

	case token.TRUE:
		p.advance()
		return &ast.BoolLiteral{Position: ast.Position{Line: tok.Line, Col: tok.Col}, Value: true}

	case token.FALSE:
		p.advance()
		return &ast.BoolLiteral{Position: ast.Position{Line: tok.Line, Col: tok.Col}, Value: false}

	case token.IDENT:
		p.advance()
		return &ast.Identifier{Position: ast.Position{Line: tok.Line, Col: tok.Col}, Name: tok.Lexeme}

	case token.LPAREN:
		return p.parseParenOrTuple()

	case token.LBRACKET:
		return p.parseListLiteral()

	case token.LBRACE:
		return p.parseDictLiteral()

	default:
		p.errorAt(tok, "unexpected token "+tok.Type.String()+" in expression")
		p.advance() // guarantee forward progress even on a malformed expression
		return &ast.Identifier{Position: ast.Position{Line: tok.Line, Col: tok.Col}, Name: "<error>"}
	}
}

// parseParenOrTuple handles both a plain parenthesized expression
// `(a + b)` and a tuple literal `(1, "x")`. Disambiguated by whether a
// comma follows the first inner expression.
func (p *Parser) parseParenOrTuple() ast.Expr {
	tok := p.advance() // consume '('
	first := p.parseExpr()

	if p.check(token.COMMA) {
		elements := []ast.Expr{first}
		for p.match(token.COMMA) {
			if p.check(token.RPAREN) { // allow a trailing comma: (1, 2,)
				break
			}
			elements = append(elements, p.parseExpr())
		}
		p.expect(token.RPAREN, "')' to close tuple")
		return &ast.TupleLiteral{Position: ast.Position{Line: tok.Line, Col: tok.Col}, Elements: elements}
	}

	p.expect(token.RPAREN, "')' to close parenthesized expression")
	return first
}

func (p *Parser) parseListLiteral() ast.Expr {
	tok := p.advance() // consume '['
	var elements []ast.Expr
	if !p.check(token.RBRACKET) {
		for {
			elements = append(elements, p.parseExpr())
			if !p.match(token.COMMA) {
				break
			}
			if p.check(token.RBRACKET) { // trailing comma: [1, 2,]
				break
			}
		}
	}
	p.expect(token.RBRACKET, "']' to close list literal")
	return &ast.ListLiteral{Position: ast.Position{Line: tok.Line, Col: tok.Col}, Elements: elements}
}

func (p *Parser) parseDictLiteral() ast.Expr {
	tok := p.advance() // consume '{'
	var keys, values []ast.Expr
	if !p.check(token.RBRACE) {
		for {
			k := p.parseExpr()
			p.expect(token.COLON, "':' between dict key and value")
			v := p.parseExpr()
			keys = append(keys, k)
			values = append(values, v)
			if !p.match(token.COMMA) {
				break
			}
			if p.check(token.RBRACE) { // trailing comma: {"a": 1,}
				break
			}
		}
	}
	p.expect(token.RBRACE, "'}' to close dict literal")
	return &ast.DictLiteral{Position: ast.Position{Line: tok.Line, Col: tok.Col}, Keys: keys, Values: values}
}
