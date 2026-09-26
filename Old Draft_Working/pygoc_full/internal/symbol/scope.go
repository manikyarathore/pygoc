// Package symbol implements PyGoC's scoped symbol table (Phase 3a).
//
// Design note carried over from the roadmap: in Python-family semantics,
// if/while/for bodies do NOT introduce a new variable scope — only
// functions (and the global scope) do. So there are exactly two
// ScopeKinds, and the semantic analyzer never creates a new Scope for a
// block body, only for a function body.
package symbol

// ScopeKind distinguishes the two kinds of scope PyGo has.
type ScopeKind int

const (
	GlobalScope ScopeKind = iota
	FunctionScope
)

// Scope holds name->Symbol bindings for one lexical scope, plus a
// pointer to its enclosing scope (nil for the global scope).
type Scope struct {
	Parent  *Scope
	Kind    ScopeKind
	symbols map[string]*Symbol
}

// NewScope creates a scope with the given parent and kind.
func NewScope(parent *Scope, kind ScopeKind) *Scope {
	return &Scope{Parent: parent, Kind: kind, symbols: make(map[string]*Symbol)}
}

// Define adds a new symbol to THIS scope. Returns an error if a symbol
// with the same name already exists in this exact scope (shadowing an
// outer-scope symbol is fine and not an error; redefining within the
// same scope is).
func (s *Scope) Define(sym *Symbol) error {
	if _, exists := s.symbols[sym.Name]; exists {
		return &RedefinitionError{Name: sym.Name, Line: sym.Line, Col: sym.Col}
	}
	s.symbols[sym.Name] = sym
	return nil
}

// Resolve looks up a name, walking outward through parent scopes.
func (s *Scope) Resolve(name string) (*Symbol, bool) {
	for sc := s; sc != nil; sc = sc.Parent {
		if sym, ok := sc.symbols[name]; ok {
			return sym, true
		}
	}
	return nil, false
}

// DefinedLocally reports whether name is defined in exactly this scope
// (not an outer one).
func (s *Scope) DefinedLocally(name string) bool {
	_, ok := s.symbols[name]
	return ok
}

// RedefinitionError is returned by Define when a name is already bound
// in the same scope.
type RedefinitionError struct {
	Name      string
	Line, Col int
}

func (e *RedefinitionError) Error() string {
	return "'" + e.Name + "' is already defined in this scope"
}
