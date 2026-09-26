package parser

import (
	"testing"

	"github.com/manikyarathore/pygoc/internal/ast"
	"github.com/manikyarathore/pygoc/internal/lexer"
	"github.com/manikyarathore/pygoc/internal/token"
)

// parseSrc is the standard test entrypoint: lex then parse, failing the
// test immediately if either stage reports errors. This mirrors how the
// real pipeline will chain lexer -> parser.
func parseSrc(t *testing.T, src string) *ast.Program {
	t.Helper()
	toks, lexErrs := lexer.New(src).Tokenize()
	if len(lexErrs) != 0 {
		t.Fatalf("unexpected lex errors: %v", lexErrs)
	}
	prog, parseErrs := New(toks).ParseProgram()
	if len(parseErrs) != 0 {
		t.Fatalf("unexpected parse errors: %v", parseErrs)
	}
	return prog
}

func TestSimpleAssignment(t *testing.T) {
	prog := parseSrc(t, "x = 10\n")
	if len(prog.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(prog.Statements))
	}
	assign, ok := prog.Statements[0].(*ast.AssignStmt)
	if !ok {
		t.Fatalf("expected *ast.AssignStmt, got %T", prog.Statements[0])
	}
	if assign.Op != token.ASSIGN {
		t.Fatalf("expected ASSIGN, got %v", assign.Op)
	}
	if _, ok := assign.Value.(*ast.IntLiteral); !ok {
		t.Fatalf("expected IntLiteral value, got %T", assign.Value)
	}
}

// TestArithmeticPrecedence is the most important test in this file: it
// verifies that `a + b * 2 - c // 3` builds the tree
// `(a + (b * 2)) - (c // 3)`, i.e. that * and // bind tighter than +
// and -, and that + / - are left-associative.
func TestArithmeticPrecedence(t *testing.T) {
	prog := parseSrc(t, "y = a + b * 2 - c // 3\n")
	assign := prog.Statements[0].(*ast.AssignStmt)

	// top level: (a + b*2) - (c // 3)
	top, ok := assign.Value.(*ast.BinaryExpr)
	if !ok || top.Op != token.MINUS {
		t.Fatalf("expected top-level MINUS, got %#v", assign.Value)
	}

	left, ok := top.Left.(*ast.BinaryExpr)
	if !ok || left.Op != token.PLUS {
		t.Fatalf("expected left side to be PLUS, got %#v", top.Left)
	}
	if _, ok := left.Left.(*ast.Identifier); !ok {
		t.Fatalf("expected identifier 'a' on far left, got %#v", left.Left)
	}
	mul, ok := left.Right.(*ast.BinaryExpr)
	if !ok || mul.Op != token.STAR {
		t.Fatalf("expected b*2 to be STAR, got %#v", left.Right)
	}

	right, ok := top.Right.(*ast.BinaryExpr)
	if !ok || right.Op != token.DSLASH {
		t.Fatalf("expected right side to be DSLASH (//), got %#v", top.Right)
	}
}

func TestIfElifElseWithCorrectDedent(t *testing.T) {
	src := "" +
		"if a:\n" +
		"    x = 1\n" +
		"elif b:\n" +
		"    x = 2\n" +
		"else:\n" +
		"    x = 3\n" +
		"y = 4\n"
	prog := parseSrc(t, src)
	if len(prog.Statements) != 2 {
		t.Fatalf("expected 2 top-level statements (if-block + y=4), got %d", len(prog.Statements))
	}
	ifStmt, ok := prog.Statements[0].(*ast.IfStmt)
	if !ok {
		t.Fatalf("expected *ast.IfStmt, got %T", prog.Statements[0])
	}
	if len(ifStmt.Then) != 1 || len(ifStmt.Elifs) != 1 || len(ifStmt.Else) != 1 {
		t.Fatalf("if-block shape wrong: then=%d elifs=%d else=%d",
			len(ifStmt.Then), len(ifStmt.Elifs), len(ifStmt.Else))
	}
	if _, ok := prog.Statements[1].(*ast.AssignStmt); !ok {
		t.Fatalf("expected y=4 to be parsed as a separate top-level AssignStmt, got %T", prog.Statements[1])
	}
}

func TestWhileLoop(t *testing.T) {
	prog := parseSrc(t, "while x < 10:\n    x += 1\n")
	ws, ok := prog.Statements[0].(*ast.WhileStmt)
	if !ok {
		t.Fatalf("expected *ast.WhileStmt, got %T", prog.Statements[0])
	}
	cond, ok := ws.Cond.(*ast.BinaryExpr)
	if !ok || cond.Op != token.LT {
		t.Fatalf("expected LT condition, got %#v", ws.Cond)
	}
	body, ok := ws.Body[0].(*ast.AssignStmt)
	if !ok || body.Op != token.PLUSEQ {
		t.Fatalf("expected PLUSEQ body statement, got %#v", ws.Body[0])
	}
}

func TestForRangeOneArg(t *testing.T) {
	prog := parseSrc(t, "for i in range(5):\n    print(i)\n")
	fs, ok := prog.Statements[0].(*ast.ForStmt)
	if !ok {
		t.Fatalf("expected *ast.ForStmt, got %T", prog.Statements[0])
	}
	if fs.VarName != "i" || fs.Stop != nil {
		t.Fatalf("expected one-arg range(5) with Stop==nil, got VarName=%s Stop=%#v", fs.VarName, fs.Stop)
	}
}

func TestForRangeTwoArgs(t *testing.T) {
	prog := parseSrc(t, "for i in range(1, 10):\n    print(i)\n")
	fs := prog.Statements[0].(*ast.ForStmt)
	if fs.Stop == nil {
		t.Fatalf("expected two-arg range(1, 10) with a non-nil Stop")
	}
}

func TestFuncDeclWithSingleReturnType(t *testing.T) {
	prog := parseSrc(t, "def add(a: int, b: int) -> int:\n    return a + b\n")
	fn, ok := prog.Statements[0].(*ast.FuncDecl)
	if !ok {
		t.Fatalf("expected *ast.FuncDecl, got %T", prog.Statements[0])
	}
	if fn.Name != "add" || len(fn.Params) != 2 {
		t.Fatalf("wrong name/param count: name=%s params=%d", fn.Name, len(fn.Params))
	}
	if fn.Params[0].Name != "a" || fn.Params[0].Type.Name != "int" {
		t.Fatalf("wrong first param: %#v", fn.Params[0])
	}
	if len(fn.ReturnTypes) != 1 || fn.ReturnTypes[0].Name != "int" {
		t.Fatalf("wrong return types: %#v", fn.ReturnTypes)
	}
	ret, ok := fn.Body[0].(*ast.ReturnStmt)
	if !ok || len(ret.Values) != 1 {
		t.Fatalf("expected single-value return statement, got %#v", fn.Body[0])
	}
}

func TestFuncDeclWithMultipleReturnTypes(t *testing.T) {
	prog := parseSrc(t, "def divmod2(a: int, b: int) -> (int, int):\n    return a // b, a % b\n")
	fn := prog.Statements[0].(*ast.FuncDecl)
	if len(fn.ReturnTypes) != 2 {
		t.Fatalf("expected 2 return types, got %d", len(fn.ReturnTypes))
	}
	ret := fn.Body[0].(*ast.ReturnStmt)
	if len(ret.Values) != 2 {
		t.Fatalf("expected 2 return values, got %d", len(ret.Values))
	}
}

func TestBreakAndContinue(t *testing.T) {
	prog := parseSrc(t, "while True:\n    break\n")
	ws := prog.Statements[0].(*ast.WhileStmt)
	if _, ok := ws.Body[0].(*ast.BreakStmt); !ok {
		t.Fatalf("expected BreakStmt, got %T", ws.Body[0])
	}

	prog2 := parseSrc(t, "while True:\n    continue\n")
	ws2 := prog2.Statements[0].(*ast.WhileStmt)
	if _, ok := ws2.Body[0].(*ast.ContinueStmt); !ok {
		t.Fatalf("expected ContinueStmt, got %T", ws2.Body[0])
	}
}

func TestListDictTupleLiterals(t *testing.T) {
	prog := parseSrc(t, "nums = [1, 2, 3]\n")
	assign := prog.Statements[0].(*ast.AssignStmt)
	list, ok := assign.Value.(*ast.ListLiteral)
	if !ok || len(list.Elements) != 3 {
		t.Fatalf("expected 3-element ListLiteral, got %#v", assign.Value)
	}

	prog2 := parseSrc(t, `d = {"a": 1, "b": 2}` + "\n")
	assign2 := prog2.Statements[0].(*ast.AssignStmt)
	dict, ok := assign2.Value.(*ast.DictLiteral)
	if !ok || len(dict.Keys) != 2 {
		t.Fatalf("expected 2-entry DictLiteral, got %#v", assign2.Value)
	}

	prog3 := parseSrc(t, `t = (1, "x")`+"\n")
	assign3 := prog3.Statements[0].(*ast.AssignStmt)
	tup, ok := assign3.Value.(*ast.TupleLiteral)
	if !ok || len(tup.Elements) != 2 {
		t.Fatalf("expected 2-element TupleLiteral, got %#v", assign3.Value)
	}
}

func TestTupleUnpackingAssignment(t *testing.T) {
	prog := parseSrc(t, "a, b = t\n")
	assign := prog.Statements[0].(*ast.AssignStmt)
	if len(assign.Targets) != 2 {
		t.Fatalf("expected 2 assignment targets, got %d", len(assign.Targets))
	}
}

func TestTernaryExpression(t *testing.T) {
	prog := parseSrc(t, "y = x if x > 0 else -x\n")
	assign := prog.Statements[0].(*ast.AssignStmt)
	tern, ok := assign.Value.(*ast.TernaryExpr)
	if !ok {
		t.Fatalf("expected *ast.TernaryExpr, got %T", assign.Value)
	}
	if _, ok := tern.Cond.(*ast.BinaryExpr); !ok {
		t.Fatalf("expected condition to be a BinaryExpr, got %#v", tern.Cond)
	}
	if _, ok := tern.Else.(*ast.UnaryExpr); !ok {
		t.Fatalf("expected else-branch to be a UnaryExpr (-x), got %#v", tern.Else)
	}
}

func TestMethodCallAndAttributeChaining(t *testing.T) {
	prog := parseSrc(t, "s = a.b.upper()\n")
	assign := prog.Statements[0].(*ast.AssignStmt)
	call, ok := assign.Value.(*ast.CallExpr)
	if !ok {
		t.Fatalf("expected *ast.CallExpr, got %T", assign.Value)
	}
	attr, ok := call.Callee.(*ast.AttributeExpr)
	if !ok || attr.Attr != "upper" {
		t.Fatalf("expected callee to be .upper attribute access, got %#v", call.Callee)
	}
	inner, ok := attr.Object.(*ast.AttributeExpr)
	if !ok || inner.Attr != "b" {
		t.Fatalf("expected chained .b attribute access, got %#v", attr.Object)
	}
}

func TestSlicing(t *testing.T) {
	prog := parseSrc(t, "y = nums[1:3]\n")
	assign := prog.Statements[0].(*ast.AssignStmt)
	sl, ok := assign.Value.(*ast.SliceExpr)
	if !ok || sl.Low == nil || sl.High == nil {
		t.Fatalf("expected a full slice with both bounds, got %#v", assign.Value)
	}
}

func TestBooleanOperatorsAndUnaryNot(t *testing.T) {
	prog := parseSrc(t, "y = not a and b or c\n")
	assign := prog.Statements[0].(*ast.AssignStmt)
	// "or" binds loosest, so top level must be OR.
	top, ok := assign.Value.(*ast.BinaryExpr)
	if !ok || top.Op != token.OR {
		t.Fatalf("expected top-level OR, got %#v", assign.Value)
	}
	// left side of the OR: (not a) and b
	left, ok := top.Left.(*ast.BinaryExpr)
	if !ok || left.Op != token.AND {
		t.Fatalf("expected AND under OR, got %#v", top.Left)
	}
	if _, ok := left.Left.(*ast.UnaryExpr); !ok {
		t.Fatalf("expected 'not a' as a UnaryExpr, got %#v", left.Left)
	}
}

// TestErrorRecoveryContinuesParsing verifies the panic-mode recovery
// philosophy: one malformed statement should not prevent the rest of
// the file from being parsed, and the parser must terminate (not hang)
// on bad input.
func TestErrorRecoveryContinuesParsing(t *testing.T) {
	src := "x = 1\n" +
		"y = = 2\n" + // malformed: stray extra '='
		"z = 3\n"

	toks, lexErrs := lexer.New(src).Tokenize()
	if len(lexErrs) != 0 {
		t.Fatalf("unexpected lex errors: %v", lexErrs)
	}
	prog, parseErrs := New(toks).ParseProgram()

	if len(parseErrs) == 0 {
		t.Fatalf("expected at least one parse error for malformed input")
	}
	// The key claim under test: parsing did not hang, and it recovered
	// enough to still find the well-formed statements around the bad one.
	foundX, foundZ := false, false
	for _, s := range prog.Statements {
		if as, ok := s.(*ast.AssignStmt); ok {
			if id, ok := as.Targets[0].(*ast.Identifier); ok {
				if id.Name == "x" {
					foundX = true
				}
				if id.Name == "z" {
					foundZ = true
				}
			}
		}
	}
	if !foundX || !foundZ {
		t.Fatalf("expected recovery to still find x=1 and z=3 around the bad line; got %d statements", len(prog.Statements))
	}
}

func TestUnaryMinusAndParenthesizedExpression(t *testing.T) {
	prog := parseSrc(t, "y = -(a + b)\n")
	assign := prog.Statements[0].(*ast.AssignStmt)
	unary, ok := assign.Value.(*ast.UnaryExpr)
	if !ok || unary.Op != token.MINUS {
		t.Fatalf("expected top-level unary MINUS, got %#v", assign.Value)
	}
	if _, ok := unary.Operand.(*ast.BinaryExpr); !ok {
		t.Fatalf("expected parenthesized (a+b) to parse as a BinaryExpr, got %#v", unary.Operand)
	}
}
