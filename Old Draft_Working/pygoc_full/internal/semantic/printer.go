package semantic

import (
	"fmt"
	"sort"
	"strings"

	"github.com/manikyarathore/pygoc/internal/ast"
	"github.com/manikyarathore/pygoc/internal/symbol"
	"github.com/manikyarathore/pygoc/internal/types"
)

// entry is one row of the printed symbol table.
type entry struct {
	Name  string
	Type  string
	Scope string
}

// PrintSymbolTable renders every symbol the analyzer collected in its
// GLOBAL scope, in the format specified by the roadmap's output
// contract for `pygoc sema`. (Function-local/parameter symbols live in
// scopes that have already been exited by the time analysis finishes,
// which is expected — this view matches what the top level of the
// program actually declares.)
func (a *Analyzer) PrintSymbolTable() string {
	var rows []entry
	// Walking the map isn't exposed by symbol.Scope on purpose (it's an
	// implementation detail), so we recover the printable rows the only
	// way the public API allows: by re-resolving each name the caller
	// already knows about is awkward, so instead the analyzer keeps its
	// own ordered record as it defines symbols. See noteOrder below.
	for _, name := range a.definedOrder {
		sym, ok := a.Table.Resolve(name)
		if !ok {
			continue
		}
		t, _ := sym.Type.(types.Type)
		scopeName := "global"
		rows = append(rows, entry{Name: sym.Name, Type: t.String(), Scope: scopeName})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })

	var sb strings.Builder
	sb.WriteString("Symbol Table\n")
	sb.WriteString("────────────────────────\n")
	sb.WriteString(fmt.Sprintf("%-10s%-10s%s\n", "Name", "Type", "Scope"))
	sb.WriteString("────────────────────────\n")
	for _, r := range rows {
		sb.WriteString(fmt.Sprintf("%-10s%-10s%s\n", r.Name, r.Type, r.Scope))
	}
	sb.WriteString("────────────────────────\n")
	return sb.String()
}

// PrintTypeAnalysis renders one "statement → type" line per top-level
// statement, matching the roadmap's output contract.
func PrintTypeAnalysis(prog *ast.Program, tbl *symbol.Table) string {
	var sb strings.Builder
	sb.WriteString("Type Analysis\n")
	for _, s := range prog.Statements {
		switch n := s.(type) {
		case *ast.AssignStmt:
			t, err := types.Infer(n.Value, tbl)
			if err != nil {
				sb.WriteString(fmt.Sprintf("%s → error: %s\n", stmtSrcHint(n), err))
				continue
			}
			sb.WriteString(fmt.Sprintf("%-15s → %s\n", stmtSrcHint(n), t))
		case *ast.ExprStmt:
			_, err := types.Infer(n.X, tbl)
			if err != nil {
				sb.WriteString(fmt.Sprintf("%s → error: %s\n", stmtSrcHint(n), err))
				continue
			}
			sb.WriteString(fmt.Sprintf("%-15s → valid\n", stmtSrcHint(n)))
		case *ast.FuncDecl:
			sb.WriteString(fmt.Sprintf("def %-11s → declared\n", n.Name))
		}
	}
	return sb.String()
}

// stmtSrcHint gives a short, readable label for a statement without
// needing a full unparse-to-source pass — good enough for the debug
// output this powers.
func stmtSrcHint(s ast.Stmt) string {
	switch n := s.(type) {
	case *ast.AssignStmt:
		if id, ok := n.Targets[0].(*ast.Identifier); ok {
			return id.Name + " " + n.Op.String() + " ..."
		}
		return "assignment"
	case *ast.ExprStmt:
		if call, ok := n.X.(*ast.CallExpr); ok {
			if fn, ok := call.Callee.(*ast.Identifier); ok {
				return fn.Name + "(...)"
			}
		}
		return "expression"
	default:
		return "statement"
	}
}
