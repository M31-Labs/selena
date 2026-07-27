package wgsl

import (
	"errors"
	"strings"
	"testing"

	"m31labs.dev/selena/ir"
)

// TestEmitReturnsErrorInsteadOfPanickingForUnwiredIRNode is the end-to-end
// counterpart of emit/internal's Resolver unit tests: it proves Emit's whole
// call chain — ir.Print raising a panic deep inside emitMesh, all the way up
// through Emit's internal.Recover wrapper — surfaces as a normal Go error
// rather than crashing the process. lower/ never produces a mesh Module whose
// fragment output is a bare CellUV (CellUV is a feedback-kind concept; the
// mesh Resolver never wires CellUVFn), so this hand-built Module is exactly
// the "should never happen" shape ir.CodeEmitUnwiredNode exists for.
func TestEmitReturnsErrorInsteadOfPanickingForUnwiredIRNode(t *testing.T) {
	mod := ir.DirectionalDiffuse()
	mod.Fragment.Output = ir.CellUV{}

	_, err := Emit(mod)
	if err == nil {
		t.Fatal("Emit succeeded, want an error (CellUV has no renderer wired in a mesh fragment stage)")
	}
	var ee *ir.EmitError
	if !errors.As(err, &ee) {
		t.Fatalf("error type = %T, want *ir.EmitError (wrapped)", err)
	}
	if ee.Code != ir.CodeEmitUnwiredNode {
		t.Fatalf("EmitError.Code = %s, want %s", ee.Code, ir.CodeEmitUnwiredNode)
	}
}

func TestEmitDirectionalDiffuse(t *testing.T) {
	src, err := Emit(ir.DirectionalDiffuse())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"struct Uniforms {",
		"mvp : mat4x4<f32>",
		"@group(0) @binding(0) var<uniform> u : Uniforms;",
		"@location(0) position : vec3<f32>",
		"@builtin(position) position : vec4<f32>",
		"@vertex",
		"fn vertexMain(in : VertexInput) -> VertexOutput",
		"out.vNormal = normalize((u.normalMatrix * in.normal));",
		"out.position = (u.mvp * vec4<f32>(in.position, 1.0));",
		"@fragment",
		"let diff = max(dot(n, normalize(u.lightDir)), 0.0);",
		"return vec4<f32>(rgb, 1.0);",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("WGSL missing %q\n--- got ---\n%s", want, src)
		}
	}
}
