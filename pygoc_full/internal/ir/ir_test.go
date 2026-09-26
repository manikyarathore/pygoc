package ir

import (
	"strings"
	"testing"

	"github.com/manikyarathore/pygoc/internal/lexer"
	"github.com/manikyarathore/pygoc/internal/parser"
)

func buildIRSrc(t *testing.T, src string) *Module {
	t.Helper()
	toks, lexErrs := lexer.New(src).Tokenize()
	if len(lexErrs) != 0 {
		t.Fatalf("unexpected lex errors: %v", lexErrs)
	}
	prog, parseErrs := parser.New(toks).ParseProgram()
	if len(parseErrs) != 0 {
		t.Fatalf("unexpected parse errors: %v", parseErrs)
	}
	return Build(prog)
}

// TestIRContainsExpectedSubstrings loops over several PyGo snippets,
// each checked against a set of substrings its printed IR must contain
// — this is deliberately a substring/looping check rather than a
// brittle full-text golden match, since exact temp/label numbering is
// an implementation detail, not part of the contract being tested.
func TestIRContainsExpectedSubstrings(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []string
	}{
		{
			name: "arithmetic assignment folds through a temp",
			src:  "x = 10 + 20\nprint(x)\n",
			want: []string{"= 10 + 20", "x = t", "call print, x"},
		},
		{
			name: "if produces a conditional branch",
			src:  "x = 1\nif x < 10:\n    y = 2\n",
			want: []string{"< 10", "if t", "goto B"},
		},
		{
			name: "while produces a loop-back goto",
			src:  "x = 0\nwhile x < 5:\n    x += 1\n",
			want: []string{"x = x + 1", "goto B"},
		},
		{
			name: "for-range desugars into a comparison and increment",
			src:  "for i in range(5):\n    x = i\n",
			want: []string{"i = 0", "i < t", "i + 1"},
		},
		{
			name: "function declaration becomes its own IR function",
			src:  "def add(a: int, b: int) -> int:\n    return a + b\n",
			want: []string{"func add:", "return t"},
		},
		{
			name: "recursive call lowers to a call instruction with a result temp",
			src:  "def fact(n: int) -> int:\n    if n <= 1:\n        return 1\n    return n * fact(n - 1)\n",
			want: []string{"call fact,"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mod := buildIRSrc(t, c.src)
			out := mod.String()
			for _, w := range c.want {
				if !strings.Contains(out, w) {
					t.Errorf("expected IR to contain %q, got:\n%s", w, out)
				}
			}
		})
	}
}

func TestModuleHasMainAndDeclaredFunctions(t *testing.T) {
	mod := buildIRSrc(t, "def add(a: int, b: int) -> int:\n    return a + b\nx = add(1, 2)\nprint(x)\n")
	if len(mod.Functions) != 2 {
		t.Fatalf("expected 2 functions (main + add), got %d", len(mod.Functions))
	}
	names := map[string]bool{}
	for _, fn := range mod.Functions {
		names[fn.Name] = true
	}
	if !names["main"] || !names["add"] {
		t.Fatalf("expected both 'main' and 'add' functions, got %v", names)
	}
}

// TestBreakAndContinueTargetsAreSafe verifies that continue inside a
// for-loop targets the increment step, not the condition test directly
// — jumping straight to the condition would skip the increment and hang
// the generated program in an infinite loop.
func TestContinueTargetsIncrementNotConditionDirectly(t *testing.T) {
	mod := buildIRSrc(t, "for i in range(5):\n    continue\n")
	fn := mod.Functions[0]

	blocksByLabel := map[string]*Block{}
	for _, b := range fn.Blocks {
		blocksByLabel[b.Label] = b
	}

	// Precisely locate the loop-BODY block via the loop's own IfGoto
	// (its TrueLabel names the body block unambiguously) — scanning
	// "the last goto found anywhere in the function" would be wrong,
	// since the increment block also ends with its own goto back to the
	// condition test.
	var bodyBlock *Block
	for _, b := range fn.Blocks {
		for _, instr := range b.Instrs {
			if ig, ok := instr.(IfGoto); ok {
				bodyBlock = blocksByLabel[ig.TrueLabel]
			}
		}
	}
	if bodyBlock == nil {
		t.Fatalf("could not locate the loop body block via IfGoto.TrueLabel")
	}

	// The body's only statement is `continue`, so its first (and only)
	// instruction must be a Goto — find where it points.
	var continueTargetLabel string
	for _, instr := range bodyBlock.Instrs {
		if g, ok := instr.(Goto); ok {
			continueTargetLabel = g.Label
			break
		}
	}
	if continueTargetLabel == "" {
		t.Fatalf("expected the loop body to contain a goto for 'continue'")
	}

	target, ok := blocksByLabel[continueTargetLabel]
	if !ok {
		t.Fatalf("continue's target label %q does not match any block", continueTargetLabel)
	}
	found := false
	for _, instr := range target.Instrs {
		if strings.Contains(instr.String(), "+") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected continue's target block (%s) to contain the increment step, got:\n%s", continueTargetLabel, target.String())
	}
}
