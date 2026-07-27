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
// The internal.RecoverSplit wrapper converts a structured emission failure
// (ir.EmitError, raised by ir.Print or emit/internal's shared statement
// emitter — see their doc comments) into this function's normal error return.
func Emit(m ir.Module) (vertex, fragment string, err error) {
	return internal.RecoverSplit(func() (string, string, error) {
		switch m.Kind {
		case ir.KindPoints:
			return emitPointsVertex(m), emitPointsFragment(m), nil
		case ir.KindPost:
			return emitPostVertex(m), emitPostFragment(m), nil
		case ir.KindFeedback:
			return emitFeedbackVertex(m), emitFeedbackFragment(m), nil
		default:
			if m.VertexAuthored {
				return emitVertexAuthored(m), emitFragmentAuthored(m), nil
			}
			return emitVertex(m), emitFragment(m), nil
		}
	})
}

// emitVertexAuthored emits the GLES3 (GLSL ES 3.00) vertex shader for a material
// that authors its own vertex() stage (B4). Unlike WebGL1, GLES3 has gl_VertexID,
// so the vertexIndex builtin aliases to it directly (no host attribute needed).
func emitVertexAuthored(m ir.Module) string {
	var b strings.Builder
	b.WriteString("#version 300 es\n")
	for _, a := range m.Attributes {
		fmt.Fprintf(&b, "in %s %s;\n", typeName(a.Type), a.Name)
	}
	for _, u := range m.Uniforms {
		fmt.Fprintf(&b, "uniform %s %s;\n", typeName(u.Type), u.Name)
	}
	for _, au := range m.ArrayUniforms {
		fmt.Fprintf(&b, "uniform %s %s[%d];\n", typeName(au.ElemType), au.Name, au.Count)
	}
	if m.StateField != "" {
		b.WriteString("uniform highp sampler2D stateTex;\n")
	}
	for _, v := range m.Varyings {
		fmt.Fprintf(&b, "out %s %s;\n", typeName(v.Type), v.Name)
	}
	res := internal.NewBare(prismDialect)
	res.Varyings = internal.NameSet(m.Varyings)
	res.StateSampleUVFn = func(uv string) string { return fmt.Sprintf("textureLod(stateTex, %s, 0.0)", uv) }
	b.WriteString("\nvoid main() {\n")
	if m.UsesVertexIndex {
		b.WriteString("  uint vertexIndex = uint(gl_VertexID);\n")
	}
	internal.EmitStmtList(&b, m.Vertex.Body, res, "  ", false)
	fmt.Fprintf(&b, "  gl_Position = %s;\n}\n", ir.Print(m.Vertex.Output, res))
	return b.String()
}

// emitFragmentAuthored emits the GLES3 fragment shader for an authored-vertex
// material. It reads author varyings via `in` and supports stateAt(uv) (sampling
// stateTex) when the material declares a statefield.
func emitFragmentAuthored(m ir.Module) string {
	var b strings.Builder
	b.WriteString("#version 300 es\nprecision highp float;\n")
	for _, u := range m.Uniforms {
		fmt.Fprintf(&b, "uniform %s %s;\n", typeName(u.Type), u.Name)
	}
	for _, au := range m.ArrayUniforms {
		fmt.Fprintf(&b, "uniform %s %s[%d];\n", typeName(au.ElemType), au.Name, au.Count)
	}
	if m.StateField != "" {
		b.WriteString("uniform highp sampler2D stateTex;\n")
	}
	for _, v := range m.Varyings {
		fmt.Fprintf(&b, "in %s %s;\n", typeName(v.Type), v.Name)
	}
	for _, t := range m.Textures {
		if t.Cube {
			fmt.Fprintf(&b, "uniform samplerCube %s;\n", t.Name)
		} else {
			fmt.Fprintf(&b, "uniform sampler2D %s;\n", t.Name)
		}
	}
	res := internal.NewBare(prismDialect)
	res.StateSampleUVFn = func(uv string) string { return fmt.Sprintf("texture(stateTex, %s)", uv) }
	res.ReturnFn = glesReturnFn
	b.WriteString("out vec4 fragColor;\n\nvoid main() {\n")
	internal.EmitStmtList(&b, m.Fragment.Body, res, "  ", false)
	fmt.Fprintf(&b, "  fragColor = %s;\n}\n", ir.Print(m.Fragment.Output, res))
	return b.String()
}

// emitFeedbackVertex emits a GLES3 fullscreen-triangle vertex for a feedback
// ping-pong pass, passing vUV (the cell's [0,1] grid coordinate) to the fragment.
func emitFeedbackVertex(m ir.Module) string {
	var b strings.Builder
	b.WriteString("#version 300 es\n\n")
	b.WriteString("out vec2 vUV;\n\n")
	b.WriteString("void main() {\n")
	b.WriteString("  const vec2[3] positions = vec2[3](\n")
	b.WriteString("    vec2(-1.0, -1.0), vec2(3.0, -1.0), vec2(-1.0, 3.0)\n")
	b.WriteString("  );\n")
	b.WriteString("  vUV = positions[gl_VertexID] * 0.5 + 0.5;\n")
	b.WriteString("  gl_Position = vec4(positions[gl_VertexID], 0.0, 1.0);\n")
	b.WriteString("}\n")
	return b.String()
}

// emitFeedbackFragment emits the GLES3 feedback fragment. state(dx, dy) reads
// the previous-state highp sampler2D (stateTex) stepped by texelSize; the next
// state is written to the bound float FBO via fragColor (the host ping-pongs
// which texture/FBO is in/out each step).
func emitFeedbackFragment(m ir.Module) string {
	var b strings.Builder
	b.WriteString("#version 300 es\nprecision highp float;\n")
	b.WriteString("in vec2 vUV;\n\n")
	b.WriteString("uniform highp sampler2D stateTex;\n")
	b.WriteString("uniform vec2 texelSize;\n")
	for _, u := range m.Uniforms {
		fmt.Fprintf(&b, "uniform %s %s;\n", typeName(u.Type), u.Name)
	}
	for _, au := range m.ArrayUniforms {
		fmt.Fprintf(&b, "uniform %s %s[%d];\n", typeName(au.ElemType), au.Name, au.Count)
	}
	b.WriteString("out vec4 fragColor;\n\n")
	res := internal.Resolver{
		Dialect: prismDialect,
		StateSampleFn: func(dx, dy int64) string {
			if dx == 0 && dy == 0 {
				return "texture(stateTex, vUV)"
			}
			return fmt.Sprintf("texture(stateTex, vUV + vec2(%s, %s) * texelSize)", glFloatLit(dx), glFloatLit(dy))
		},
		CellUVFn: func() string { return "vUV" },
		ReturnFn: glesReturnFn,
	}
	b.WriteString("void main() {\n")
	internal.EmitStmtList(&b, m.Fragment.Body, res, "  ", false)
	fmt.Fprintf(&b, "  fragColor = %s;\n}\n", ir.Print(m.Fragment.Output, res))
	return b.String()
}

// glFloatLit renders an integer cell offset as a GLSL float literal (1 -> 1.0).
func glFloatLit(v int64) string { return fmt.Sprintf("%d.0", v) }

// glesReturnFn renders an early ir.ReturnCF for GLSL ES 3.00 (GLES). Every
// fragment main() here returns void and writes the fragment colour through
// the explicit `out vec4 fragColor` global, so an early return must write it
// too, before a bare `return;` — unlike WGSL/Metal mesh, points, and post,
// where fragmentMain declares a vec4/float4 return type and a plain
// `return val;` suffices (the emit/internal default ReturnFn covers those).
func glesReturnFn(val string) string {
	return "fragColor = " + val + "; return;"
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
	for _, au := range m.ArrayUniforms {
		fmt.Fprintf(&b, "uniform %s %s[%d];\n", typeName(au.ElemType), au.Name, au.Count)
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
	b.WriteString("#version 300 es\nprecision highp float;\n")
	for _, u := range m.Uniforms {
		fmt.Fprintf(&b, "uniform %s %s;\n", typeName(u.Type), u.Name)
	}
	for _, au := range m.ArrayUniforms {
		fmt.Fprintf(&b, "uniform %s %s[%d];\n", typeName(au.ElemType), au.Name, au.Count)
	}
	for _, v := range m.Varyings {
		fmt.Fprintf(&b, "in %s %s;\n", typeName(v.Type), v.Name)
	}
	for _, t := range m.Textures {
		if t.Cube {
			fmt.Fprintf(&b, "uniform samplerCube %s;\n", t.Name)
		} else {
			fmt.Fprintf(&b, "uniform sampler2D %s;\n", t.Name)
		}
	}
	res := internal.NewBare(prismDialect)
	res.ReturnFn = glesReturnFn
	b.WriteString("out vec4 fragColor;\n\nvoid main() {\n")
	internal.EmitStmtList(&b, m.Fragment.Body, res, "  ", false)
	fmt.Fprintf(&b, "  fragColor = %s;\n}\n", ir.Print(m.Fragment.Output, res))
	return b.String()
}

// emitPointsVertex emits the GLES3 points vertex shader.
//
// Varyings use the split vec3+float model to match the WGSL contract:
//
//	v_color  vec3  — rgb base colour (NOT vec4; alpha is separate)
//	v_alpha  float — per-point alpha pre-multiplied by u_opacity
//	v_fogFactor, v_pointSize float
//
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
//
//	v_color  vec3  — rgb (NOT vec4)
//	v_alpha  float — alpha (already pre-multiplied by u_opacity in vertex)
//	v_fogFactor, v_pointSize float
//
// pt.pointUV resolves to gl_PointCoord directly (no dead varying).
func emitPointsFragment(m ir.Module) string {
	var b strings.Builder
	b.WriteString("#version 300 es\nprecision highp float;\n")
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
	res.ReturnFn = glesReturnFn
	b.WriteString("void main() {\n")
	// pt.pointUV → v_pointCoord in the IR; alias to gl_PointCoord (no dead varying).
	b.WriteString("  vec2 v_pointCoord = gl_PointCoord;\n")
	internal.EmitStmtList(&b, m.Fragment.Body, res, "  ", false)
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
	b.WriteString("#version 300 es\nprecision highp float;\n")
	b.WriteString("in vec2 v_uv;\n\n")
	b.WriteString("uniform sampler2D _sceneColor;\n")
	b.WriteString("uniform sampler2D _sceneDepth;\n")
	for _, u := range m.Uniforms {
		fmt.Fprintf(&b, "uniform %s %s;\n", typeName(u.Type), u.Name)
	}
	for _, au := range m.ArrayUniforms {
		fmt.Fprintf(&b, "uniform %s %s[%d];\n", typeName(au.ElemType), au.Name, au.Count)
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
		// GLSL ES 3.00 has textureLod and textureSize in core — no extension and
		// no host-supplied size uniform, unlike the ES 1.00 emitter.
		SceneSampleLevelFn: func(name, uv, lod string) string {
			switch name {
			case "sceneColor":
				return fmt.Sprintf("textureLod(_sceneColor, %s, %s)", uv, lod)
			case "sceneDepth":
				return fmt.Sprintf("textureLod(_sceneDepth, %s, %s)", uv, lod)
			default:
				return "vec4(0.0)"
			}
		},
		SceneSizeFn: func() string { return "vec2(textureSize(_sceneColor, 0))" },
		ReturnFn:    glesReturnFn,
	}
	b.WriteString("void main() {\n")
	internal.EmitStmtList(&b, m.Fragment.Body, res, "  ", false)
	fmt.Fprintf(&b, "  fragColor = %s;\n}\n", ir.Print(m.Fragment.Output, res))
	return b.String()
}

// typeName spells an ir.Type in GLSL ES, delegating to prism/dialect.
func typeName(t ir.Type) string { return prismDialect.TypeName(internal.TypeToGPU(t)) }
