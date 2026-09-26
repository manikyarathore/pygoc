// Command pygoc is PyGoC's CLI: it dispatches to whichever compiler
// phase (or the full six-phase pipeline) the user asked for. No
// compiler logic lives here — every subcommand is a thin wrapper that
// calls into internal/lexer, internal/parser, internal/semantic,
// internal/ir, internal/optimizer, and internal/codegen in sequence.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/manikyarathore/pygoc/internal/ast"
	"github.com/manikyarathore/pygoc/internal/codegen"
	"github.com/manikyarathore/pygoc/internal/ir"
	"github.com/manikyarathore/pygoc/internal/lexer"
	"github.com/manikyarathore/pygoc/internal/optimizer"
	"github.com/manikyarathore/pygoc/internal/parser"
	"github.com/manikyarathore/pygoc/internal/semantic"
)

func main() {
	if len(os.Args) < 3 {
		printUsage()
		os.Exit(1)
	}
	cmd := os.Args[1]
	path := os.Args[2]

	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error reading file:", err)
		os.Exit(1)
	}
	src := string(data)

	switch cmd {
	case "lex":
		cmdLex(src)
	case "ast":
		cmdAst(src)
	case "sema":
		cmdSema(src)
	case "ir":
		cmdIR(src)
	case "optimize":
		cmdOptimize(src)
	case "compile":
		cmdCompile(src, path)
	case "run":
		cmdRun(src, path)
	case "check":
		cmdCheck(src)
	case "pipeline":
		cmdPipeline(src)
	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: pygoc <lex|ast|sema|ir|optimize|compile|run|check|pipeline> <file.py>")
}

// ---- shared helpers -------------------------------------------------

// lexAndParse runs Phases 1-2 and prints any errors it finds. ok is
// false if the program cannot safely proceed to later phases.
func lexAndParse(src string) (*ast.Program, bool) {
	toks, lexErrs := lexer.New(src).Tokenize()
	if len(lexErrs) != 0 {
		for _, e := range lexErrs {
			fmt.Println("Syntax Error (lexical):", e)
		}
		return nil, false
	}
	prog, parseErrs := parser.New(toks).ParseProgram()
	if len(parseErrs) != 0 {
		for _, e := range parseErrs {
			fmt.Println("Syntax Error:")
			fmt.Println(e)
		}
		return nil, false
	}
	return prog, true
}

func printTokens(src string) {
	toks, lexErrs := lexer.New(src).Tokenize()
	for _, tok := range toks {
		if tok.Lexeme != "" {
			fmt.Printf("%s(%s)\n", tok.Type, tok.Lexeme)
		} else {
			fmt.Printf("%s\n", tok.Type)
		}
	}
	for _, e := range lexErrs {
		fmt.Println("Syntax Error (lexical):", e)
	}
}

func printAST(prog *ast.Program) {
	fmt.Print(ast.Sprint(prog))
}

// runSema runs Phase 3 and prints its errors/output; returns the
// analyzer and whether the program passed.
func runSema(prog *ast.Program, printOutput bool) (*semantic.Analyzer, bool) {
	an := semantic.New()
	errs := an.Analyze(prog)
	if printOutput {
		fmt.Print(an.PrintSymbolTable())
		fmt.Println()
		fmt.Print(semantic.PrintTypeAnalysis(prog, an.Table))
	}
	if len(errs) != 0 {
		for _, e := range errs {
			fmt.Println("Semantic Error:")
			fmt.Println(e)
			fmt.Println()
		}
		return an, false
	}
	return an, true
}

// ---- subcommands -------------------------------------------------

func cmdLex(src string) {
	printTokens(src)
}

func cmdAst(src string) {
	prog, ok := lexAndParse(src)
	if !ok {
		os.Exit(1)
	}
	printAST(prog)
}

func cmdSema(src string) {
	prog, ok := lexAndParse(src)
	if !ok {
		os.Exit(1)
	}
	if _, ok := runSema(prog, true); !ok {
		os.Exit(1)
	}
}

func cmdIR(src string) {
	prog, ok := lexAndParse(src)
	if !ok {
		os.Exit(1)
	}
	if _, ok := runSema(prog, false); !ok {
		os.Exit(1)
	}
	mod := ir.Build(prog)
	fmt.Print(mod.String())
}

func cmdOptimize(src string) {
	prog, ok := lexAndParse(src)
	if !ok {
		os.Exit(1)
	}
	if _, ok := runSema(prog, false); !ok {
		os.Exit(1)
	}
	mod := ir.Build(prog)
	printOptimizeStages(mod)
}

func printOptimizeStages(mod *ir.Module) {
	for _, fn := range mod.Functions {
		fmt.Printf("---- function %s ----\n\n", fn.Name)
		r := optimizer.Run(fn)

		fmt.Println("=== Original IR ===")
		fmt.Print(r.Original.String())
		fmt.Println()

		fmt.Println("=== Constant Folding ===")
		fmt.Print(r.AfterConstFold.String())
		fmt.Println()

		fmt.Println("=== Constant Propagation ===")
		fmt.Print(r.AfterConstProp.String())
		fmt.Println()

		fmt.Println("=== Algebraic Simplification ===")
		fmt.Print(r.AfterAlgebraic.String())
		fmt.Println()

		fmt.Println("=== Common Subexpression Elimination ===")
		fmt.Print(r.AfterCSE.String())
		fmt.Println()

		fmt.Println("=== Dead Code Elimination ===")
		fmt.Print(r.AfterDeadCode.String())
		fmt.Println()

		fmt.Println("=== Final Optimized IR ===")
		fmt.Print(r.Final().String())
		fmt.Println()
	}
}

// compileToGo runs Phases 1-6 fully and returns the generated Go
// source, or ok=false if any earlier phase failed.
func compileToGo(src string) (string, bool) {
	prog, ok := lexAndParse(src)
	if !ok {
		return "", false
	}
	if _, ok := runSema(prog, false); !ok {
		return "", false
	}
	mod := ir.Build(prog)
	optimized := optimizer.RunModule(mod).Final
	out, err := codegen.Generate(optimized)
	if err != nil {
		fmt.Println("Code Generation Error:", err)
		return "", false
	}
	return out, true
}

func cmdCompile(src, path string) {
	out, ok := compileToGo(src)
	if !ok {
		os.Exit(1)
	}
	fmt.Print(out)

	outPath := strings.TrimSuffix(path, filepath.Ext(path)) + ".go"
	if err := os.WriteFile(outPath, []byte(out), 0644); err == nil {
		fmt.Fprintln(os.Stderr, "\n(also written to "+outPath+")")
	}
}

func cmdRun(src, path string) {
	out, ok := compileToGo(src)
	if !ok {
		os.Exit(1)
	}
	outPath := strings.TrimSuffix(path, filepath.Ext(path)) + ".go"
	if err := os.WriteFile(outPath, []byte(out), 0644); err != nil {
		fmt.Fprintln(os.Stderr, "error writing generated Go file:", err)
		os.Exit(1)
	}
	cmd := exec.Command("go", "run", outPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error running generated program:", err)
		os.Exit(1)
	}
}

func cmdCheck(src string) {
	prog, ok := lexAndParse(src)
	if !ok {
		os.Exit(1)
	}
	if _, ok := runSema(prog, false); !ok {
		os.Exit(1)
	}
	fmt.Println("OK — no semantic errors found.")
}

func cmdPipeline(src string) {
	sep := strings.Repeat("═", 38)
	fmt.Println(sep)
	fmt.Println("        PyGoC Compiler Pipeline")
	fmt.Println(sep)
	fmt.Println()

	fmt.Println("[1] LEXICAL ANALYSIS")
	fmt.Println(strings.Repeat("-", 20))
	fmt.Println("Tokens:")
	printTokens(src)
	fmt.Println()

	prog, ok := lexAndParse(src)
	if !ok {
		fmt.Println(sep)
		fmt.Println("      Compilation Failed (syntax errors)")
		fmt.Println(sep)
		os.Exit(1)
	}

	fmt.Println("[2] SYNTAX ANALYSIS")
	fmt.Println(strings.Repeat("-", 19))
	fmt.Println("AST:")
	printAST(prog)
	fmt.Println()

	fmt.Println("[3] SEMANTIC ANALYSIS")
	fmt.Println(strings.Repeat("-", 21))
	an, ok := runSema(prog, true)
	fmt.Println()
	if !ok {
		fmt.Println(sep)
		fmt.Println("      Compilation Failed (semantic errors)")
		fmt.Println(sep)
		os.Exit(1)
	}
	_ = an

	mod := ir.Build(prog)
	fmt.Println("[4] INTERMEDIATE CODE")
	fmt.Println(strings.Repeat("-", 21))
	fmt.Println("TAC + CFG:")
	fmt.Print(mod.String())
	fmt.Println()

	fmt.Println("[5] CODE OPTIMIZATION")
	fmt.Println(strings.Repeat("-", 21))
	printOptimizeStages(mod)

	fmt.Println("[6] CODE GENERATION")
	fmt.Println(strings.Repeat("-", 19))
	optimized := optimizer.RunModule(mod).Final
	out, err := codegen.Generate(optimized)
	if err != nil {
		fmt.Println("Code Generation Error:", err)
		os.Exit(1)
	}
	fmt.Println("Generated Go:")
	fmt.Print(out)
	fmt.Println()

	fmt.Println(sep)
	fmt.Println("          Compilation Complete")
	fmt.Println(sep)
}
