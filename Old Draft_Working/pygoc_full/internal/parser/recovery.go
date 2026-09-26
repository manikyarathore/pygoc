package parser

import (
	"fmt"

	"github.com/manikyarathore/pygoc/internal/token"
)

// ParseError is a syntax error with source position, so the CLI can
// print "file.py:4:9: expected ':' to start block" style messages.
type ParseError struct {
	Line, Col int
	Msg       string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("%d:%d: %s", e.Line, e.Col, e.Msg)
}

// errorAt records a syntax error without stopping the parse — this is
// the parser-level half of the compiler's overall error-recovery
// philosophy (see internal/lexer's Tokenize() for the lexer-level half),
// so a single `pygoc compile` run can report every syntax error in a
// file, not just the first.
func (p *Parser) errorAt(tok token.Token, msg string) {
	p.errors = append(p.errors, &ParseError{Line: tok.Line, Col: tok.Col, Msg: msg})
}

// synchronize implements panic-mode recovery: after a statement fails to
// parse cleanly, skip tokens until we're at a position where resuming
// parsing is likely to produce sensible results again — specifically,
// right after a NEWLINE (a fresh statement boundary) or at a token that
// clearly begins a new statement (an "if", "def", "return", etc. can't
// legally appear mid-expression, so seeing one means a new statement is
// starting here regardless of how the previous one went wrong).
//
// This is the same technique real compilers use (gcc and clang both do
// panic-mode recovery at statement boundaries) — it trades perfect
// re-synchronization for something simple and reliable: worst case, we
// under-recover and report a few extra spurious errors on this bad
// statement, but we never fail to make progress or crash.
func (p *Parser) synchronize() {
	for !p.isAtEnd() {
		if p.previous().Type == token.NEWLINE {
			return
		}
		switch p.current().Type {
		case token.IF, token.WHILE, token.FOR, token.DEF,
			token.RETURN, token.BREAK, token.CONTINUE:
			return
		}
		p.advance()
	}
}
