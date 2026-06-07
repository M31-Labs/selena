package wgsl

import (
	"fmt"
	"strings"

	"m31labs.dev/prism/dialect"
	"m31labs.dev/selena/emit/internal"
	"m31labs.dev/selena/ir"
)

var prismDialect = dialect.WGSL{}

// Emit renders m as a single WGSL source string exposing vertexMain and
// fragmentMain, for the WebGPU backend (browser + GoSX desktop via Chromium).
func Emit(m ir.Module) (string, error) {
	var b strings.Builder

	if len(m.Uniforms) > 0 {
		b.WriteString("struct Uniforms {\n")
		for _, u := range m.Uniforms {
			fmt.Fprintf(&b, "  %s : %s,\n", u.Name, typeName(u.Type))
		}
		b.WriteString("};\n@group(0) @binding(0) var<uniform> u : Uniforms;\n\n")
	}

	// Textures: WGSL binds the texture and its sampler separately. Convention:
	// uniform block is binding 0, then texture i takes 1+2i and its sampler 2+2i.
	for i, t := range m.Textures {
		fmt.Fprintf(&b, "@group(0) @binding(%d) var %s : texture_2d<f32>;\n", 1+2*i, t.Name)
		fmt.Fprintf(&b, "@group(0) @binding(%d) var %sSampler : sampler;\n", 2+2*i, t.Name)
	}
	if len(m.Textures) > 0 {
		b.WriteString("\n")
	}

	b.WriteString("struct VertexInput {\n")
	for i, a := range m.Attributes {
		fmt.Fprintf(&b, "  @location(%d) %s : %s,\n", i, a.Name, typeName(a.Type))
	}
	b.WriteString("};\n\n")

	b.WriteString("struct VertexOutput {\n  @builtin(position) position : vec4<f32>,\n")
	for i, v := range m.Varyings {
		fmt.Fprintf(&b, "  @location(%d) %s : %s,\n", i, v.Name, typeName(v.Type))
	}
	b.WriteString("};\n\n")

	// Vertex stage.
	vs := internal.NewQualified(prismDialect, m, false)
	b.WriteString("@vertex\nfn vertexMain(in : VertexInput) -> VertexOutput {\n  var out : VertexOutput;\n")
	for _, s := range m.Vertex.Body {
		if vs.Varyings[s.Target] {
			fmt.Fprintf(&b, "  out.%s = %s;\n", s.Target, ir.Print(s.Value, vs))
		} else {
			fmt.Fprintf(&b, "  let %s = %s;\n", s.Target, ir.Print(s.Value, vs))
		}
	}
	fmt.Fprintf(&b, "  out.position = %s;\n  return out;\n}\n\n", ir.Print(m.Vertex.Output, vs))

	// Fragment stage.
	fs := internal.NewQualified(prismDialect, m, true)
	b.WriteString("@fragment\nfn fragmentMain(in : VertexOutput) -> @location(0) vec4<f32> {\n")
	for _, s := range m.Fragment.Body {
		fmt.Fprintf(&b, "  let %s = %s;\n", s.Target, ir.Print(s.Value, fs))
	}
	fmt.Fprintf(&b, "  return %s;\n}\n", ir.Print(m.Fragment.Output, fs))

	return b.String(), nil
}

// typeName spells an ir.Type in WGSL, delegating to prism/dialect.
func typeName(t ir.Type) string { return prismDialect.TypeName(internal.TypeToGPU(t)) }
