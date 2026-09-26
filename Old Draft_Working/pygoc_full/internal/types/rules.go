package types

import (
	"fmt"

	"github.com/manikyarathore/pygoc/internal/token"
)

// TypeError is returned when an operator is applied to incompatible
// operand types.
type TypeError struct {
	Msg string
}

func (e *TypeError) Error() string { return e.Msg }

// BinaryOpResult applies PyGo's operator type rules and returns the
// result type of `left op right`, or an error if the combination is
// invalid.
//
// Rules (mirroring docs/language-spec.md):
//
//	int + int       -> int
//	float + float   -> float
//	int + float     -> float   (implicit widening, either direction)
//	int / int       -> float   (true division, like Python)
//	int // int      -> int     (floor division)
//	string + string -> string  (concatenation; no other string arithmetic)
//	comparisons     -> bool, operands must be the same numeric-or-string kind
//	and / or        -> bool, operands must both be bool
func BinaryOpResult(op token.Type, left, right Type) (Type, error) {
	switch op {
	case token.PLUS:
		if left.Kind == String && right.Kind == String {
			return StringT, nil
		}
		return numericResult(op, left, right)

	case token.MINUS, token.STAR, token.PERCENT:
		return numericResult(op, left, right)

	case token.SLASH:
		// true division always produces a float, matching Python's `/`
		if !left.IsNumeric() || !right.IsNumeric() {
			return UnknownT, mismatchErr(op, left, right)
		}
		return FloatT, nil

	case token.DSLASH:
		// floor division: int // int -> int; anything involving a float -> float
		if !left.IsNumeric() || !right.IsNumeric() {
			return UnknownT, mismatchErr(op, left, right)
		}
		if left.Kind == Int && right.Kind == Int {
			return IntT, nil
		}
		return FloatT, nil

	case token.EQ, token.NEQ, token.LT, token.GT, token.LE, token.GE:
		if left.Kind != right.Kind && !(left.IsNumeric() && right.IsNumeric()) {
			return UnknownT, mismatchErr(op, left, right)
		}
		return BoolT, nil

	case token.AND, token.OR:
		if left.Kind != Bool || right.Kind != Bool {
			return UnknownT, mismatchErr(op, left, right)
		}
		return BoolT, nil

	default:
		return UnknownT, &TypeError{Msg: fmt.Sprintf("unsupported binary operator %s", op)}
	}
}

func numericResult(op token.Type, left, right Type) (Type, error) {
	if !left.IsNumeric() || !right.IsNumeric() {
		return UnknownT, mismatchErr(op, left, right)
	}
	if left.Kind == Int && right.Kind == Int {
		return IntT, nil
	}
	return FloatT, nil
}

func mismatchErr(op token.Type, left, right Type) error {
	return &TypeError{Msg: fmt.Sprintf("cannot apply '%s' to %s and %s", op, left, right)}
}

// UnaryOpResult applies PyGo's unary operator rules.
func UnaryOpResult(op token.Type, operand Type) (Type, error) {
	switch op {
	case token.MINUS:
		if !operand.IsNumeric() {
			return UnknownT, &TypeError{Msg: fmt.Sprintf("cannot negate %s", operand)}
		}
		return operand, nil
	case token.NOT:
		if operand.Kind != Bool {
			return UnknownT, &TypeError{Msg: fmt.Sprintf("'not' requires bool, got %s", operand)}
		}
		return BoolT, nil
	default:
		return UnknownT, &TypeError{Msg: fmt.Sprintf("unsupported unary operator %s", op)}
	}
}
