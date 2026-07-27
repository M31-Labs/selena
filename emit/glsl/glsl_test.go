package glsl

import (
	"strings"
	"testing"

	"m31labs.dev/selena/ir"
)

func TestEmitDirectionalDiffuse(t *testing.T) {
	vert, frag, err := Emit(ir.DirectionalDiffuse())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"attribute vec3 position;",
		"uniform mat4 mvp;",
		"varying vec3 vNormal;",
		"vNormal = normalize((normalMatrix * normal));",
		"gl_Position = (mvp * vec4(position, 1.0));",
	} {
		if !strings.Contains(vert, want) {
			t.Errorf("GLSL vertex missing %q\n--- got ---\n%s", want, vert)
		}
	}
	for _, want := range []string{
		"precision mediump float;",
		"float diff = max(dot(n, normalize(lightDir)), 0.0);",
		"gl_FragColor = vec4(rgb, 1.0);",
	} {
		if !strings.Contains(frag, want) {
			t.Errorf("GLSL fragment missing %q\n--- got ---\n%s", want, frag)
		}
	}
}

// TestEmitEarlyReturnValueUsingDerivativeEnablesExtension is the regression
// test for ir/uses.go stmtMatches' ReturnCF case: a dpdx/dpdy/fwidth call
// packed directly into an early return's value (not a preceding Let) must
// still be detected, so the #extension GL_OES_standard_derivatives directive
// is emitted. Without that case the switch falls through to its default
// (false) for any ReturnCF, silently omitting the extension declaration —
// GLSL ES 1.00 requires it before dFdx/dFdy/fwidth are legal at all, so the
// emitted shader would fail to compile.
func TestEmitEarlyReturnValueUsingDerivativeEnablesExtension(t *testing.T) {
	mod := ir.Module{
		// A real compiled Module always has a non-nil Vertex.Output (the
		// default mvp*position transform, or an authored vertex() result);
		// this hand-built module needs one too so emitVertex's unconditional
		// ir.Print(m.Vertex.Output, ...) has something to render. A zero-value
		// Stage{} used to work here only because ir.Print's old default case
		// silently printed a placeholder comment for the resulting nil Expr.
		Vertex: ir.Stage{
			Output: ir.Construct{Type: ir.Vec4, Args: []ir.Expr{
				ir.Lit{Value: 0.0}, ir.Lit{Value: 0.0}, ir.Lit{Value: 0.0}, ir.Lit{Value: 1.0},
			}},
		},
		Fragment: ir.Stage{
			Body: []ir.Stmt{
				{CF: ir.IfCF{
					Cond: ir.Binary{Op: "<", L: ir.Lit{Value: 0.0}, R: ir.Lit{Value: 1.0}},
					Then: []ir.Stmt{
						{CF: ir.ReturnCF{Value: ir.Construct{Type: ir.Vec4, Args: []ir.Expr{
							ir.Call{Func: "dpdx", Args: []ir.Expr{ir.Lit{Value: 0.5}}},
							ir.Lit{Value: 0.0}, ir.Lit{Value: 0.0}, ir.Lit{Value: 1.0},
						}}}},
					},
				}},
			},
			Output: ir.Construct{Type: ir.Vec4, Args: []ir.Expr{
				ir.Lit{Value: 0.0}, ir.Lit{Value: 0.0}, ir.Lit{Value: 0.0}, ir.Lit{Value: 1.0},
			}},
		},
	}
	_, frag, err := Emit(mod)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(frag, "#extension GL_OES_standard_derivatives : enable") {
		t.Fatalf("GLSL fragment does not enable derivatives for a dpdx packed into an early return:\n%s", frag)
	}
}
