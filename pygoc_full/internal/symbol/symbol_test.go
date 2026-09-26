package symbol

import "testing"

func TestDefineAndResolveGlobal(t *testing.T) {
	tbl := NewTable()
	if err := tbl.Define(&Symbol{Name: "x", Kind: VarSymbol}); err != nil {
		t.Fatalf("unexpected error defining x: %v", err)
	}
	sym, ok := tbl.Resolve("x")
	if !ok || sym.Name != "x" {
		t.Fatalf("expected to resolve x, got ok=%v sym=%v", ok, sym)
	}
}

func TestRedefinitionInSameScopeErrors(t *testing.T) {
	tbl := NewTable()
	tbl.Define(&Symbol{Name: "x", Kind: VarSymbol})
	if err := tbl.Define(&Symbol{Name: "x", Kind: VarSymbol}); err == nil {
		t.Fatalf("expected redefinition error, got nil")
	}
}

func TestResolveUnknownNameFails(t *testing.T) {
	tbl := NewTable()
	if _, ok := tbl.Resolve("nope"); ok {
		t.Fatalf("expected nope to be unresolved")
	}
}

// TestScopeChainCases loops over several nested-scope scenarios rather
// than writing one test function per case.
func TestScopeChainCases(t *testing.T) {
	cases := []struct {
		name      string
		setup     func(tbl *Table)
		lookup    string
		wantFound bool
	}{
		{
			name: "function scope shadows global",
			setup: func(tbl *Table) {
				tbl.Define(&Symbol{Name: "x", Kind: VarSymbol})
				tbl.EnterFunctionScope()
				tbl.Define(&Symbol{Name: "x", Kind: ParamSymbol})
			},
			lookup:    "x",
			wantFound: true,
		},
		{
			name: "function scope can see global",
			setup: func(tbl *Table) {
				tbl.Define(&Symbol{Name: "g", Kind: VarSymbol})
				tbl.EnterFunctionScope()
			},
			lookup:    "g",
			wantFound: true,
		},
		{
			name: "exiting function scope hides its locals",
			setup: func(tbl *Table) {
				tbl.EnterFunctionScope()
				tbl.Define(&Symbol{Name: "local", Kind: VarSymbol})
				tbl.ExitScope()
			},
			lookup:    "local",
			wantFound: false,
		},
		{
			name: "nested function scopes chain correctly",
			setup: func(tbl *Table) {
				tbl.Define(&Symbol{Name: "outer", Kind: VarSymbol})
				tbl.EnterFunctionScope()
				tbl.Define(&Symbol{Name: "mid", Kind: VarSymbol})
				tbl.EnterFunctionScope()
			},
			lookup:    "outer",
			wantFound: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tbl := NewTable()
			c.setup(tbl)
			_, found := tbl.Resolve(c.lookup)
			if found != c.wantFound {
				t.Fatalf("resolve(%q) = %v, want %v", c.lookup, found, c.wantFound)
			}
		})
	}
}

func TestIsLocalAndIsCaptured(t *testing.T) {
	tbl := NewTable()
	tbl.Define(&Symbol{Name: "g", Kind: VarSymbol}) // global

	tbl.EnterFunctionScope()
	tbl.Define(&Symbol{Name: "outerVar", Kind: VarSymbol}) // outer function local

	tbl.EnterFunctionScope() // nested function
	tbl.Define(&Symbol{Name: "innerVar", Kind: VarSymbol})

	if !tbl.IsLocal("innerVar") {
		t.Fatalf("expected innerVar to be local to the innermost scope")
	}
	if tbl.IsLocal("outerVar") {
		t.Fatalf("expected outerVar NOT to be local (it's captured)")
	}
	if !tbl.IsCaptured("outerVar") {
		t.Fatalf("expected outerVar to be captured from the enclosing function scope")
	}
	if tbl.IsCaptured("g") {
		t.Fatalf("expected g NOT to be 'captured' — it's global, not from an enclosing FUNCTION scope")
	}
}

func TestExitScopeAtGlobalIsNoOp(t *testing.T) {
	tbl := NewTable()
	tbl.Define(&Symbol{Name: "x", Kind: VarSymbol})
	tbl.ExitScope() // should not panic or lose the global scope
	if _, ok := tbl.Resolve("x"); !ok {
		t.Fatalf("expected x still resolvable after a no-op ExitScope at global level")
	}
}
