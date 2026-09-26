package lexer

import "github.com/manikyarathore/pygoc/internal/token"

// This file owns ONE job: turning leading whitespace at the start of each
// logical line into INDENT / DEDENT tokens. Everything else about
// scanning (identifiers, numbers, operators...) lives in lexer.go.
//
// Algorithm (this is the same shape as CPython's own tokenizer):
//   1. Maintain a stack of indentation widths, starting with [0].
//   2. At the start of every non-blank, non-comment-only line, measure
//      how many leading spaces it has.
//   3. If that width is GREATER than the top of the stack: push it,
//      emit one INDENT token. A new, deeper block has begun.
//   4. If it's EQUAL: emit nothing. Same block level.
//   5. If it's LESS: pop the stack until the top matches, emitting one
//      DEDENT per pop. If we pop past a level and never land exactly on
//      a previously-seen width, that's an indentation error — the
//      programmer's dedent doesn't line up with any enclosing block.
//
// Deliberate simplification (documented, not accidental): PyGo requires
// SPACES for indentation, not tabs. Mixing tabs and spaces is a classic
// source of Python bugs; we simply reject tabs in leading whitespace
// with a clear diagnostic rather than trying to guess tab width.

const initialIndent = 0

// measureIndent counts leading spaces starting at l.pos (which must be
// at the beginning of a line) and returns the width plus whether a
// disallowed tab character was found.
func (l *Lexer) measureIndent() (width int, sawTab bool) {
	p := l.pos
	for p < len(l.src) {
		switch l.src[p] {
		case ' ':
			width++
			p++
		case '\t':
			sawTab = true
			p++
		default:
			return width, sawTab
		}
	}
	return width, sawTab
}

// isBlankOrCommentLine reports whether the line starting at byte offset
// `p` (after leading whitespace) contains nothing but a comment or ends
// immediately — such lines never affect indentation and never produce
// NEWLINE/INDENT/DEDENT tokens, exactly like in real Python.
func (l *Lexer) isBlankOrCommentLine(afterWhitespace int) bool {
	if afterWhitespace >= len(l.src) {
		return true
	}
	ch := l.src[afterWhitespace]
	return ch == '\n' || ch == '#'
}

// handleLineStart is called by Tokenize() whenever we're positioned at
// the first column of a new physical line AND we're not inside an open
// bracket (paren/bracket/brace nesting suppresses indentation logic,
// same as Python's implicit line continuation inside `( ... )`).
//
// Returns:
//
//	toks   - zero or more INDENT/DEDENT tokens to emit before the line's
//	         real content
//	skip   - true if this was a blank/comment-only line and the caller
//	         should just consume it and loop again
//	err    - non-nil on an indentation error (tabs used, or a dedent
//	         that doesn't match any enclosing block width)
func (l *Lexer) handleLineStart() (toks []token.Token, skip bool, err error) {
	startLine, startCol := l.line, 1
	width, sawTab := l.measureIndent()
	contentPos := l.pos + width

	if l.isBlankOrCommentLine(contentPos) {
		// Blank or comment-only line: consume it, emit nothing, don't
		// touch the indent stack at all.
		return nil, true, nil
	}

	if sawTab {
		return nil, false, &LexError{
			Line: startLine, Col: startCol,
			Msg: "tabs are not allowed in indentation; use spaces only",
		}
	}

	// Actually consume the whitespace we measured.
	l.advanceBy(width)

	top := l.indentStack[len(l.indentStack)-1]

	switch {
	case width > top:
		l.indentStack = append(l.indentStack, width)
		toks = append(toks, token.Token{Type: token.INDENT, Line: startLine, Col: 1})

	case width < top:
		for len(l.indentStack) > 0 && l.indentStack[len(l.indentStack)-1] > width {
			l.indentStack = l.indentStack[:len(l.indentStack)-1]
			toks = append(toks, token.Token{Type: token.DEDENT, Line: startLine, Col: 1})
		}
		if l.indentStack[len(l.indentStack)-1] != width {
			return nil, false, &LexError{
				Line: startLine, Col: startCol,
				Msg: "indentation does not match any enclosing block level",
			}
		}

	default:
		// width == top: same block, nothing to emit.
	}

	return toks, false, nil
}

// closingDedents is called once at EOF to pop every remaining
// indentation level, so every INDENT is guaranteed a matching DEDENT.
func (l *Lexer) closingDedents(line int) []token.Token {
	var toks []token.Token
	for len(l.indentStack) > 1 {
		l.indentStack = l.indentStack[:len(l.indentStack)-1]
		toks = append(toks, token.Token{Type: token.DEDENT, Line: line, Col: 1})
	}
	return toks
}
