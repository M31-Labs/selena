package wgsl

import (
	"fmt"
	"strings"

	"m31labs.dev/selena/emit/internal"
	"m31labs.dev/selena/ir"
)

// emitScreen emits a WGSL screen pass.
//
// Binding contract, all in @group(0), the same arithmetic as a mesh material:
//
//	@binding(0)        Uniforms (only when the material has uniforms)
//	@binding(1+2i)     texture i, @binding(2+2i) its sampler
//
// The vertex stage is a fullscreen triangle drawn with draw(3) and no vertex
// buffers. v_uv is 0..1 across the viewport the host sets, origin at the top
// left. The host places the pass by setting the viewport and scissor to the
// target rectangle.
func emitScreen(m ir.Module) (string, error) {
	var b strings.Builder

	if len(m.Uniforms) > 0 || len(m.ArrayUniforms) > 0 {
		b.WriteString("struct Uniforms {\n")
		for _, u := range m.Uniforms {
			fmt.Fprintf(&b, "  %s : %s,\n", u.Name, typeName(u.Type))
		}
		for _, au := range m.ArrayUniforms {
			fmt.Fprintf(&b, "  %s : array<%s, %d>,\n", au.Name, typeName(au.ElemType), au.Count)
		}
		b.WriteString("};\n@group(0) @binding(0) var<uniform> u : Uniforms;\n\n")
	}

	for i, t := range m.Textures {
		fmt.Fprintf(&b, "@group(0) @binding(%d) var %s : texture_2d<f32>;\n", 1+2*i, t.Name)
		fmt.Fprintf(&b, "@group(0) @binding(%d) var %sSampler : sampler;\n", 2+2*i, t.Name)
	}
	if len(m.Textures) > 0 {
		b.WriteString("\n")
	}

	b.WriteString(`struct ScreenOutput {
  @builtin(position) position : vec4<f32>,
  @location(0) v_uv           : vec2<f32>,
};

const _screenPositions = array<vec2<f32>, 3>(
  vec2<f32>(-1.0, -1.0),
  vec2<f32>( 3.0, -1.0),
  vec2<f32>(-1.0,  3.0),
);
const _screenUVs = array<vec2<f32>, 3>(
  vec2<f32>(0.0, 1.0),
  vec2<f32>(2.0, 1.0),
  vec2<f32>(0.0, -1.0),
);

@vertex fn vertexMain(@builtin(vertex_index) vi : u32) -> ScreenOutput {
  var out : ScreenOutput;
  out.position = vec4<f32>(_screenPositions[vi], 0.0, 1.0);
  out.v_uv     = _screenUVs[vi];
  return out;
}

struct ScreenInput {
  @location(0) v_uv : vec2<f32>,
};

`)

	fs := internal.NewQualified(prismDialect, m, true)
	b.WriteString("@fragment fn fragmentMain(in : ScreenInput) -> @location(0) vec4<f32> {\n")
	internal.EmitStmtList(&b, m.Fragment.Body, fs, "  ", true)
	fmt.Fprintf(&b, "  return %s;\n}\n", ir.Print(m.Fragment.Output, fs))
	return b.String(), nil
}
