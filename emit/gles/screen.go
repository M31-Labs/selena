package gles

import (
	"fmt"
	"strings"

	"m31labs.dev/selena/emit/internal"
	"m31labs.dev/selena/ir"
)

// emitScreenVertex emits the GLSL ES 3.00 vertex shader of a screen pass: a
// fullscreen triangle from gl_VertexID, drawn with glDrawArrays(GL_TRIANGLES,
// 0, 3) and no vertex buffers. v_uv is 0..1 across the viewport the host sets,
// origin at the top left, the same as the WGSL and Metal passes.
func emitScreenVertex(m ir.Module) string {
	var b strings.Builder
	b.WriteString("#version 300 es\n\n")
	b.WriteString("out vec2 v_uv;\n\n")
	b.WriteString("void main() {\n")
	b.WriteString("  const vec2[3] positions = vec2[3](\n")
	b.WriteString("    vec2(-1.0, -1.0), vec2(3.0, -1.0), vec2(-1.0, 3.0)\n")
	b.WriteString("  );\n")
	b.WriteString("  const vec2[3] uvs = vec2[3](\n")
	b.WriteString("    vec2(0.0, 1.0), vec2(2.0, 1.0), vec2(0.0, -1.0)\n")
	b.WriteString("  );\n")
	b.WriteString("  v_uv = uvs[gl_VertexID];\n")
	b.WriteString("  gl_Position = vec4(positions[gl_VertexID], -1.0, 1.0);\n")
	b.WriteString("}\n")
	return b.String()
}

// emitScreenFragment emits the GLSL ES 3.00 fragment shader of a screen pass.
func emitScreenFragment(m ir.Module) string {
	var b strings.Builder
	b.WriteString("#version 300 es\nprecision highp float;\n")
	for _, u := range m.Uniforms {
		fmt.Fprintf(&b, "uniform %s %s;\n", typeName(u.Type), u.Name)
	}
	for _, au := range m.ArrayUniforms {
		fmt.Fprintf(&b, "uniform %s %s[%d];\n", typeName(au.ElemType), au.Name, au.Count)
	}
	b.WriteString("in vec2 v_uv;\n")
	for _, t := range m.Textures {
		fmt.Fprintf(&b, "uniform %s %s;\n", samplerType(t, false), t.Name)
	}
	res := internal.NewBare(prismDialect)
	res.TypedUintLiterals = ir.NeedsTypedUintLiterals(m)
	res.ReturnFn = glesReturnFn
	b.WriteString("out vec4 fragColor;\n\nvoid main() {\n")
	internal.EmitStmtList(&b, m.Fragment.Body, res, "  ", false)
	fmt.Fprintf(&b, "  fragColor = %s;\n}\n", ir.Print(m.Fragment.Output, res))
	return b.String()
}
