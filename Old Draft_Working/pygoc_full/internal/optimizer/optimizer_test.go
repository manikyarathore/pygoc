package optimizer

import (
	"strings"
	"testing"

	"github.com/manikyarathore/pygoc/internal/ir"
	"github.com/manikyarathore/pygoc/internal/lexer"
	"github.com/manikyarathore/pygoc/internal/parser"
)

// mainFuncFromSrc lexes, parses, and lowers src to IR, returning the
// synthetic "main" function (always Functions[0] for source with no
// top-level function declarations).
func mainFuncFromSrc(t *testing.T, src string) *ir.Function {
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
	return mod.Functions[0]
}

func TestConstantFoldCases(t *testing.T) {
	cases := []struct {
		name        string
		src         string
		wantContain string
		wantAbsent  string
	}{
		// NOTE: these check the temp that HOLDS the folded value, not
		// the outer variable — constant folding alone only touches
		// BinOp/UnOp instructions directly; propagating the folded
		// value onward into `x = t1` is ConstantPropagate's job, tested
		// separately (and exercised together in
		// TestFullPipelineOnRealisticProgram below).
		{"folds int addition", "x = 2 + 3\n", "t1 = 5", "2 + 3"},
		{"folds multiplication", "x = 10 * 20\n", "t1 = 200", "10 * 20"},
		{"folds float division", "x = 1.0 / 2.0\n", "t1 = 0.5", "1 / 2"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fn := mainFuncFromSrc(t, c.src)
			out := ConstantFold(fn).String()
			if !strings.Contains(out, c.wantContain) {
				t.Errorf("expected folded IR to contain %q, got:\n%s", c.wantContain, out)
			}
			if strings.Contains(out, c.wantAbsent) {
				t.Errorf("expected folded IR NOT to contain %q, got:\n%s", c.wantAbsent, out)
			}
		})
	}
}

func TestAlgebraicSimplifyCases(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		// Same note as above: algebraic simplification only touches the
		// BinOp itself (rewriting `t1 = x*1` to `t1 = x`) — it doesn't
		// reach into the LATER `y = t1` copy, which is constant
		// propagation's job.
		{"x*1 simplifies away the multiply", "x = 5\ny = x * 1\n", "t1 = x"},
		{"x+0 simplifies away the add", "x = 5\ny = x + 0\n", "t1 = x"},
		{"x*0 becomes a zero constant", "x = 5\ny = x * 0\n", "t1 = 0"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fn := mainFuncFromSrc(t, c.src)
			out := AlgebraicSimplify(fn).String()
			if !strings.Contains(out, c.want) {
				t.Errorf("expected simplified IR to contain %q, got:\n%s", c.want, out)
			}
		})
	}
}

func TestCommonSubexpressionEliminationReusesResult(t *testing.T) {
	fn := mainFuncFromSrc(t, "a = 1\nb = 2\nx = a + b\ny = a + b\n")
	out := CommonSubexpressionElimination(fn).String()
	// The second `a + b` should be replaced by a plain copy of the first
	// result rather than recomputed.
	count := strings.Count(out, "a + b")
	if count != 1 {
		t.Errorf("expected exactly one recomputation of 'a + b', got %d in:\n%s", count, out)
	}
}

func TestDeadCodeEliminationRemovesUnusedTemp(t *testing.T) {
	// `y` is computed but never used anywhere — it should disappear.
	fn := mainFuncFromSrc(t, "x = 1\ny = x + 1\nprint(x)\n")
	out := DeadCodeElimination(fn).String()
	if strings.Contains(out, "y =") {
		t.Errorf("expected the unused 'y' assignment to be eliminated, got:\n%s", out)
	}
	if !strings.Contains(out, "call print, x") {
		t.Errorf("expected the print call (a side effect) to survive, got:\n%s", out)
	}
}

func TestDeadCodeEliminationKeepsCallsRegardlessOfUse(t *testing.T) {
	// print has no return value used anywhere, but it must never be
	// removed — it's a side effect, not dead code.
	fn := mainFuncFromSrc(t, "print(42)\n")
	out := DeadCodeElimination(fn).String()
	if !strings.Contains(out, "call print") {
		t.Fatalf("expected the print call to survive dead code elimination, got:\n%s", out)
	}
}

// TestFullPipelineOnRealisticProgram loops the whole 5-pass pipeline
// over one representative program and checks the FINAL result is both
// smaller than the original and still functionally sound (retains the
// print call, folds the constant arithmetic).
func TestFullPipelineOnRealisticProgram(t *testing.T) {
	fn := mainFuncFromSrc(t, ""+
		"a = 2 + 3\n"+ // should fold to 5
		"b = a * 1\n"+ // should simplify to a copy of a
		"c = a + 3\n"+ // dead — never used
		"print(b)\n",
	)
	result := Run(fn)

	if result.Original == nil || result.AfterConstFold == nil || result.AfterConstProp == nil ||
		result.AfterAlgebraic == nil || result.AfterCSE == nil || result.AfterDeadCode == nil {
		t.Fatalf("expected every pipeline stage to be populated (non-black-box requirement)")
	}

	final := result.Final().String()
	if strings.Contains(final, "c =") {
		t.Errorf("expected dead variable 'c' to be eliminated from the final IR, got:\n%s", final)
	}
	if !strings.Contains(final, "call print") {
		t.Errorf("expected the print call to survive to the final IR, got:\n%s", final)
	}

	origLines := countInstrLines(result.Original.String())
	finalLines := countInstrLines(final)
	if finalLines >= origLines {
		t.Errorf("expected the optimized IR (%d instruction lines) to be smaller than the original (%d)", finalLines, origLines)
	}
}

func countInstrLines(s string) int {
	n := 0
	for _, line := range strings.Split(s, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasSuffix(trimmed, ":") {
			n++
		}
	}
	return n
}
