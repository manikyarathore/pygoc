// Package codegen implements Phase 6: walks the OPTIMIZED IR (module
// produced by optimizer.RunModule) and emits idiomatic, gofmt-clean Go
// source.
//
// STRATEGY NOTE: because the IR is already a labeled-block CFG with
// explicit Goto/IfGoto instructions, codegen does NOT attempt to
// "restructure" the control flow back into nested if/while Go syntax
// (that reconstruction is a genuinely hard, separate algorithm). Go
// itself supports goto and labels natively, so codegen emits Go that
// mirrors the CFG almost one-to-one: every IR block becomes a Go label,
// every IfGoto becomes `if cond { goto L1 } else { goto L2 }`. The one
// trick this requires: every variable (including every temp) is
// declared with `var name Type` at the TOP of the function, before any
// label — Go forbids a `goto` from jumping over a variable's first
// declaration into its scope, and declaring everything up front sides-
// steps that restriction entirely, since no declaration ever appears
// after a label.
package codegen

import "github.com/manikyarathore/pygoc/internal/types"

// GoTypeName maps a PyGo type to its Go spelling. This lives in codegen
// (not internal/types) deliberately — the type system itself has no
// business knowing about Go's specific type names; that's a codegen
// concern.
func GoTypeName(t types.Type) string {
	switch t.Kind {
	case types.Int:
		return "int"
	case types.Float:
		return "float64"
	case types.Bool:
		return "bool"
	case types.String:
		return "string"
	default:
		return "interface{}" // should not occur for a semantically-validated program
	}
}
