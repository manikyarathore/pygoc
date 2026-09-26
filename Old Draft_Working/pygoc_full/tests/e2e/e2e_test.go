// Package e2e_test is the looping end-to-end harness: every .py file in
// this directory with a matching .expected.txt is compiled through all
// six phases, the resulting Go program is actually `go run`, and its
// stdout is diffed against the expected output. Adding a new end-to-end
// test case means adding two files here — no new Go code required.
package e2e_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/manikyarathore/pygoc/internal/codegen"
	"github.com/manikyarathore/pygoc/internal/ir"
	"github.com/manikyarathore/pygoc/internal/lexer"
	"github.com/manikyarathore/pygoc/internal/optimizer"
	"github.com/manikyarathore/pygoc/internal/parser"
	"github.com/manikyarathore/pygoc/internal/semantic"
)

func TestEndToEnd(t *testing.T) {
	pyFiles, err := filepath.Glob("*.py")
	if err != nil {
		t.Fatalf("glob failed: %v", err)
	}
	if len(pyFiles) == 0 {
		t.Fatalf("no .py test files found in tests/e2e — did the example files ship correctly?")
	}

	for _, pyFile := range pyFiles {
		name := strings.TrimSuffix(pyFile, ".py")
		expectedPath := name + ".expected.txt"
		if _, err := os.Stat(expectedPath); err != nil {
			continue // no matching expected-output file for this .py; skip it
		}

		t.Run(name, func(t *testing.T) {
			srcBytes, err := os.ReadFile(pyFile)
			if err != nil {
				t.Fatalf("reading %s: %v", pyFile, err)
			}
			expectedBytes, err := os.ReadFile(expectedPath)
			if err != nil {
				t.Fatalf("reading %s: %v", expectedPath, err)
			}
			expected := strings.TrimSpace(string(expectedBytes))

			goSrc := compileFull(t, string(srcBytes))

			tmpDir := t.TempDir()
			goFile := filepath.Join(tmpDir, "main.go")
			if err := os.WriteFile(goFile, []byte(goSrc), 0644); err != nil {
				t.Fatalf("writing generated Go: %v", err)
			}

			cmd := exec.Command("go", "run", goFile)
			outBytes, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("generated program failed to run: %v\noutput:\n%s\ngenerated source:\n%s", err, outBytes, goSrc)
			}
			actual := strings.TrimSpace(string(outBytes))
			if actual != expected {
				t.Errorf("output mismatch for %s\n  got:  %q\n  want: %q\ngenerated source:\n%s", pyFile, actual, expected, goSrc)
			}
		})
	}
}

// compileFull runs all six phases exactly as the CLI's `pygoc compile`
// command does, failing the test immediately (with full context) if any
// phase reports an error — an e2e test case is expected to be a VALID
// program; testing error handling itself belongs in each phase's own
// unit tests, not here.
func compileFull(t *testing.T, src string) string {
	t.Helper()
	toks, lexErrs := lexer.New(src).Tokenize()
	if len(lexErrs) != 0 {
		t.Fatalf("lex errors: %v", lexErrs)
	}
	prog, parseErrs := parser.New(toks).ParseProgram()
	if len(parseErrs) != 0 {
		t.Fatalf("parse errors: %v", parseErrs)
	}
	an := semantic.New()
	if errs := an.Analyze(prog); len(errs) != 0 {
		t.Fatalf("semantic errors: %v", errs)
	}
	mod := ir.Build(prog)
	optimized := optimizer.RunModule(mod).Final
	out, err := codegen.Generate(optimized)
	if err != nil {
		t.Fatalf("codegen error: %v", err)
	}
	return out
}
