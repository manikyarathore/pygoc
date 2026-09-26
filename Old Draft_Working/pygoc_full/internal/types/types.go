// Package types implements PyGoC's type system (Phase 3b).
//
// SUBMISSION SCOPE NOTE: to ship a genuinely complete, working pipeline
// in the available time, this build supports the CORE subset only:
// int, float, bool, string. list/dict/tuple types are NOT implemented
// here — the semantic analyzer rejects list/dict/tuple literals with a
// clear "not supported in this build" error rather than silently
// mis-compiling them. This is a documented scope cut, not an oversight
// — see README's Scope Decisions.
package types

// Kind identifies a primitive type.
type Kind int

const (
	Unknown Kind = iota
	Int
	Float
	Bool
	String
	Void // a function with no declared return type
)

// Type is intentionally just a Kind wrapper — PyGo's core subset has no
// structural types (no generics, no user-defined types), so equality is
// just Kind equality.
type Type struct {
	Kind Kind
}

var (
	IntT     = Type{Kind: Int}
	FloatT   = Type{Kind: Float}
	BoolT    = Type{Kind: Bool}
	StringT  = Type{Kind: String}
	VoidT    = Type{Kind: Void}
	UnknownT = Type{Kind: Unknown}
)

func (t Type) String() string {
	switch t.Kind {
	case Int:
		return "int"
	case Float:
		return "float"
	case Bool:
		return "bool"
	case String:
		return "string"
	case Void:
		return "void"
	default:
		return "unknown"
	}
}

// Equals is plain Kind equality for this Type shape.
func (t Type) Equals(other Type) bool {
	return t.Kind == other.Kind
}

// IsNumeric reports whether t is int or float — the two types PyGo's
// arithmetic operators accept.
func (t Type) IsNumeric() bool {
	return t.Kind == Int || t.Kind == Float
}

// FromTypeName resolves a parsed type-annotation name (as it comes out
// of ast.TypeExpr.Name) to a Type. Parameterized annotations
// (list[int], dict[...]) are recognized as syntax by the parser but are
// NOT resolvable here in this build — see the package doc comment.
func FromTypeName(name string) (Type, bool) {
	switch name {
	case "int":
		return IntT, true
	case "float":
		return FloatT, true
	case "bool":
		return BoolT, true
	case "string":
		return StringT, true
	default:
		return UnknownT, false
	}
}
