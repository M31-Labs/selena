package metal

import (
	"fmt"
	"strings"

	"m31labs.dev/selena/emit/internal"
	"m31labs.dev/selena/ir"
)

// emitScreen emits a Metal screen pass.
//
// Buffer and texture indices follow a mesh material: Uniforms at [[buffer(0)]]
// (only when the material has uniforms), texture i at [[texture(i)]] with its
// sampler at [[sampler(i)]]. The vertex stage is a fullscreen triangle drawn
// with drawPrimitives(.triangle, 0, 3) and no vertex buffers. v_uv is 0..1
// across the viewport the host sets, origin at the top left.
func emitScreen(m ir.Module) (string, error) {
	var b strings.Builder
	b.WriteString("#include <metal_stdlib>\nusing namespace metal;\n\n")

	hasUniforms := len(m.Uniforms) > 0 || len(m.ArrayUniforms) > 0
	if hasUniforms {
		b.WriteString("struct Uniforms {\n")
		for _, u := range m.Uniforms {
			fmt.Fprintf(&b, "  %s %s;\n", typeName(u.Type), u.Name)
		}
		for _, au := range m.ArrayUniforms {
			fmt.Fprintf(&b, "  %s %s[%d];\n", typeName(au.ElemType), au.Name, au.Count)
		}
		b.WriteString("};\n\n")
	}

	b.WriteString(`struct ScreenOut {
  float4 position [[position]];
  float2 v_uv;
};

vertex ScreenOut vertexMain(uint vid [[vertex_id]]) {
  const float2 pos[3] = { {-1,-1},{3,-1},{-1,3} };
  const float2 uvs[3] = { {0,1},{2,1},{0,-1} };
  ScreenOut out;
  out.position = float4(pos[vid], 0, 1);
  out.v_uv     = uvs[vid];
  return out;
}

`)

	fs := internal.NewQualified(prismDialect, m, true)
	fragSig := "fragment float4 fragmentMain(ScreenOut in [[stage_in]]"
	if hasUniforms {
		fragSig += ", constant Uniforms& u [[buffer(0)]]"
	}
	for i, t := range m.Textures {
		fragSig += fmt.Sprintf(", %s %s [[texture(%d)]], sampler %sSampler [[sampler(%d)]]", metalTextureType(t), t.Name, i, t.Name, i)
	}
	b.WriteString(fragSig + ") {\n")
	internal.EmitStmtList(&b, m.Fragment.Body, fs, "  ", false)
	fmt.Fprintf(&b, "  return %s;\n}\n", ir.Print(m.Fragment.Output, fs))
	return b.String(), nil
}
