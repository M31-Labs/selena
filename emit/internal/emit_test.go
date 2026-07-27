package internal

import (
	"errors"
	"strings"
	"testing"

	"m31labs.dev/selena/ir"
)

// assertEmitError fails t unless r is a *ir.EmitError with the given code,
// and returns it for further inspection.
func assertEmitError(t *testing.T, r any, wantCode string) *ir.EmitError {
	t.Helper()
	if r == nil {
		t.Fatal("did not panic, want a *ir.EmitError panic")
	}
	ee, ok := r.(*ir.EmitError)
	if !ok {
		t.Fatalf("panic value type = %T, want *ir.EmitError", r)
	}
	if ee.Code != wantCode {
		t.Fatalf("EmitError.Code = %s, want %s", ee.Code, wantCode)
	}
	if ee.Message == "" {
		t.Fatal("EmitError.Message is empty, want a useful message")
	}
	return ee
}

// TestEmitStmtPanicsWithEmitErrorForUnknownStmtCF is the direct test for
// CodeEmitUnknownStmt: emitStmt's switch used to have no default case, so an
// unrecognized StmtCF variant would silently fall through the whole function
// and drop the statement from the emitted source entirely — worse than a
// missing extension directive (see ir/uses.go's file doc comment for that
// sibling defect), since nothing about the emitted shader signals a statement
// went missing. It now panics with a structured *ir.EmitError instead; each
// backend's top-level Emit function recovers this into a normal error return
// via Recover/RecoverSplit below.
func TestEmitStmtPanicsWithEmitErrorForUnknownStmtCF(t *testing.T) {
	defer func() {
		assertEmitError(t, recover(), ir.CodeEmitUnknownStmt)
	}()
	var b strings.Builder
	EmitStmtList(&b, []ir.Stmt{{CF: ir.UnrenderableStmtCFForTests{}}}, Resolver{}, "  ", true)
}

// TestResolverSceneSamplePanicsWhenUnwired, TestResolverSceneSizePanicsWhenUnwired,
// TestResolverStateSamplePanicsWhenUnwired, TestResolverStateSampleUVPanicsWhenUnwired,
// and TestResolverCellUVPanicsWhenUnwired are the direct tests for
// CodeEmitUnwiredNode: each of these five Resolver methods used to return a
// plausible-looking zero-value expression ("vec4<f32>(0.0)", "vec2(0.0,
// 0.0)") when its corresponding *Fn field was nil, commented "unreachable in
// valid programs; guard only". lower/ never produces one of these ir.Expr
// nodes inside a stage whose emitter leaves the matching Fn nil, so the guard
// was never meant to be reachable — but a silent zero value is exactly the
// "compile succeeds, shader is wrong" failure this project exists to remove,
// so a lowering bug that let one leak through would have rendered a
// black/zero pixel with no diagnostic anywhere. Each now panics instead.
func TestResolverSceneSamplePanicsWhenUnwired(t *testing.T) {
	defer func() { assertEmitError(t, recover(), ir.CodeEmitUnwiredNode) }()
	Resolver{}.SceneSample("sceneColor", "uv")
}

func TestResolverSceneSizePanicsWhenUnwired(t *testing.T) {
	defer func() { assertEmitError(t, recover(), ir.CodeEmitUnwiredNode) }()
	Resolver{}.SceneSize()
}

func TestResolverStateSamplePanicsWhenUnwired(t *testing.T) {
	defer func() { assertEmitError(t, recover(), ir.CodeEmitUnwiredNode) }()
	Resolver{}.StateSample(0, 0)
}

func TestResolverStateSampleUVPanicsWhenUnwired(t *testing.T) {
	defer func() { assertEmitError(t, recover(), ir.CodeEmitUnwiredNode) }()
	Resolver{}.StateSampleUV("uv")
}

func TestResolverCellUVPanicsWhenUnwired(t *testing.T) {
	defer func() { assertEmitError(t, recover(), ir.CodeEmitUnwiredNode) }()
	Resolver{}.CellUV()
}

// TestResolverSceneSampleLevelDegradesInsteadOfPanicking guards against an
// over-broad fix: SceneSampleLevel is an intentional degrade (falls back to
// the implicit-LOD SceneSample form when SceneSampleLevelFn is nil, per its
// doc comment), not a "should never happen" guard, so it must not be swept
// into the CodeEmitUnwiredNode treatment the other five methods got. This
// documents that boundary and would fail if SceneSampleLevel started
// panicking too.
func TestResolverSceneSampleLevelDegradesInsteadOfPanicking(t *testing.T) {
	got := Resolver{SceneSampleFn: func(name, uv string) string { return name + "(" + uv + ")" }}.
		SceneSampleLevel("sceneColor", "uv", "0.0")
	want := "sceneColor(uv)"
	if got != want {
		t.Fatalf("SceneSampleLevel with nil SceneSampleLevelFn = %q, want %q (degrade to SceneSample)", got, want)
	}
}

// TestRecoverConvertsEmitErrorToError is the direct test for the mechanism
// wgsl.Emit and metal.Emit use to convert a panicked *ir.EmitError into a
// normal error return.
func TestRecoverConvertsEmitErrorToError(t *testing.T) {
	_, err := Recover(func() (string, error) {
		Resolver{}.CellUV() // panics with a *ir.EmitError
		return "unreachable", nil
	})
	var ee *ir.EmitError
	if !errors.As(err, &ee) {
		t.Fatalf("Recover error = %v (%T), want *ir.EmitError", err, err)
	}
	if ee.Code != ir.CodeEmitUnwiredNode {
		t.Fatalf("EmitError.Code = %s, want %s", ee.Code, ir.CodeEmitUnwiredNode)
	}
}

// TestRecoverPassesThroughSuccess checks Recover does not interfere with a
// normal, non-panicking call.
func TestRecoverPassesThroughSuccess(t *testing.T) {
	got, err := Recover(func() (string, error) { return "ok", nil })
	if err != nil || got != "ok" {
		t.Fatalf("Recover(ok) = (%q, %v), want (\"ok\", nil)", got, err)
	}
}

// TestRecoverReraisesOtherPanics checks Recover does not swallow a panic that
// is not an *ir.EmitError — a real bug, not a diagnosed emission failure,
// must still crash loudly rather than being silently reported as a
// well-formed compile error.
func TestRecoverReraisesOtherPanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("Recover swallowed a non-EmitError panic, want it re-raised")
		}
		if s, ok := r.(string); !ok || s != "boom" {
			t.Fatalf("re-raised panic value = %v, want \"boom\"", r)
		}
	}()
	_, _ = Recover(func() (string, error) { panic("boom") })
}

// TestRecoverSplitConvertsEmitErrorToError is RecoverSplit's counterpart to
// TestRecoverConvertsEmitErrorToError, for the vertex+fragment backends
// (glsl.Emit, gles.Emit).
func TestRecoverSplitConvertsEmitErrorToError(t *testing.T) {
	_, _, err := RecoverSplit(func() (string, string, error) {
		Resolver{}.CellUV()
		return "unreachable", "unreachable", nil
	})
	var ee *ir.EmitError
	if !errors.As(err, &ee) {
		t.Fatalf("RecoverSplit error = %v (%T), want *ir.EmitError", err, err)
	}
}
