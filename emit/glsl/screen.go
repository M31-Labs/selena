package glsl

import (
	"fmt"
	"strings"

	"m31labs.dev/selena/emit/internal"
	"m31labs.dev/selena/ir"
)

// emitScreenVertex emits the GLSL ES 1.00 vertex shader of a screen pass.
// WebGL 1 has no gl_VertexID, so the host draws a clip-space triangle or quad
// from a buffer bound to the vec2 attribute a_position, for example the
// triangle (-1,-1), (3,-1), (-1,3). v_uv is 0..1 across the viewport the host
// sets with the origin at the top left, the same as every other target; a
// quad over (-1..1) therefore maps y = 1 to v = 0.
func emitScreenVertex(m ir.Module) string {
	var b strings.Builder
	b.WriteString("attribute vec2 a_position;\n\n")
	b.WriteString("varying vec2 v_uv;\n\n")
	b.WriteString("void main() {\n")
	b.WriteString("  v_uv = vec2(a_position.x * 0.5 + 0.5, 0.5 - a_position.y * 0.5);\n")
	b.WriteString("  gl_Position = vec4(a_position, 0.0, 1.0);\n")
	b.WriteString("}\n")
	return b.String()
}

// emitScreenFragment emits the GLSL ES 1.00 fragment shader of a screen pass.
// The default float precision is highp, because screen passes read data such as
// decibel levels and texel indices that mediump (10 bits on some GPUs) cannot
// hold. Fragment highp is optional in GLSL ES 1.00; the host checks
// getShaderPrecisionFormat.
func emitScreenFragment(m ir.Module) string {
	var b strings.Builder
	writeFragmentPrologue(&b, m, "highp")
	for _, u := range m.Uniforms {
		fmt.Fprintf(&b, "uniform %s %s;\n", typeName(u.Type), u.Name)
	}
	for _, au := range m.ArrayUniforms {
		fmt.Fprintf(&b, "uniform %s %s[%d];\n", typeName(au.ElemType), au.Name, au.Count)
	}
	b.WriteString("varying vec2 v_uv;\n")
	for _, t := range m.Textures {
		fmt.Fprintf(&b, "uniform %s %s;\n", samplerType(t, false), t.Name)
	}
	writeTexSizeUniforms(&b, m, true)
	res := internal.NewBare(prismDialect)
	res.Fragment = true
	res.ReturnFn = glReturnFn
	b.WriteString("\nvoid main() {\n")
	internal.EmitStmtList(&b, m.Fragment.Body, res, "  ", false)
	fmt.Fprintf(&b, "  gl_FragColor = %s;\n}\n", ir.Print(m.Fragment.Output, res))
	return b.String()
}
