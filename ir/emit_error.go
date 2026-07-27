package ir

import "fmt"

// Emission diagnostic codes. Parse (SEL0xxx) and lower (SEL1xxx-SEL3xxx)
// failures carry a code, a source span where one exists, and a hint — see
// diagnostics.go and lower/diagnostic.go. Backend emission had no equivalent:
// every emit/{wgsl,glsl,gles,metal}.Emit signature declared an error return,
// but nothing on any of those paths ever constructed one, so an emission-time
// failure could only be a silent placeholder (ir.Print's old "/* unknown expr
// */" fallback, or a Resolver method degrading to a hard-coded zero value) or
// an unrecovered Go panic. These codes give emission the same treatment.
//
// ir.Module carries no source spans (lowering intentionally strips them; see
// ir.Stmt/ir.Expr), so an EmitError's Range is always zero — an honestly empty
// span, not a missing one. selena.compileError renders that the same way it
// already renders any other span-less diagnostic.
const (
	// CodeEmitUnknownExpr reports that ir.Print reached an Expr
	// implementation its switch does not know how to render. Every Expr
	// variant in this package is handled today, so this is unreachable
	// unless a future variant's Print case is forgotten — the emission-side
	// counterpart to ir/uses.go's compile-time-exhaustive StmtCF/Expr walkers.
	CodeEmitUnknownExpr = "SEL4001"
	// CodeEmitUnknownStmt reports that a backend's statement emitter reached
	// a StmtCF implementation its switch does not know how to render. Every
	// StmtCF variant is handled today (see emit/internal/emit.go's
	// emitStmt); this is unreachable unless a future variant's case is
	// forgotten.
	CodeEmitUnknownStmt = "SEL4002"
	// CodeEmitUnwiredNode reports that an ir.Expr node reached a backend
	// Resolver stage that was never configured to render it (a SceneSample,
	// SceneSize, StateSample, StateSampleUV, or CellUV expression whose
	// corresponding Fn field is nil — see emit/internal/emit.go). lower/
	// only ever produces one of these nodes inside a stage/kind whose
	// emitters do wire the matching Fn, so this is unreachable for any
	// program lower/ accepts; it exists so a lowering bug that lets one leak
	// into the wrong stage fails loudly instead of silently rendering a
	// plausible-looking zero value.
	CodeEmitUnwiredNode = "SEL4003"
)

// EmitError is a structured backend emission diagnostic. Package ir raises it
// by panicking (see emitPanic); each backend's top-level Emit function
// recovers it and returns it as a normal error, so ir.Print and the shared
// statement emitter in emit/internal do not need an error return threaded
// through every call in the expression tree — see wgsl.Emit, metal.Emit,
// glsl.Emit, and gles.Emit's recover blocks.
type EmitError struct {
	Code    string
	Message string
}

func (e *EmitError) Error() string { return e.Code + ": " + e.Message }

// emitPanic raises an *EmitError as a panic. It is unexported: only this
// package's own emission fallback paths (ir.Print's default case) raise one
// directly; emit/internal raises the same type by constructing it directly
// (see emit/internal/emit.go), since panic/recover crosses package
// boundaries fine but emitPanic's call site does not need to.
func emitPanic(code, format string, args ...any) {
	panic(&EmitError{Code: code, Message: fmt.Sprintf(format, args...)})
}

// UnrenderableStmtCFForTests is a StmtCF implementation with no rendering in
// any backend emitter, on purpose. It exists only so emit/internal's test for
// CodeEmitUnknownStmt can exercise emitStmt's default case: StmtCF's
// unexported isStmtCF marker means only this package can define a variant of
// it, so a downstream package's test cannot construct an "unrecognized" one
// any other way. No lowerer or emitter ever produces this type — it must be
// constructed explicitly, as the test does.
type UnrenderableStmtCFForTests struct{}

func (UnrenderableStmtCFForTests) isStmtCF() {}
