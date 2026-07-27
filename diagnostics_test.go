package selena

import (
	"testing"

	"m31labs.dev/selena/ir"
)

// TestCompileErrorWrapsEmitError is the direct test for compileError's
// *ir.EmitError branch: an emission-time failure must produce the same
// structured Diagnostic shape (Code, Message, Hint) that a parse or lower
// failure already gets, matching the style of every other SEL diagnostic —
// see TestCompileErrorRejectsReservedNames in compile_test.go for the
// lower.DiagnosticError equivalent this mirrors. Range is zero because
// ir.Module carries no source spans (see ir.EmitError's doc comment), not
// because the mapping forgot one.
func TestCompileErrorWrapsEmitError(t *testing.T) {
	inner := &ir.EmitError{Code: ir.CodeEmitUnknownExpr, Message: "cannot render IR expression of type ir.someFutureExpr"}
	got := compileError(inner)

	ce, ok := got.(*CompileError)
	if !ok {
		t.Fatalf("compileError(*ir.EmitError) type = %T, want *CompileError", got)
	}
	if len(ce.Diagnostics) != 1 {
		t.Fatalf("diagnostics = %+v, want exactly 1", ce.Diagnostics)
	}
	d := ce.Diagnostics[0]
	if d.Code != ir.CodeEmitUnknownExpr {
		t.Fatalf("diagnostic code = %s, want %s", d.Code, ir.CodeEmitUnknownExpr)
	}
	if d.Message != inner.Message {
		t.Fatalf("diagnostic message = %q, want %q", d.Message, inner.Message)
	}
	if !d.Range.IsZero() {
		t.Fatalf("diagnostic range = %+v, want zero (ir.Module carries no source spans)", d.Range)
	}
	if d.Hint == "" {
		t.Fatal("diagnostic hint is empty, want a hint (see diagnosticHint's SEL4001-SEL4003 case)")
	}
	if got.Error() == "" {
		t.Fatal("CompileError.Error() is empty")
	}
}

// TestCompileErrorNilIsNil guards the common early-return path compileError
// shares across all its branches.
func TestCompileErrorNilIsNil(t *testing.T) {
	if got := compileError(nil); got != nil {
		t.Fatalf("compileError(nil) = %v, want nil", got)
	}
}
