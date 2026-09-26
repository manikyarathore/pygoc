package semantic

import (
	"testing"

	"github.com/manikyarathore/pygoc/internal/lexer"
	"github.com/manikyarathore/pygoc/internal/parser"
)

// analyzeSrc lexes, parses, and semantically analyzes src, returning
// the semantic errors. Fails the test immediately on a lex/parse error,
// since those aren't what this file is testing.
func analyzeSrc(t *testing.T, src string) []error {
	t.Helper()
	toks, lexErrs := lexer.New(src).Tokenize()
	if len(lexErrs) != 0 {
		t.Fatalf("unexpected lex errors: %v", lexErrs)
	}
	prog, parseErrs := parser.New(toks).ParseProgram()
	if len(parseErrs) != 0 {
		t.Fatalf("unexpected parse errors: %v", parseErrs)
	}
	return New().Analyze(prog)
}

// TestSemanticCases loops over many small programs, each expected to
// either pass cleanly or fail with at least one semantic error — this
// is the "loop to test properly" harness for the whole semantic layer.
func TestSemanticCases(t *testing.T) {
	cases := []struct {
		name    string
		src     string
		wantErr bool
	}{
		{"simple valid assignment", "x = 10\n", false},
		{"valid arithmetic", "x = 1 + 2\ny = x * 3\n", false},
		{"undefined variable used", "y = x + 1\n", true},
		{"type mismatch on reassignment", "x = 1\nx = \"oops\"\n", true},
		{"valid if with bool condition", "x = 1\nif x < 10:\n    y = 2\n", false},
		{"if condition not bool", "x = 1\nif x:\n    y = 2\n", true},
		{"valid while loop", "x = 0\nwhile x < 5:\n    x += 1\n", false},
		{"valid for-range loop", "for i in range(5):\n    x = i\n", false},
		{"for-range reused var across two loops is fine", "for i in range(5):\n    x = i\nfor i in range(3):\n    y = i\n", false},
		{
			"valid recursive function with matching return type",
			"def fact(n: int) -> int:\n    if n <= 1:\n        return 1\n    return n * fact(n - 1)\n",
			false,
		},
		{
			"function missing return statement",
			"def f(n: int) -> int:\n    x = n + 1\n",
			true,
		},
		{
			"function return type mismatch",
			"def f() -> int:\n    return \"oops\"\n",
			true,
		},
		{
			"call to undefined function",
			"x = mystery(1)\n",
			true,
		},
		{"print is always valid", "print(1 + 2)\n", false},
		{"tuple-unpacking assignment is rejected in this build", "x = 1\na, b = x\n", true},
		{"list literal is rejected in this build", "x = [1, 2, 3]\n", true},
		{
			"compound assignment on undefined variable",
			"x += 1\n",
			true,
		},
		{
			"redefining a function name at global scope errors",
			"def f() -> int:\n    return 1\ndef f() -> int:\n    return 2\n",
			true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			errs := analyzeSrc(t, c.src)
			if c.wantErr && len(errs) == 0 {
				t.Fatalf("expected a semantic error, got none")
			}
			if !c.wantErr && len(errs) != 0 {
				t.Fatalf("expected no semantic errors, got: %v", errs)
			}
		})
	}
}

func TestReadOnlyClosureCapture(t *testing.T) {
	// A nested function may READ an outer variable...
	errs := analyzeSrc(t, ""+
		"def outer() -> int:\n"+
		"    x = 10\n"+
		"    def inner() -> int:\n"+
		"        return x\n"+
		"    return inner()\n",
	)
	if len(errs) != 0 {
		t.Fatalf("expected reading a captured variable to be valid, got: %v", errs)
	}
}
