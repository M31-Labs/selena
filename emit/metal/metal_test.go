package metal

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
		"#include <metal_stdlib>",
		"float4x4 mvp;",
		"float3 position [[attribute(0)]];",
		"float4 position [[position]];",
		"vertex VertexOut vertexMain(VertexIn in [[stage_in]], constant Uniforms& u [[buffer(0)]])",
		"out.vNormal = normalize((u.normalMatrix * in.normal));",
		"out.position = (u.mvp * float4(in.position, 1.0));",
		"fragment float4 fragmentMain(",
		"float diff = max(dot(n, normalize(u.lightDir)), 0.0);",
		"return float4(rgb, 1.0);",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("MSL missing %q\n--- got ---\n%s", want, src)
		}
	}
}
