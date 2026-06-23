package gles

import (
	"fmt"
	"strings"

	"m31labs.dev/prism/dialect"
	"m31labs.dev/selena/emit/internal"
	"m31labs.dev/selena/ir"
)

var prismDialect = dialect.GLES{}

// Emit renders m as GLSL ES 3.00 vertex and fragment sources for the Android
// GLSurfaceView backend consumed by gosx-native. Like the WebGL GLSL emitter it
// uses bare globals, but the GLES3 IO model differs: in/out instead of
// attribute/varying, and an explicit fragment output instead of gl_FragColor.
func Emit(m ir.Module) (vertex, fragment string, err error) {
	switch m.Kind {
	case ir.KindPoints:
		return emitPointsVertex(m), emitPointsFragment(m), nil
	case ir.KindPost:
		return emitPostVertex(m), emitPostFragment(m), nil
	default:
		return emitVertex(m), emitFragment(m), nil
	}
}

func emitVertex(m ir.Module) string {
	var b strings.Builder
	b.WriteString("#version 300 es\n")
	for _, a := range m.Attributes {
		fmt.Fprintf(&b, "in %s %s;\n", typeName(a.Type), a.Name)
	}
	for _, u := range m.Uniforms {
		fmt.Fprintf(&b, "uniform %s %s;\n", typeName(u.Type), u.Name)
	}
	for _, v := range m.Varyings {
		fmt.Fprintf(&b, "out %s %s;\n", typeName(v.Type), v.Name)
	}
	vary := internal.NameSet(m.Varyings)
	res := internal.NewBare(prismDialect)
	b.WriteString("\nvoid main() {\n")
	for _, s := range m.Vertex.Body {
		if vary[s.Target] {
			fmt.Fprintf(&b, "  %s = %s;\n", s.Target, ir.Print(s.Value, res))
		} else {
			fmt.Fprintf(&b, "  %s %s = %s;\n", typeName(s.Type), s.Target, ir.Print(s.Value, res))
		}
	}
	fmt.Fprintf(&b, "  gl_Position = %s;\n}\n", ir.Print(m.Vertex.Output, res))
	return b.String()
}

func emitFragment(m ir.Module) string {
	var b strings.Builder
	b.WriteString("#version 300 es\nprecision mediump float;\n")
	for _, u := range m.Uniforms {
		fmt.Fprintf(&b, "uniform %s %s;\n", typeName(u.Type), u.Name)
	}
	for _, v := range m.Varyings {
		fmt.Fprintf(&b, "in %s %s;\n", typeName(v.Type), v.Name)
	}
	for _, t := range m.Textures {
		fmt.Fprintf(&b, "uniform sampler2D %s;\n", t.Name)
	}
	res := internal.NewBare(prismDialect)
	b.WriteString("out vec4 fragColor;\n\nvoid main() {\n")
	for _, s := range m.Fragment.Body {
		fmt.Fprintf(&b, "  %s %s = %s;\n", typeName(s.Type), s.Target, ir.Print(s.Value, res))
	}
	fmt.Fprintf(&b, "  fragColor = %s;\n}\n", ir.Print(m.Fragment.Output, res))
	return b.String()
}

// emitPointsVertex emits the GLES3 points vertex shader.
//
// Varyings use the split vec3+float model to match the WGSL contract:
//   v_color  vec3  — rgb base colour (NOT vec4; alpha is separate)
//   v_alpha  float — per-point alpha pre-multiplied by u_opacity
//   v_fogFactor, v_pointSize float
// pt.pointUV resolves to gl_PointCoord in the fragment (no dead v_pointCoord varying).
func emitPointsVertex(m ir.Module) string {
	var b strings.Builder
	b.WriteString("#version 300 es\nprecision highp float;\nprecision highp int;\n\n")
	b.WriteString("in vec3 a_position;\n")
	b.WriteString("in float a_size;\n")
	b.WriteString("in vec4 a_color;\n\n")
	b.WriteString("uniform mat4 u_viewMatrix;\n")
	b.WriteString("uniform mat4 u_projectionMatrix;\n")
	b.WriteString("uniform mat4 u_modelMatrix;\n")
	b.WriteString("uniform float u_defaultSize;\n")
	b.WriteString("uniform vec4 u_defaultColor;\n")
	b.WriteString("uniform bool u_hasPerVertexColor;\n")
	b.WriteString("uniform bool u_hasPerVertexSize;\n")
	b.WriteString("uniform bool u_sizeAttenuation;\n")
	b.WriteString("uniform float u_viewportHeight;\n")
	b.WriteString("uniform float u_minPixelSize;\n")
	b.WriteString("uniform float u_maxPixelSize;\n")
	b.WriteString("uniform int u_hasFog;\n")
	b.WriteString("uniform float u_fogDensity;\n")
	b.WriteString("uniform float u_opacity;\n")
	for _, u := range m.Uniforms {
		fmt.Fprintf(&b, "uniform %s %s;\n", typeName(u.Type), u.Name)
	}
	b.WriteString("\n")
	// Split v_color into vec3 rgb + separate float alpha so the fragment can
	// pass v_color to mix() as vec3 (matching the WGSL/Metal varyings contract).
	b.WriteString("out vec3 v_color;\n")
	b.WriteString("out float v_alpha;\n")
	b.WriteString("out float v_fogFactor;\n")
	b.WriteString("out float v_pointSize;\n")
	b.WriteString("\nvoid main() {\n")
	b.WriteString("  vec4 worldPos = u_modelMatrix * vec4(a_position, 1.0);\n")
	b.WriteString("  vec4 viewPos = u_viewMatrix * worldPos;\n")
	b.WriteString("  gl_Position = u_projectionMatrix * viewPos;\n")
	b.WriteString("  float size = u_hasPerVertexSize ? a_size : u_defaultSize;\n")
	b.WriteString("  float pixelSize;\n")
	b.WriteString("  if (u_sizeAttenuation) {\n")
	b.WriteString("    pixelSize = max(size * (u_viewportHeight * 0.5) / max(-viewPos.z, 0.001), 1.0);\n")
	b.WriteString("  } else {\n")
	b.WriteString("    pixelSize = max(size, 1.0);\n")
	b.WriteString("  }\n")
	b.WriteString("  if (u_minPixelSize > 0.0) pixelSize = max(pixelSize, u_minPixelSize);\n")
	b.WriteString("  if (u_maxPixelSize > 0.0) pixelSize = min(pixelSize, u_maxPixelSize);\n")
	b.WriteString("  gl_PointSize = pixelSize;\n")
	b.WriteString("  v_pointSize = pixelSize;\n")
	// Split vec4 a_color into rgb (v_color) and alpha*opacity (v_alpha).
	b.WriteString("  vec4 _col = u_hasPerVertexColor ? a_color : u_defaultColor;\n")
	b.WriteString("  v_color = _col.rgb;\n")
	b.WriteString("  v_alpha = _col.a * u_opacity;\n")
	b.WriteString("  if (u_hasFog != 0) {\n")
	b.WriteString("    float dist = length(viewPos.xyz);\n")
	b.WriteString("    v_fogFactor = clamp(exp(-u_fogDensity * u_fogDensity * dist * dist), 0.0, 1.0);\n")
	b.WriteString("  } else {\n")
	b.WriteString("    v_fogFactor = 1.0;\n")
	b.WriteString("  }\n")
	b.WriteString("}\n")
	return b.String()
}

// emitPointsFragment emits the GLES3 points fragment shader.
//
// Varyings declared here must match the vertex shader exactly:
//   v_color  vec3  — rgb (NOT vec4)
//   v_alpha  float — alpha (already pre-multiplied by u_opacity in vertex)
//   v_fogFactor, v_pointSize float
//
// pt.pointUV resolves to gl_PointCoord directly (no dead varying).
func emitPointsFragment(m ir.Module) string {
	var b strings.Builder
	b.WriteString("#version 300 es\nprecision mediump float;\n")
	b.WriteString("in vec3 v_color;\n")
	b.WriteString("in float v_alpha;\n")
	b.WriteString("in float v_fogFactor;\n")
	b.WriteString("in float v_pointSize;\n\n")
	// Engine uniforms (u_opacity is consumed in the vertex to produce v_alpha).
	b.WriteString("uniform int u_hasFog;\n")
	b.WriteString("uniform vec3 u_fogColor;\n")
	for _, u := range m.Uniforms {
		fmt.Fprintf(&b, "uniform %s %s;\n", typeName(u.Type), u.Name)
	}
	b.WriteString("out vec4 fragColor;\n\n")
	res := internal.NewBare(prismDialect)
	b.WriteString("void main() {\n")
	// pt.pointUV → v_pointCoord in the IR; alias to gl_PointCoord (no dead varying).
	b.WriteString("  vec2 v_pointCoord = gl_PointCoord;\n")
	for _, s := range m.Fragment.Body {
		fmt.Fprintf(&b, "  %s %s = %s;\n", typeName(s.Type), s.Target, ir.Print(s.Value, res))
	}
	fmt.Fprintf(&b, "  fragColor = %s;\n}\n", ir.Print(m.Fragment.Output, res))
	return b.String()
}

// emitPostVertex emits a GLES3 fullscreen triangle vertex shader.
func emitPostVertex(m ir.Module) string {
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
	b.WriteString("  gl_Position = vec4(positions[gl_VertexID], 0.0, 1.0);\n")
	b.WriteString("}\n")
	return b.String()
}

// emitPostFragment emits the GLES3 post-process fragment shader.
func emitPostFragment(m ir.Module) string {
	var b strings.Builder
	b.WriteString("#version 300 es\nprecision mediump float;\n")
	b.WriteString("in vec2 v_uv;\n\n")
	b.WriteString("uniform sampler2D _sceneColor;\n")
	b.WriteString("uniform sampler2D _sceneDepth;\n")
	for _, u := range m.Uniforms {
		fmt.Fprintf(&b, "uniform %s %s;\n", typeName(u.Type), u.Name)
	}
	b.WriteString("out vec4 fragColor;\n\n")
	res := internal.Resolver{
		Dialect:  prismDialect,
		Uniforms: internal.NameSet(m.Uniforms),
		SceneSampleFn: func(name, uv string) string {
			switch name {
			case "sceneColor":
				return fmt.Sprintf("texture(_sceneColor, %s)", uv)
			case "sceneDepth":
				return fmt.Sprintf("texture(_sceneDepth, %s)", uv)
			default:
				return "vec4(0.0)"
			}
		},
	}
	b.WriteString("void main() {\n")
	for _, s := range m.Fragment.Body {
		fmt.Fprintf(&b, "  %s %s = %s;\n", typeName(s.Type), s.Target, ir.Print(s.Value, res))
	}
	fmt.Fprintf(&b, "  fragColor = %s;\n}\n", ir.Print(m.Fragment.Output, res))
	return b.String()
}

// typeName spells an ir.Type in GLSL ES, delegating to prism/dialect.
func typeName(t ir.Type) string { return prismDialect.TypeName(internal.TypeToGPU(t)) }
