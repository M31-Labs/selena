package wgsl

import (
	"strings"
	"testing"

	"m31labs.dev/selena/ir"
)

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
