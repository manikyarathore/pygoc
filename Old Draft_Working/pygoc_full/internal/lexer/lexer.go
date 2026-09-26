// Package lexer converts PyGo source text into a stream of tokens.
//
// This is Phase 1 of the pipeline:
//
//	source (.py)  --[lexer]-->  []token.Token  --[parser]-->  AST
//
// The lexer is deliberately split into two files:
//   - indent.go   owns indentation (INDENT/DEDENT) bookkeeping only.
//   - lexer.go    (this file) owns everything else: identifiers,
//     numbers, strings, operators, delimiters, comments, and the
//     top-level orchestration loop that ties both together.
package lexer

import (
	"fmt"
	"strings"

	"github.com/manikyarathore/pygoc/internal/token"
)

// LexError is a lexical error with source position, so the CLI can print
// "file.py:4:9: tabs are not allowed in indentation" style messages.
type LexError struct {
	Line, Col int
	Msg       string
}

func (e *LexError) Error() string {
	return fmt.Sprintf("%d:%d: %s", e.Line, e.Col, e.Msg)
}

// Lexer holds all scanning state. Source is read entirely into memory
// and converted to a rune slice up front — PyGo source files are small
// enough that streaming isn't worth the complexity it would add.
type Lexer struct {
	src  []rune
	pos  int // index of the next unread rune in src
	line int
	col  int

	indentStack []int
	parenDepth  int // >0 means we're inside ( ) [ ] { } — newlines don't
	// terminate a logical line while this is true (matches Python's
	// implicit continuation rule inside brackets).
}

// New creates a Lexer over the given source text.
func New(src string) *Lexer {
	return &Lexer{
		src:         []rune(src),
		pos:         0,
		line:        1,
		col:         1,
		indentStack: []int{initialIndent},
	}
}

// Tokenize runs the full scan and returns every token, plus any lexical
// errors encountered. Scanning continues after an error (this is the
// lexer-level half of the compiler's overall error-recovery philosophy —
// see internal/parser/recovery.go for the parser-level half) so a single
// run can report every problem in the file, not just the first.
func (l *Lexer) Tokenize() ([]token.Token, []error) {
	var tokens []token.Token
	var errs []error

	atLineStart := true

	for {
		// CRITICAL: the EOF check must run BEFORE the indentation check
		// on every iteration. If it ran after, a file ending exactly at
		// a line boundary (atLineStart==true, pos==len(src) — i.e. the
		// completely normal case of a file ending in "\n") would make
		// handleLineStart() report "blank line, skip" forever, since
		// skipToNextLine() has nothing left to consume and pos never
		// advances. Checking EOF first guarantees termination.
		if l.pos >= len(l.src) {
			// If the last line had content but no trailing newline,
			// close it out with a synthetic NEWLINE first, so the
			// parser never has to special-case "last statement".
			if !atLineStart {
				tokens = append(tokens, token.Token{Type: token.NEWLINE, Line: l.line, Col: l.col})
			}
			tokens = append(tokens, l.closingDedents(l.line)...)
			tokens = append(tokens, token.Token{Type: token.EOF, Line: l.line, Col: l.col})
			break
		}

		if atLineStart && l.parenDepth == 0 {
			toks, skip, err := l.handleLineStart()
			if err != nil {
				errs = append(errs, err)
				// Recovery: skip to the next physical newline and retry,
				// so one bad line doesn't cascade into false errors on
				// every line after it.
				l.skipToNextLine()
				continue
			}
			tokens = append(tokens, toks...)
			if skip {
				l.skipToNextLine()
				continue
			}
			atLineStart = false
		}

		ch := l.src[l.pos]

		switch {
		case ch == '\n':
			l.advance()
			if l.parenDepth == 0 {
				tokens = append(tokens, token.Token{Type: token.NEWLINE, Line: l.line - 1, Col: l.col})
				atLineStart = true
			}
			// inside brackets: newline is just whitespace, ignored

		case ch == ' ' || ch == '\t':
			l.advance()

		case ch == '#':
			l.skipComment()

		case ch == '"':
			tok, err := l.scanString()
			if err != nil {
				errs = append(errs, err)
			} else {
				tokens = append(tokens, tok)
			}

		case isDigit(ch):
			tokens = append(tokens, l.scanNumber())

		case isIdentStart(ch):
			tokens = append(tokens, l.scanIdentifier())

		default:
			// NOTE: scanOperator() always consumes the rune it looked at
			// via its own l.advance() call, even on its error path — so
			// we must NOT advance again here. Doing so would silently
			// eat one extra character (e.g. the newline right after an
			// illegal char), corrupting line tracking and dropping a
			// NEWLINE token further down.
			tok, err := l.scanOperator()
			if err != nil {
				errs = append(errs, err)
			} else {
				tokens = append(tokens, tok)
			}
		}
	}

	return tokens, errs
}

// ---- low-level rune helpers -------------------------------------------------

func (l *Lexer) advance() rune {
	ch := l.src[l.pos]
	l.pos++
	if ch == '\n' {
		l.line++
		l.col = 1
	} else {
		l.col++
	}
	return ch
}

func (l *Lexer) advanceBy(n int) {
	for i := 0; i < n; i++ {
		l.advance()
	}
}

func (l *Lexer) peek() rune {
	if l.pos >= len(l.src) {
		return 0
	}
	return l.src[l.pos]
}

func (l *Lexer) peekNext() rune {
	if l.pos+1 >= len(l.src) {
		return 0
	}
	return l.src[l.pos+1]
}

func (l *Lexer) skipToNextLine() {
	for l.pos < len(l.src) && l.src[l.pos] != '\n' {
		l.advance()
	}
	if l.pos < len(l.src) {
		l.advance() // consume the newline itself
	}
}

func (l *Lexer) skipComment() {
	for l.pos < len(l.src) && l.src[l.pos] != '\n' {
		l.advance()
	}
	// the '\n' itself is left for the main loop to handle, so NEWLINE
	// emission stays in exactly one place.
}

// ---- character classes -------------------------------------------------

func isDigit(ch rune) bool      { return ch >= '0' && ch <= '9' }
func isIdentStart(ch rune) bool { return ch == '_' || (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') }
func isIdentCont(ch rune) bool  { return isIdentStart(ch) || isDigit(ch) }

// ---- literal scanners -------------------------------------------------

func (l *Lexer) scanNumber() token.Token {
	startLine, startCol := l.line, l.col
	var sb strings.Builder
	isFloat := false

	for isDigit(l.peek()) {
		sb.WriteRune(l.advance())
	}
	if l.peek() == '.' && isDigit(l.peekNext()) {
		isFloat = true
		sb.WriteRune(l.advance()) // consume '.'
		for isDigit(l.peek()) {
			sb.WriteRune(l.advance())
		}
	}

	lexeme := sb.String()
	tt := token.INT
	if isFloat {
		tt = token.FLOAT
	}
	return token.Token{Type: tt, Lexeme: lexeme, Line: startLine, Col: startCol}
}

func (l *Lexer) scanIdentifier() token.Token {
	startLine, startCol := l.line, l.col
	var sb strings.Builder
	for isIdentCont(l.peek()) {
		sb.WriteRune(l.advance())
	}
	lexeme := sb.String()
	return token.Token{Type: token.LookupIdent(lexeme), Lexeme: lexeme, Line: startLine, Col: startCol}
}

func (l *Lexer) scanString() (token.Token, error) {
	startLine, startCol := l.line, l.col
	l.advance() // opening quote
	var sb strings.Builder

	for {
		if l.pos >= len(l.src) || l.peek() == '\n' {
			return token.Token{}, &LexError{Line: startLine, Col: startCol, Msg: "unterminated string literal"}
		}
		ch := l.peek()
		if ch == '"' {
			l.advance() // closing quote
			break
		}
		if ch == '\\' {
			l.advance()
			esc := l.advance()
			switch esc {
			case 'n':
				sb.WriteRune('\n')
			case 't':
				sb.WriteRune('\t')
			case '"':
				sb.WriteRune('"')
			case '\\':
				sb.WriteRune('\\')
			default:
				return token.Token{}, &LexError{Line: l.line, Col: l.col, Msg: fmt.Sprintf("unknown escape sequence '\\%c'", esc)}
			}
			continue
		}
		sb.WriteRune(l.advance())
	}

	return token.Token{Type: token.STRING, Lexeme: sb.String(), Literal: sb.String(), Line: startLine, Col: startCol}, nil
}

// ---- operators & delimiters -------------------------------------------------

func (l *Lexer) scanOperator() (token.Token, error) {
	startLine, startCol := l.line, l.col
	ch := l.advance()

	two := func(next rune, twoType, oneType token.Type) token.Token {
		if l.peek() == next {
			l.advance()
			return token.Token{Type: twoType, Line: startLine, Col: startCol}
		}
		return token.Token{Type: oneType, Line: startLine, Col: startCol}
	}

	switch ch {
	case '+':
		return two('=', token.PLUSEQ, token.PLUS), nil
	case '-':
		if l.peek() == '>' {
			l.advance()
			return token.Token{Type: token.ARROW, Line: startLine, Col: startCol}, nil
		}
		return two('=', token.MINUSEQ, token.MINUS), nil
	case '*':
		return two('=', token.STAREQ, token.STAR), nil
	case '/':
		if l.peek() == '/' {
			l.advance()
			return token.Token{Type: token.DSLASH, Line: startLine, Col: startCol}, nil
		}
		return two('=', token.SLASHEQ, token.SLASH), nil
	case '%':
		return token.Token{Type: token.PERCENT, Line: startLine, Col: startCol}, nil
	case '=':
		return two('=', token.EQ, token.ASSIGN), nil
	case '!':
		if l.peek() == '=' {
			l.advance()
			return token.Token{Type: token.NEQ, Line: startLine, Col: startCol}, nil
		}
		return token.Token{}, &LexError{Line: startLine, Col: startCol, Msg: "unexpected character '!' (did you mean '!='?)"}
	case '<':
		return two('=', token.LE, token.LT), nil
	case '>':
		return two('=', token.GE, token.GT), nil
	case '(':
		l.parenDepth++
		return token.Token{Type: token.LPAREN, Line: startLine, Col: startCol}, nil
	case ')':
		l.parenDepth--
		return token.Token{Type: token.RPAREN, Line: startLine, Col: startCol}, nil
	case '[':
		l.parenDepth++
		return token.Token{Type: token.LBRACKET, Line: startLine, Col: startCol}, nil
	case ']':
		l.parenDepth--
		return token.Token{Type: token.RBRACKET, Line: startLine, Col: startCol}, nil
	case '{':
		l.parenDepth++
		return token.Token{Type: token.LBRACE, Line: startLine, Col: startCol}, nil
	case '}':
		l.parenDepth--
		return token.Token{Type: token.RBRACE, Line: startLine, Col: startCol}, nil
	case ',':
		return token.Token{Type: token.COMMA, Line: startLine, Col: startCol}, nil
	case ':':
		return token.Token{Type: token.COLON, Line: startLine, Col: startCol}, nil
	case '.':
		return token.Token{Type: token.DOT, Line: startLine, Col: startCol}, nil
	default:
		return token.Token{}, &LexError{Line: startLine, Col: startCol, Msg: fmt.Sprintf("unexpected character %q", ch)}
	}
}
