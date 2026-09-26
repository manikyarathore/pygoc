package lexer

import (
	"testing"

	"github.com/manikyarathore/pygoc/internal/token"
)

func typesOf(toks []token.Token) []token.Type {
	out := make([]token.Type, len(toks))
	for i, t := range toks {
		out[i] = t.Type
	}
	return out
}

func assertTypes(t *testing.T, src string, want []token.Type) {
	t.Helper()
	toks, errs := New(src).Tokenize()
	if len(errs) != 0 {
		t.Fatalf("unexpected lex errors: %v", errs)
	}
	got := typesOf(toks)
	if len(got) != len(want) {
		t.Fatalf("token count mismatch\n got:  %v\n want: %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("token %d mismatch: got %v want %v\nfull got:  %v\nfull want: %v",
				i, got[i], want[i], got, want)
		}
	}
}

func TestSimpleAssignment(t *testing.T) {
	assertTypes(t, "x = 10\n", []token.Type{
		token.IDENT, token.ASSIGN, token.INT, token.NEWLINE, token.EOF,
	})
}

func TestArithmeticExpression(t *testing.T) {
	assertTypes(t, "y = a + b * 2 - c // 3\n", []token.Type{
		token.IDENT, token.ASSIGN, token.IDENT, token.PLUS, token.IDENT,
		token.STAR, token.INT, token.MINUS, token.IDENT, token.DSLASH,
		token.INT, token.NEWLINE, token.EOF,
	})
}

func TestFloatLiteral(t *testing.T) {
	assertTypes(t, "pi = 3.14\n", []token.Type{
		token.IDENT, token.ASSIGN, token.FLOAT, token.NEWLINE, token.EOF,
	})
}

func TestStringLiteralAndEscapes(t *testing.T) {
	toks, errs := New(`s = "hello\nworld"` + "\n").Tokenize()
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	var str token.Token
	found := false
	for _, tk := range toks {
		if tk.Type == token.STRING {
			str = tk
			found = true
		}
	}
	if !found {
		t.Fatalf("no STRING token produced")
	}
	if str.Literal.(string) != "hello\nworld" {
		t.Fatalf("escape sequence not decoded correctly: got %q", str.Literal)
	}
}

func TestUnterminatedStringIsError(t *testing.T) {
	_, errs := New(`s = "oops` + "\n").Tokenize()
	if len(errs) == 0 {
		t.Fatalf("expected an error for unterminated string, got none")
	}
}

func TestKeywordsRecognized(t *testing.T) {
	assertTypes(t, "if x == 1: return True\n", []token.Type{
		token.IF, token.IDENT, token.EQ, token.INT, token.COLON,
		token.RETURN, token.TRUE, token.NEWLINE, token.EOF,
	})
}

func TestSimpleIfBlockProducesIndentDedent(t *testing.T) {
	src := "if x:\n    y = 1\nz = 2\n"
	assertTypes(t, src, []token.Type{
		token.IF, token.IDENT, token.COLON, token.NEWLINE,
		token.INDENT,
		token.IDENT, token.ASSIGN, token.INT, token.NEWLINE,
		token.DEDENT,
		token.IDENT, token.ASSIGN, token.INT, token.NEWLINE,
		token.EOF,
	})
}

func TestNestedBlocksProduceMultipleDedents(t *testing.T) {
	src := "" +
		"if a:\n" +
		"    if b:\n" +
		"        x = 1\n" +
		"y = 2\n"
	assertTypes(t, src, []token.Type{
		token.IF, token.IDENT, token.COLON, token.NEWLINE,
		token.INDENT,
		token.IF, token.IDENT, token.COLON, token.NEWLINE,
		token.INDENT,
		token.IDENT, token.ASSIGN, token.INT, token.NEWLINE,
		token.DEDENT,
		token.DEDENT,
		token.IDENT, token.ASSIGN, token.INT, token.NEWLINE,
		token.EOF,
	})
}

func TestBlankAndCommentLinesDoNotAffectIndentation(t *testing.T) {
	src := "" +
		"if a:\n" +
		"    x = 1\n" +
		"\n" +
		"    # a comment\n" +
		"    y = 2\n" +
		"z = 3\n"
	assertTypes(t, src, []token.Type{
		token.IF, token.IDENT, token.COLON, token.NEWLINE,
		token.INDENT,
		token.IDENT, token.ASSIGN, token.INT, token.NEWLINE,
		token.IDENT, token.ASSIGN, token.INT, token.NEWLINE,
		token.DEDENT,
		token.IDENT, token.ASSIGN, token.INT, token.NEWLINE,
		token.EOF,
	})
}

func TestTabsInIndentationIsError(t *testing.T) {
	src := "if a:\n\tx = 1\n"
	_, errs := New(src).Tokenize()
	if len(errs) == 0 {
		t.Fatalf("expected an error for tab-based indentation, got none")
	}
}

func TestMismatchedDedentIsError(t *testing.T) {
	src := "if a:\n    if b:\n        x = 1\n      y = 2\n"
	_, errs := New(src).Tokenize()
	if len(errs) == 0 {
		t.Fatalf("expected an indentation-mismatch error, got none")
	}
}

func TestParenthesesSuppressNewlineAndIndent(t *testing.T) {
	src := "add(\n    1,\n    2\n)\n"
	assertTypes(t, src, []token.Type{
		token.IDENT, token.LPAREN,
		token.INT, token.COMMA,
		token.INT,
		token.RPAREN, token.NEWLINE, token.EOF,
	})
}

func TestFunctionDefWithArrowAndMultipleReturn(t *testing.T) {
	src := "def f(a: int, b: int) -> int:\n    return a + b\n"
	assertTypes(t, src, []token.Type{
		token.DEF, token.IDENT, token.LPAREN,
		token.IDENT, token.COLON, token.IDENT, token.COMMA,
		token.IDENT, token.COLON, token.IDENT,
		token.RPAREN, token.ARROW, token.IDENT, token.COLON, token.NEWLINE,
		token.INDENT,
		token.RETURN, token.IDENT, token.PLUS, token.IDENT, token.NEWLINE,
		token.DEDENT,
		token.EOF,
	})
}

func TestCompoundAssignmentOperators(t *testing.T) {
	assertTypes(t, "x += 1\ny -= 2\nz *= 3\nw /= 4\n", []token.Type{
		token.IDENT, token.PLUSEQ, token.INT, token.NEWLINE,
		token.IDENT, token.MINUSEQ, token.INT, token.NEWLINE,
		token.IDENT, token.STAREQ, token.INT, token.NEWLINE,
		token.IDENT, token.SLASHEQ, token.INT, token.NEWLINE,
		token.EOF,
	})
}

func TestNoTrailingNewlineStillClosesLineAndBlocks(t *testing.T) {
	src := "if a:\n    x = 1"
	assertTypes(t, src, []token.Type{
		token.IF, token.IDENT, token.COLON, token.NEWLINE,
		token.INDENT,
		token.IDENT, token.ASSIGN, token.INT, token.NEWLINE,
		token.DEDENT,
		token.EOF,
	})
}

func TestIllegalCharacterRecoversAndContinues(t *testing.T) {
	toks, errs := New("x = $\ny = 1\n").Tokenize()
	if len(errs) == 0 {
		t.Fatalf("expected an error for illegal character '$'")
	}
	got := typesOf(toks)
	want := []token.Type{
		token.IDENT, token.ASSIGN, token.NEWLINE,
		token.IDENT, token.ASSIGN, token.INT, token.NEWLINE,
		token.EOF,
	}
	if len(got) != len(want) {
		t.Fatalf("lexer did not recover correctly\n got:  %v\n want: %v", got, want)
	}
}
