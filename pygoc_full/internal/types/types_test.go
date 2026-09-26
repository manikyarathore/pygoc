package types

import (
	"testing"

	"github.com/manikyarathore/pygoc/internal/ast"
	"github.com/manikyarathore/pygoc/internal/lexer"
	"github.com/manikyarathore/pygoc/internal/parser"
	"github.com/manikyarathore/pygoc/internal/symbol"
	"github.com/manikyarathore/pygoc/internal/token"
)

func TestBinaryOpResultCases(t *testing.T) {
	cases := []struct {
		name       string
		op         token.Type
		left, right Type
		want       Type
		wantErr    bool
	}{
		{"int+int", token.PLUS, IntT, IntT, IntT, false},
		{"float+float", token.PLUS, FloatT, FloatT, FloatT, false},
		{"int+float widens", token.PLUS, IntT, FloatT, FloatT, false},
		{"string+string concatenates", token.PLUS, StringT, StringT, StringT, false},
		{"string+int is an error", token.PLUS, StringT, IntT, UnknownT, true},
		{"true division always float", token.SLASH, IntT, IntT, FloatT, false},
		{"floor division int stays int", token.DSLASH, IntT, IntT, IntT, false},
		{"floor division with float widens", token.DSLASH, IntT, FloatT, FloatT, false},
		{"comparison yields bool", token.LT, IntT, IntT, BoolT, false},
		{"and requires both bool", token.AND, BoolT, BoolT, BoolT, false},
		{"and rejects non-bool", token.AND, IntT, BoolT, UnknownT, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := BinaryOpResult(c.op, c.left, c.right)
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got none (result=%v)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !got.Equals(c.want) {
				t.Fatalf("got %v, want %v", got, c.want)
			}
		})
	}
}

func TestUnaryOpResultCases(t *testing.T) {
	cases := []struct {
		name    string
		op      token.Type
		operand Type
		want    Type
		wantErr bool
	}{
		{"negate int", token.MINUS, IntT, IntT, false},
		{"negate bool is an error", token.MINUS, BoolT, UnknownT, true},
		{"not bool", token.NOT, BoolT, BoolT, false},
		{"not int is an error", token.NOT, IntT, UnknownT, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := UnaryOpResult(c.op, c.operand)
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !got.Equals(c.want) {
				t.Fatalf("got %v, want %v", got, c.want)
			}
		})
	}
}

// inferExprSrc is a small test helper: parses a single expression
// statement's expression and infers its type against a table pre-loaded
// with the given variable types.
func inferExprSrc(t *testing.T, src string, preload map[string]Type) (Type, error) {
	t.Helper()
	toks, lexErrs := lexer.New(src + "\n").Tokenize()
	if len(lexErrs) != 0 {
		t.Fatalf("lex errors: %v", lexErrs)
	}
	prog, parseErrs := parser.New(toks).ParseProgram()
	if len(parseErrs) != 0 {
		t.Fatalf("parse errors: %v", parseErrs)
	}
	exprStmt, ok := prog.Statements[0].(*ast.ExprStmt)
	if !ok {
		t.Fatalf("expected an ExprStmt, got %T", prog.Statements[0])
	}
	tbl := symbol.NewTable()
	for name, ty := range preload {
		tbl.Define(&symbol.Symbol{Name: name, Type: ty, Kind: symbol.VarSymbol})
	}
	return Infer(exprStmt.X, tbl)
}

func TestInferenceLoopedCases(t *testing.T) {
	cases := []struct {
		name    string
		src     string
		preload map[string]Type
		want    Type
		wantErr bool
	}{
		{"int literal", "10", nil, IntT, false},
		{"float literal", "3.14", nil, FloatT, false},
		{"string literal", `"hi"`, nil, StringT, false},
		{"arithmetic expression", "a + b * 2", map[string]Type{"a": IntT, "b": IntT}, IntT, false},
		{"comparison", "a < b", map[string]Type{"a": IntT, "b": IntT}, BoolT, false},
		{"undefined variable errors", "nope", nil, UnknownT, true},
		{"mixed type add errors", `a + "x"`, map[string]Type{"a": IntT}, UnknownT, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := inferExprSrc(t, c.src, c.preload)
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got none (result=%v)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !got.Equals(c.want) {
				t.Fatalf("got %v, want %v", got, c.want)
			}
		})
	}
}
