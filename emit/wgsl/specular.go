package wgsl

import (
	"fmt"
	"strings"

	"m31labs.dev/selena/emit/internal"
	"m31labs.dev/selena/ir"
)

func writeMeshFragment(b *strings.Builder, m ir.Module, fs internal.Resolver) {
	if !m.SpecularMRT {
		b.WriteString("@fragment\nfn fragmentMain(in : VertexOutput) -> @location(0) vec4<f32> {\n")
		internal.EmitStmtList(b, m.Fragment.Body, fs, "  ", true)
		fmt.Fprintf(b, "  return %s;\n}\n", ir.Print(m.Fragment.Output, fs))
		return
	}
	if body, color, specular, ok := ir.FusedSpecular(m); ok {
		b.WriteString("struct SelenaFragmentOutput {\n  @location(0) color : vec4<f32>,\n  @location(1) specular : vec4<f32>,\n};\n\n")
		b.WriteString("@fragment\nfn fragmentMainSpecular(in : VertexOutput) -> SelenaFragmentOutput {\n")
		internal.EmitStmtList(b, body, fs, "  ", true)
		fmt.Fprintf(b, "  let selenaMRTColor = %s;\n  let selenaMRTSpecular = (%s).rgb;\n  return SelenaFragmentOutput(selenaMRTColor, vec4<f32>(selenaMRTSpecular, selenaMRTColor.a));\n}\n", ir.Print(color, fs), ir.Print(specular, fs))
		return
	}
	internal.EmitSpecularHelpers(b, m, fs, "fn selenaSurface(in : VertexOutput) -> vec4<f32>", "fn selenaSpecular(in : VertexOutput) -> vec4<f32>", true)
	b.WriteString("struct SelenaFragmentOutput {\n  @location(0) color : vec4<f32>,\n  @location(1) specular : vec4<f32>,\n};\n\n")
	b.WriteString("@fragment\nfn fragmentMainSpecular(in : VertexOutput) -> SelenaFragmentOutput {\n  let color = selenaSurface(in);\n")
	if m.Specular == nil {
		b.WriteString("  let specular = vec3<f32>(0.0);\n")
	} else {
		b.WriteString("  let specular = selenaSpecular(in).rgb;\n")
	}
	b.WriteString("  return SelenaFragmentOutput(color, vec4<f32>(specular, color.a));\n}\n")
}
