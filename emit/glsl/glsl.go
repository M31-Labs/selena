package glsl

import (
	"fmt"
	"strings"

	"m31labs.dev/prism/dialect"
	"m31labs.dev/selena/emit/internal"
	"m31labs.dev/selena/ir"
)

var prismDialect = dialect.GLSL{}

// Emit renders m as GLSL ES 1.00 vertex and fragment sources for the WebGL
// backend (browser + GoSX desktop via Chromium). Unlike WGSL, GLSL ships the
// two stages as separate sources and addresses everything through bare globals
// (attribute/uniform/varying) — the divergence this slice exists to surface.
func Emit(m ir.Module) (vertex, fragment string, err error) {
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
}

// emitVertexAuthored emits the WebGL1 (GLSL ES 1.00) vertex shader for a material
// that authors its own vertex() stage (B4).
//
// WebGL1 has no gl_VertexID: when the body uses the vertexIndex builtin the host
// must supply a `float a_vertexIndex` attribute buffer (one float per vertex);
// this emitter declares it and aliases `vertexIndex` to it. This is the one
// documented procedural-geometry limitation versus GLES3/WGSL/Metal, which have
// real vertex-index builtins.
func emitVertexAuthored(m ir.Module) string {
	var b strings.Builder
	for _, a := range m.Attributes {
		fmt.Fprintf(&b, "attribute %s %s;\n", typeName(a.Type), a.Name)
	}
	if m.UsesVertexIndex {
		// WebGL1 fallback: host-supplied float vertex-index attribute.
		b.WriteString("attribute float a_vertexIndex;\n")
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
		fmt.Fprintf(&b, "varying %s %s;\n", typeName(v.Type), v.Name)
	}
	res := internal.NewBare(prismDialect)
	res.Varyings = internal.NameSet(m.Varyings)
	res.StateSampleUVFn = func(uv string) string { return fmt.Sprintf("texture2DLod(stateTex, %s, 0.0)", uv) }
	b.WriteString("\nvoid main() {\n")
	if m.UsesVertexIndex {
		b.WriteString("  float vertexIndex = a_vertexIndex;\n")
	}
	internal.EmitStmtList(&b, m.Vertex.Body, res, "  ", false)
	fmt.Fprintf(&b, "  gl_Position = %s;\n}\n", ir.Print(m.Vertex.Output, res))
	return b.String()
}

// emitFragmentAuthored emits the WebGL1 fragment shader for an authored-vertex
// material. It reads author varyings as bare globals and supports stateAt(uv)
// (sampling stateTex) when the material declares a statefield.
func emitFragmentAuthored(m ir.Module) string {
	var b strings.Builder
	writeFragmentPrologue(&b, m, "mediump")
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
		fmt.Fprintf(&b, "varying %s %s;\n", typeName(v.Type), v.Name)
	}
	for _, t := range m.Textures {
		if t.Cube {
			fmt.Fprintf(&b, "uniform samplerCube %s;\n", t.Name)
		} else {
			fmt.Fprintf(&b, "uniform sampler2D %s;\n", t.Name)
		}
	}
	res := internal.NewBare(prismDialect)
	res.StateSampleUVFn = func(uv string) string { return fmt.Sprintf("texture2D(stateTex, %s)", uv) }
	b.WriteString("\nvoid main() {\n")
	internal.EmitStmtList(&b, m.Fragment.Body, res, "  ", false)
	fmt.Fprintf(&b, "  gl_FragColor = %s;\n}\n", ir.Print(m.Fragment.Output, res))
	return b.String()
}

// emitFeedbackVertex emits the GLSL ES 1.00 fullscreen-quad vertex for a
// feedback ping-pong pass. It passes vUV (the cell's [0,1] grid coordinate) to
// the fragment, where state(dx, dy) samples stateTex around it.
func emitFeedbackVertex(m ir.Module) string {
	var b strings.Builder
	b.WriteString("attribute vec2 a_position;\n\n")
	b.WriteString("varying vec2 vUV;\n\n")
	b.WriteString("void main() {\n")
	b.WriteString("  vUV = a_position * 0.5 + 0.5;\n")
	b.WriteString("  gl_Position = vec4(a_position, 0.0, 1.0);\n")
	b.WriteString("}\n")
	return b.String()
}

// emitFeedbackFragment emits the GLSL ES 1.00 feedback fragment. The previous
// state is read from a highp sampler2D (stateTex); state(dx, dy) lowers to a
// texture2D sample stepped by texelSize. The next state is written to the bound
// float FBO via gl_FragColor (the host ping-pongs which texture/FBO is in/out).
func emitFeedbackFragment(m ir.Module) string {
	var b strings.Builder
	writeFragmentPrologue(&b, m, "highp")
	b.WriteString("varying vec2 vUV;\n\n")
	b.WriteString("uniform highp sampler2D stateTex;\n")
	b.WriteString("uniform vec2 texelSize;\n")
	for _, u := range m.Uniforms {
		fmt.Fprintf(&b, "uniform %s %s;\n", typeName(u.Type), u.Name)
	}
	for _, au := range m.ArrayUniforms {
		fmt.Fprintf(&b, "uniform %s %s[%d];\n", typeName(au.ElemType), au.Name, au.Count)
	}
	b.WriteString("\n")
	res := internal.Resolver{
		Dialect: prismDialect,
		StateSampleFn: func(dx, dy int64) string {
			if dx == 0 && dy == 0 {
				return "texture2D(stateTex, vUV)"
			}
			return fmt.Sprintf("texture2D(stateTex, vUV + vec2(%s, %s) * texelSize)", glFloatLit(dx), glFloatLit(dy))
		},
		CellUVFn: func() string { return "vUV" },
	}
	b.WriteString("void main() {\n")
	internal.EmitStmtList(&b, m.Fragment.Body, res, "  ", false)
	fmt.Fprintf(&b, "  gl_FragColor = %s;\n}\n", ir.Print(m.Fragment.Output, res))
	return b.String()
}

// glFloatLit renders an integer cell offset as a GLSL float literal (1 -> 1.0).
func glFloatLit(v int64) string { return fmt.Sprintf("%d.0", v) }

func emitVertex(m ir.Module) string {
	var b strings.Builder
	for _, a := range m.Attributes {
		fmt.Fprintf(&b, "attribute %s %s;\n", typeName(a.Type), a.Name)
	}
	for _, u := range m.Uniforms {
		fmt.Fprintf(&b, "uniform %s %s;\n", typeName(u.Type), u.Name)
	}
	for _, au := range m.ArrayUniforms {
		fmt.Fprintf(&b, "uniform %s %s[%d];\n", typeName(au.ElemType), au.Name, au.Count)
	}
	for _, v := range m.Varyings {
		fmt.Fprintf(&b, "varying %s %s;\n", typeName(v.Type), v.Name)
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
	writeFragmentPrologue(&b, m, "mediump")
	for _, u := range m.Uniforms {
		fmt.Fprintf(&b, "uniform %s %s;\n", typeName(u.Type), u.Name)
	}
	for _, au := range m.ArrayUniforms {
		fmt.Fprintf(&b, "uniform %s %s[%d];\n", typeName(au.ElemType), au.Name, au.Count)
	}
	for _, v := range m.Varyings {
		fmt.Fprintf(&b, "varying %s %s;\n", typeName(v.Type), v.Name)
	}
	for _, t := range m.Textures {
		if t.Cube {
			fmt.Fprintf(&b, "uniform samplerCube %s;\n", t.Name)
		} else {
			fmt.Fprintf(&b, "uniform sampler2D %s;\n", t.Name)
		}
	}
	res := internal.NewBare(prismDialect)
	b.WriteString("\nvoid main() {\n")
	internal.EmitStmtList(&b, m.Fragment.Body, res, "  ", false)
	fmt.Fprintf(&b, "  gl_FragColor = %s;\n}\n", ir.Print(m.Fragment.Output, res))
	return b.String()
}

// writeFragmentPrologue writes the preprocessor directives that must precede
// every non-preprocessor statement of a GLSL ES 1.00 fragment shader, then the
// default precision qualifier.
//
// GLSL ES 1.00 has no dFdx/dFdy/fwidth in core: they arrive with
// GL_OES_standard_derivatives. Every fragment emitter in this package routes
// through here so that no surface kind can accept a derivative builtin on WGSL
// and then emit GLSL the WebGL driver rejects — the points and feedback
// fragments used to skip the directive entirely.
//
// WebGL adds a second requirement the shader source cannot express: the host
// must call gl.getExtension("OES_standard_derivatives") before compiling, or
// the driver reports "extension is not supported" and every fwidth call fails
// to resolve. bindings.Layout.Requires.GLExtensions carries that name so the
// host can enable it (see docs/compatibility.md).
func writeFragmentPrologue(b *strings.Builder, m ir.Module, precision string) {
	if ir.UsesDerivatives(m) {
		b.WriteString("#extension GL_OES_standard_derivatives : enable\n")
	}
	if ir.UsesSceneSampleLevel(m) {
		// GLSL ES 1.00 defines texture2DLod in the vertex stage only;
		// GL_EXT_shader_texture_lod adds the fragment-stage texture2DLodEXT form.
		b.WriteString("#extension GL_EXT_shader_texture_lod : enable\n")
	}
	b.WriteString("precision " + precision + " float;\n")
}

// emitPointsVertex emits the WebGL1 points vertex shader.
// Attributes: a_position (vec3) loc 0, a_size (float) loc 1, a_color (vec4) loc 2.
// Engine uniforms are declared as standard WebGL uniform vars.
//
// Varyings use the split vec3+float model to match the WGSL contract:
//
//	v_color  vec3  — rgb base colour (NOT vec4; alpha is separate)
//	v_alpha  float — per-point alpha pre-multiplied by u_opacity
//	v_fogFactor float
//	v_pointSize float
//
// pt.pointUV resolves to gl_PointCoord in the fragment (no dead v_pointCoord varying).
func emitPointsVertex(m ir.Module) string {
	var b strings.Builder
	b.WriteString("attribute vec3 a_position;\n")
	b.WriteString("attribute float a_size;\n")
	b.WriteString("attribute vec4 a_color;\n\n")
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
	// User uniforms.
	for _, u := range m.Uniforms {
		fmt.Fprintf(&b, "uniform %s %s;\n", typeName(u.Type), u.Name)
	}
	b.WriteString("\n")
	// Split v_color into vec3 rgb + separate float alpha so the fragment can
	// pass v_color to mix() as vec3 (matching the WGSL/Metal varyings contract).
	b.WriteString("varying vec3 v_color;\n")
	b.WriteString("varying float v_alpha;\n")
	b.WriteString("varying float v_fogFactor;\n")
	b.WriteString("varying float v_pointSize;\n")
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

// emitPointsFragment emits the WebGL1 points fragment shader, running the
// author surface body with the engine point varyings.
//
// Varyings declared here must match the vertex shader exactly:
//
//	v_color  vec3  — rgb (NOT vec4)
//	v_alpha  float — alpha (NOT derived from v_color.a here; already in varying)
//	v_fogFactor, v_pointSize float
//
// pt.pointUV resolves to gl_PointCoord directly (no dead varying).
func emitPointsFragment(m ir.Module) string {
	var b strings.Builder
	writeFragmentPrologue(&b, m, "mediump")
	b.WriteString("varying vec3 v_color;\n")
	b.WriteString("varying float v_alpha;\n")
	b.WriteString("varying float v_fogFactor;\n")
	b.WriteString("varying float v_pointSize;\n\n")
	// Engine uniforms (u_opacity is consumed in the vertex to produce v_alpha).
	b.WriteString("uniform int u_hasFog;\n")
	b.WriteString("uniform vec3 u_fogColor;\n")
	// User uniforms.
	for _, u := range m.Uniforms {
		fmt.Fprintf(&b, "uniform %s %s;\n", typeName(u.Type), u.Name)
	}
	b.WriteString("\n")

	// Build bare resolver. The "varyings" are bare globals here.
	// pt.pointUV lowers to the varying name "v_pointCoord", but in GLSL the
	// host draws GL_POINTS so the per-fragment UV is gl_PointCoord; we alias it
	// via a local so the author's emitted code references the right value.
	res := internal.NewBare(prismDialect)

	b.WriteString("void main() {\n")
	// pt.pointUV → v_pointCoord in the IR; alias to gl_PointCoord (no dead varying).
	b.WriteString("  vec2 v_pointCoord = gl_PointCoord;\n")
	// Author surface body.
	internal.EmitStmtList(&b, m.Fragment.Body, res, "  ", false)
	fmt.Fprintf(&b, "  gl_FragColor = %s;\n}\n", ir.Print(m.Fragment.Output, res))
	return b.String()
}

// emitPostVertex emits a minimal GLSL 1.00 fullscreen quad vertex shader for post passes.
func emitPostVertex(m ir.Module) string {
	var b strings.Builder
	b.WriteString("attribute vec2 a_position;\n\n")
	b.WriteString("varying vec2 v_uv;\n\n")
	b.WriteString("void main() {\n")
	b.WriteString("  v_uv = a_position * 0.5 + 0.5;\n")
	b.WriteString("  gl_Position = vec4(a_position, 0.0, 1.0);\n")
	b.WriteString("}\n")
	return b.String()
}

// emitPostFragment emits the WebGL1 post-process fragment shader.
// Engine textures: uniform sampler2D _sceneColor; uniform sampler2D _sceneDepth.
// User uniforms follow.
func emitPostFragment(m ir.Module) string {
	var b strings.Builder
	writeFragmentPrologue(&b, m, "mediump")
	b.WriteString("varying vec2 v_uv;\n\n")
	b.WriteString("uniform sampler2D _sceneColor;\n")
	b.WriteString("uniform sampler2D _sceneDepth;\n")
	if ir.UsesSceneSize(m) {
		// GLSL ES 1.00 has no textureSize; the host supplies the backdrop
		// resolution through this uniform (see Layout.Requires.GLSceneSizeUniform).
		b.WriteString("uniform vec2 _sceneSize;\n")
	}
	// User uniforms.
	for _, u := range m.Uniforms {
		fmt.Fprintf(&b, "uniform %s %s;\n", typeName(u.Type), u.Name)
	}
	for _, au := range m.ArrayUniforms {
		fmt.Fprintf(&b, "uniform %s %s[%d];\n", typeName(au.ElemType), au.Name, au.Count)
	}
	b.WriteString("\n")

	// Scene sample function for GLSL: delegate to texture2D.
	res := internal.Resolver{
		Dialect:  prismDialect,
		Uniforms: internal.NameSet(m.Uniforms),
		SceneSampleFn: func(name, uv string) string {
			switch name {
			case "sceneColor":
				return fmt.Sprintf("texture2D(_sceneColor, %s)", uv)
			case "sceneDepth":
				return fmt.Sprintf("texture2D(_sceneDepth, %s)", uv)
			default:
				return "vec4(0.0)"
			}
		},
		// GL_EXT_shader_texture_lod (requested by writeFragmentPrologue) adds the
		// fragment-stage explicit-LOD form; plain texture2DLod is vertex-only in
		// GLSL ES 1.00.
		SceneSampleLevelFn: func(name, uv, lod string) string {
			switch name {
			case "sceneColor":
				return fmt.Sprintf("texture2DLodEXT(_sceneColor, %s, %s)", uv, lod)
			case "sceneDepth":
				return fmt.Sprintf("texture2DLodEXT(_sceneDepth, %s, %s)", uv, lod)
			default:
				return "vec4(0.0)"
			}
		},
		SceneSizeFn: func() string { return "_sceneSize" },
	}

	b.WriteString("void main() {\n")
	internal.EmitStmtList(&b, m.Fragment.Body, res, "  ", false)
	fmt.Fprintf(&b, "  gl_FragColor = %s;\n}\n", ir.Print(m.Fragment.Output, res))
	return b.String()
}

// typeName spells an ir.Type in GLSL, delegating to prism/dialect.
func typeName(t ir.Type) string { return prismDialect.TypeName(internal.TypeToGPU(t)) }
