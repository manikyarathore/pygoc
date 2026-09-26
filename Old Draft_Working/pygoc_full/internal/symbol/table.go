package symbol

// SymbolKind distinguishes what a bound name refers to.
type SymbolKind int

const (
	VarSymbol SymbolKind = iota
	ParamSymbol
	FuncSymbol
)

// Symbol is one entry in a Scope.
//
// Type is `any` deliberately: this package (Phase 3a) is built before
// internal/types (Phase 3b) exists in the pipeline, so the symbol table
// itself doesn't need to know what a type IS — only that each symbol
// has one. The semantic analyzer (Phase 3c) populates this field with a
// real types.Type value once both packages exist.
type Symbol struct {
	Name string
	Type any
	Kind SymbolKind
	Line int
	Col  int
}

// Table wraps a "current scope" cursor that the semantic analyzer
// pushes/pops as it walks into and out of function bodies. It is the
// primary type the rest of the compiler interacts with — Scope itself
// is an implementation detail of the chain.
type Table struct {
	current *Scope
}

// NewTable creates a Table with a fresh global scope.
func NewTable() *Table {
	return &Table{current: NewScope(nil, GlobalScope)}
}

// EnterFunctionScope pushes a new function-level scope as a child of
// the current one.
func (t *Table) EnterFunctionScope() {
	t.current = NewScope(t.current, FunctionScope)
}

// ExitScope pops back to the parent scope. A no-op at the global scope
// (there's nothing to pop to).
func (t *Table) ExitScope() {
	if t.current.Parent != nil {
		t.current = t.current.Parent
	}
}

// Define binds a symbol in the current scope.
func (t *Table) Define(sym *Symbol) error {
	return t.current.Define(sym)
}

// Resolve looks up a name through the current scope chain.
func (t *Table) Resolve(name string) (*Symbol, bool) {
	return t.current.Resolve(name)
}

// IsLocal reports whether name is bound in the CURRENT scope
// specifically (not an outer one).
func (t *Table) IsLocal(name string) bool {
	return t.current.DefinedLocally(name)
}

// IsCaptured reports whether name resolves to a binding from an
// ENCLOSING FUNCTION scope (as opposed to being local, or global). This
// is exactly the primitive PyGo's read-only-closure rule needs: a
// nested function may read a captured variable but never reassign it.
func (t *Table) IsCaptured(name string) bool {
	if t.current.DefinedLocally(name) {
		return false
	}
	for sc := t.current.Parent; sc != nil; sc = sc.Parent {
		if sc.DefinedLocally(name) {
			return sc.Kind == FunctionScope
		}
	}
	return false
}

// CurrentScope exposes the raw scope, for callers that need Kind or
// direct chain access (e.g. checking whether we're at global scope).
func (t *Table) CurrentScope() *Scope {
	return t.current
}

// InGlobalScope reports whether we're currently at the top-level scope.
func (t *Table) InGlobalScope() bool {
	return t.current.Kind == GlobalScope
}
