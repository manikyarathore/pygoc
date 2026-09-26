package codegen

import (
	"strings"
	"testing"

	"github.com/manikyarathore/pygoc/internal/ir"
	"github.com/manikyarathore/pygoc/internal/lexer"
	"github.com/manikyarathore/pygoc/internal/optimizer"
	"github.com/manikyarathore/pygoc/internal/parser"
)

// generateSrc runs the full pipeline (lex -> parse -> ir.Build ->
// optimizer.RunModule -> Generate) and returns the resulting Go source.
//
// NOTE ON VERIFICATION: this test environment has no Go toolchain
// available, so these tests check STRUCTURAL properties of the
// generated source (balanced braces, expected signatures, expected
// folded values, expected imports) rather than actually compiling it.
// Treat `go build`/`go vet` on the real output as the authoritative
// check — see tests/e2e for the harness that does exactly that.
func generateSrc(t *testing.T, src string) string {
	t.Helper()
	toks, lexErrs := lexer.New(src).Tokenize()
	if len(lexErrs) != 0 {
		t.Fatalf("unexpected lex errors: %v", lexErrs)
	}
	prog, parseErrs := parser.New(toks).ParseProgram()
	if len(parseErrs) != 0 {
		t.Fatalf("unexpected parse errors: %v", parseErrs)
	}
	mod := ir.Build(prog)
	optimized := optimizer.RunModule(mod).Final
	out, err := Generate(optimized)
	if err != nil {
		t.Fatalf("unexpected codegen error: %v", err)
	}
	return out
}

func assertBalancedBraces(t *testing.T, src string) {
	t.Helper()
	depth := 0
	for _, ch := range src {
		switch ch {
		case '{':
			depth++
		case '}':
			depth--
		}
		if depth < 0 {
			t.Fatalf("unbalanced braces (closed before opened) in generated source:\n%s", src)
		}
	}
	if depth != 0 {
		t.Fatalf("unbalanced braces (depth=%d at EOF) in generated source:\n%s", depth, src)
	}
}

func TestGeneratedSourceLoopedCases(t *testing.T) {
	cases := []struct {
		name         string
		src          string
		wantContains []string
	}{
		{
			name: "simple print of folded constant",
			src:  "x = 10 + 20\nprint(x)\n",
			// NOTE: after the constprop/deadcode fixes, the full pipeline
			// correctly collapses this ALL the way down — x and its
			// intermediate temp both get eliminated once their only use
			// (print's argument) is itself constant-substituted away.
			// That's genuinely correct, more aggressive optimization,
			// not a regression — the original assertion here ("x = 30",
			// "fmt.Println(x)") was simply testing for the WRONG (less
			// optimized) shape.
			wantContains: []string{"package main", `"fmt"`, "func main()", "fmt.Println(30)"},
		},
		{
			name:         "function with params and return type",
			src:          "def add(a: int, b: int) -> int:\n    return a + b\nprint(add(1, 2))\n",
			// `a`/`b` are function PARAMETERS, not compile-time
			// constants, so the addition can never fold away — it
			// legitimately computes through a temp ("t1 = a + b" then
			// "return t1"). Checking for the substring "a + b" (which
			// matches regardless of which temp holds it) rather than
			// over-specifying the exact generated statement shape.
			wantContains: []string{"func add(a int, b int) int", "a + b"},
		},
		{
			name:         "if/else compiles to labeled goto blocks",
			src:          "x = 5\nif x < 10:\n    print(1)\nelse:\n    print(2)\n",
			wantContains: []string{"if t", "goto B", "} else {"},
		},
		{
			name:         "while loop compiles to labeled goto blocks",
			src:          "x = 0\nwhile x < 3:\n    x += 1\nprint(x)\n",
			wantContains: []string{"goto B"},
		},
		{
			name:         "for-range loop compiles correctly",
			src:          "total = 0\nfor i in range(5):\n    total += i\nprint(total)\n",
			wantContains: []string{"i = 0", "goto B"},
		},
		{
			name:         "recursive factorial function",
			src:          "def fact(n: int) -> int:\n    if n <= 1:\n        return 1\n    return n * fact(n - 1)\nprint(fact(5))\n",
			wantContains: []string{"func fact(n int) int", "fact("},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out := generateSrc(t, c.src)
			assertBalancedBraces(t, out)
			for _, want := range c.wantContains {
				if !strings.Contains(out, want) {
					t.Errorf("expected generated Go to contain %q, got:\n%s", want, out)
				}
			}
		})
	}
}

func TestNoUnnecessaryImports(t *testing.T) {
	// A program with no print() and no float floor-division should not
	// import fmt or math at all.
	out := generateSrc(t, "x = 1 + 2\n")
	if strings.Contains(out, "import") {
		t.Errorf("expected no import block for a program with no print()/math use, got:\n%s", out)
	}
}

// TestVoidFunctionNeverFallsThroughBetweenBlocks is a regression test
// for a real infinite-loop bug caught by the e2e test suite: a `while`
// loop's own end-block can be POSITIONED, in the generated text, before
// blocks belonging to a nested construct inside its body (block order
// reflects creation time, not final usage time) — and without an
// explicit terminator, Go's normal statement-by-statement fall-through
// would silently continue execution INTO that unrelated later block
// instead of ending the function, producing a real infinite loop at
// runtime rather than a compile error. Every block in a void function
// must end in an explicit jump/return, regardless of its position in
// the generated source.
func TestVoidFunctionNeverFallsThroughBetweenBlocks(t *testing.T) {
	// This is exactly the shape that triggered the original bug: a
	// while-loop whose body contains a nested if/break, immediately
	// followed by the loop's only remaining statement (print).
	src := "x = 0\nwhile True:\n    x += 1\n    if x >= 5:\n        break\nprint(x)\n"
	out := generateSrc(t, src)
	assertBalancedBraces(t, out)

	lines := strings.Split(out, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "fmt.Println(") {
			continue
		}
		// The line immediately after any fmt.Println(...) call must be
		// an explicit terminator (return/goto), NOT another block's
		// label or statement that execution would fall straight into.
		if i+1 >= len(lines) {
			t.Fatalf("fmt.Println was the last line with nothing after it:\n%s", out)
		}
		next := strings.TrimSpace(lines[i+1])
		if next != "return" && !strings.HasPrefix(next, "goto ") && next != "}" {
			t.Errorf("expected an explicit terminator right after fmt.Println, got %q — this is exactly the fall-through pattern that caused a real infinite loop; full output:\n%s", next, out)
		}
	}
}

func TestEveryLabelHasATrailingStatement(t *testing.T) {
	// Regression test for the "label immediately before '}'" bug class:
	// every generated function must not end with a bare label followed
	// only by the closing brace.
	cases := []string{
		"x = 0\nwhile x < 3:\n    x += 1\n",
		"if 1 < 2:\n    x = 1\n",
		"for i in range(3):\n    x = i\n",
	}
	for _, src := range cases {
		out := generateSrc(t, src)
		lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
		for i := 0; i < len(lines)-1; i++ {
			trimmed := strings.TrimSpace(lines[i])
			next := strings.TrimSpace(lines[i+1])
			if strings.HasSuffix(trimmed, ":") && next == "}" {
				t.Errorf("found a label with no statement before the closing brace in:\n%s", out)
			}
		}
	}
}
