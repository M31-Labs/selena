package ir

import (
	"testing"
)

// unrenderableExprForTest is an Expr implementation with no case in Print's
// switch, on purpose. Expr's unexported isExpr marker means only this package
// can define one, so this lives beside the code it exercises rather than in
// emit_error.go (no other package needs to construct an "unrecognized" Expr —
// see ir.UnrenderableStmtCFForTests's doc comment for why the StmtCF
// counterpart does need to be exported).
type unrenderableExprForTest struct{}

func (unrenderableExprForTest) isExpr() {}

// TestPrintPanicsWithEmitErrorForUnknownExpr is the direct test for
// CodeEmitUnknownExpr: Print's default case used to silently return the
// string "/* unknown expr */" for any Expr variant it did not recognize —
// exactly the "compile succeeds, emitted shader does not match the source"
// failure class this project exists to remove. It now panics with a
// structured *EmitError instead; each backend's top-level Emit function
// recovers this into a normal error return (see emit/wgsl, emit/metal,
// emit/glsl, emit/gles).
func TestPrintPanicsWithEmitErrorForUnknownExpr(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("Print did not panic for an unrecognized Expr variant")
		}
		ee, ok := r.(*EmitError)
		if !ok {
			t.Fatalf("panic value type = %T, want *EmitError", r)
		}
		if ee.Code != CodeEmitUnknownExpr {
			t.Fatalf("EmitError.Code = %s, want %s", ee.Code, CodeEmitUnknownExpr)
		}
		if ee.Message == "" {
			t.Fatal("EmitError.Message is empty, want a message naming the offending type")
		}
	}()
	Print(unrenderableExprForTest{}, testDialect{})
}

// TestEmitErrorSatisfiesError checks EmitError's Error() rendering, matching
// the style of every other structured Selena diagnostic (lower.DiagnosticError,
// parse.Error): "<code>: <message>".
func TestEmitErrorSatisfiesError(t *testing.T) {
	err := &EmitError{Code: "SEL4001", Message: "cannot render IR expression of type ir.unrenderableExprForTest"}
	want := "SEL4001: cannot render IR expression of type ir.unrenderableExprForTest"
	if err.Error() != want {
		t.Fatalf("EmitError.Error() = %q, want %q", err.Error(), want)
	}
}
