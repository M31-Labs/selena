package validate

import (
	"strings"
	"testing"

	prismvalidate "m31labs.dev/prism/validate"

	selena "m31labs.dev/selena"
	"m31labs.dev/selena/emit/gles"
	"m31labs.dev/selena/emit/glsl"
)

// essl100 prefixes src with the `#version 100` directive.
//
// The GLSL emitter targets GLSL ES 1.00 but emits no #version line, because
// WebGL treats a missing directive as `#version 100`. glslangValidator does
// NOT: with no directive it falls back to desktop GLSL 110, where fwidth,
// dFdx and dFdy are core and no extension is required. Validating the emitted
// string as-is therefore proved nothing about WebGL — a fragment that called
// fwidth with no GL_OES_standard_derivatives directive passed. Pinning the
// version is what makes glslang enforce the ES 1.00 rules the browser applies.
func essl100(src string) string {
	return "#version 100\n" + src
}

// compilePost compiles src and returns the compile result, failing the test on
// any error.
func compilePost(t *testing.T, src string) selena.Result {
	t.Helper()
	res, err := selena.Compile([]byte(src), selena.CompileOptions{})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	return res
}

func mustContain(t *testing.T, name, src, want string) {
	t.Helper()
	if !strings.Contains(src, want) {
		t.Fatalf("%s: missing %q in:\n%s", name, want, src)
	}
}

func mustNotContain(t *testing.T, name, src, notWant string) {
	t.Helper()
	if strings.Contains(src, notWant) {
		t.Fatalf("%s: unexpected %q in:\n%s", name, notWant, src)
	}
}

const derivativePostSource = `material AAEdge kind post {
    surface(post) -> color {
        let d = length(vec2f(post.uv.x - 0.5, post.uv.y - 0.5)) - 0.3
        let aa = max(fwidth(d), 0.0008)
        let m = 1.0 - smoothstep(0.0 - aa, 0.0 + aa, d)
        let c = sceneColor(post.uv)
        return rgb(c.r * m, c.g * m, c.b * m, c.a)
    }
}
`

const derivativePointsSource = `material AAPoint kind points {
    surface(pt) -> color {
        let d = length(pt.pointUV - vec2f(0.5, 0.5)) - 0.4
        let aa = max(fwidth(d), 0.0008)
        let m = 1.0 - smoothstep(0.0 - aa, 0.0 + aa, d)
        return rgb(pt.color.r * m, pt.color.g * m, pt.color.b * m, pt.alpha)
    }
}
`

const derivativeMeshSource = `material AAMesh {
    surface(geo) -> color {
        let d = geo.uv.x - 0.5
        let aa = max(fwidth(d), 0.0008)
        let m = 1.0 - smoothstep(0.0 - aa, 0.0 + aa, d)
        return rgb(m, m, m)
    }
}
`

// TestGLSLDerivativesRequestExtensionAndCompileAsESSL100 is the regression gate
// for the silent backend divergence this branch exists to close: a derivative
// builtin that WGSL accepts must not produce GLSL the WebGL driver rejects.
//
// Points and feedback fragments previously emitted no
// GL_OES_standard_derivatives directive at all — `selena check` passed, WGSL
// rendered, and the WebGL pass failed to compile with "fwidth: no matching
// overloaded function found". The assertion is on validity under the real
// target version (GLSL ES 1.00), not merely on emission succeeding.
func TestGLSLDerivativesRequestExtensionAndCompileAsESSL100(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"post", derivativePostSource},
		{"points", derivativePointsSource},
		{"mesh", derivativeMeshSource},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := compilePost(t, tc.src)
			vert, frag, err := glsl.Emit(res.Module)
			if err != nil {
				t.Fatalf("glsl.Emit: %v", err)
			}
			mustContain(t, tc.name, frag, "#extension GL_OES_standard_derivatives : enable")
			// The directive must precede every non-preprocessor statement.
			if idx := strings.Index(frag, "precision"); idx >= 0 {
				if strings.Index(frag, "#extension") > idx {
					t.Fatalf("%s: #extension must precede `precision`:\n%s", tc.name, frag)
				}
			}
			prismvalidate.Shader(t, "glslangValidator", essl100(vert), ".vert", nil)
			prismvalidate.Shader(t, "glslangValidator", essl100(frag), ".frag", nil)
		})
	}
}

// TestGLSLNoDerivativesEmitsNoExtension keeps the directive off shaders that do
// not need it, so v0.4.0 output for derivative-free materials is unchanged.
func TestGLSLNoDerivativesEmitsNoExtension(t *testing.T) {
	res := compilePost(t, `material Passthrough kind post {
    surface(post) -> color {
        return sceneColor(post.uv)
    }
}
`)
	_, frag, err := glsl.Emit(res.Module)
	if err != nil {
		t.Fatalf("glsl.Emit: %v", err)
	}
	mustNotContain(t, "passthrough", frag, "#extension")
}

// TestESSL100RejectsDerivativesWithoutExtension proves the gate above has
// teeth: strip the directive and glslangValidator must reject the shader. If
// this ever passes, the ES 1.00 rules are not being enforced and the sibling
// test is vacuous.
func TestESSL100RejectsDerivativesWithoutExtension(t *testing.T) {
	res := compilePost(t, derivativePostSource)
	_, frag, err := glsl.Emit(res.Module)
	if err != nil {
		t.Fatalf("glsl.Emit: %v", err)
	}
	stripped := strings.ReplaceAll(frag, "#extension GL_OES_standard_derivatives : enable\n", "")
	out, runErr := prismvalidate.Run("glslangValidator", essl100(stripped), ".frag", nil)
	if out.Skipped {
		t.Skip("glslangValidator not installed")
	}
	if runErr == nil {
		t.Fatalf("expected GLSL ES 1.00 to reject fwidth without GL_OES_standard_derivatives, got success:\n%s", out.Output)
	}
}

// TestGLESDerivativesAreCoreAndCompile pins the other half of the parity claim:
// GLSL ES 3.00 has derivatives in core, so the GLES artifact must NOT carry the
// ES 1.00 extension directive and must still compile.
func TestGLESDerivativesAreCoreAndCompile(t *testing.T) {
	res := compilePost(t, derivativePostSource)
	vert, frag, err := gles.Emit(res.Module)
	if err != nil {
		t.Fatalf("gles.Emit: %v", err)
	}
	mustNotContain(t, "gles", frag, "GL_OES_standard_derivatives")
	mustContain(t, "gles", frag, "#version 300 es")
	mustContain(t, "gles", frag, "fwidth(")
	prismvalidate.Shader(t, "glslangValidator", vert, ".vert", nil)
	prismvalidate.Shader(t, "glslangValidator", frag, ".frag", nil)
}

// TestDerivativesDeclareGLExtensionRequirement checks the half of the contract
// the shader source cannot express. WebGL only honours a shader `#extension`
// directive when the host has already called gl.getExtension for it, so the
// descriptor has to name it.
func TestDerivativesDeclareGLExtensionRequirement(t *testing.T) {
	res := compilePost(t, derivativePostSource)
	got := res.Layout.Requires.GLExtensions
	if len(got) != 1 || got[0] != "OES_standard_derivatives" {
		t.Fatalf("Requires.GLExtensions = %v, want [OES_standard_derivatives]", got)
	}
	plain := compilePost(t, `material Passthrough kind post {
    surface(post) -> color {
        return sceneColor(post.uv)
    }
}
`)
	if !plain.Layout.Requires.IsZero() {
		t.Fatalf("passthrough material declared host requirements: %+v", plain.Layout.Requires)
	}
}

const backdropLODSource = `material Frost kind post {
    param radius : float = 3.0
    surface(post) -> color {
        let s = sceneSize()
        let px = 1.0 / s.x
        let blur = sceneColorLevel(post.uv, radius)
        let c = sceneColor(vec2f(post.uv.x + px, post.uv.y))
        return rgb(mix(c.r, blur.r, 0.8), mix(c.g, blur.g, 0.8), mix(c.b, blur.b, 0.8), c.a)
    }
}
`

// TestBackdropLODAndSizeCompileOnEveryBackend covers sceneColorLevel(uv, lod)
// and sceneSize(). Each backend spells them differently and GLSL ES 1.00 needs
// both an extension and a host-set uniform, so all four are checked with the
// real compilers where one exists.
func TestBackdropLODAndSizeCompileOnEveryBackend(t *testing.T) {
	res := compilePost(t, backdropLODSource)

	wgslArt, ok := res.Artifact(selena.TargetWGSL)
	if !ok {
		t.Fatal("no WGSL artifact")
	}
	mustContain(t, "wgsl", wgslArt.Source, "textureSampleLevel(_sceneColorTex, _sceneColorSamp,")
	mustContain(t, "wgsl", wgslArt.Source, "textureDimensions(_sceneColorTex, 0)")
	prismvalidate.Shader(t, "naga", wgslArt.Source, ".wgsl", nil)

	glVert, glFrag, err := glsl.Emit(res.Module)
	if err != nil {
		t.Fatalf("glsl.Emit: %v", err)
	}
	mustContain(t, "glsl", glFrag, "#extension GL_EXT_shader_texture_lod : enable")
	mustContain(t, "glsl", glFrag, "texture2DLodEXT(_sceneColor,")
	mustContain(t, "glsl", glFrag, "uniform vec2 _sceneSize;")
	prismvalidate.Shader(t, "glslangValidator", essl100(glVert), ".vert", nil)
	prismvalidate.Shader(t, "glslangValidator", essl100(glFrag), ".frag", nil)

	esVert, esFrag, err := gles.Emit(res.Module)
	if err != nil {
		t.Fatalf("gles.Emit: %v", err)
	}
	mustContain(t, "gles", esFrag, "textureLod(_sceneColor,")
	mustContain(t, "gles", esFrag, "vec2(textureSize(_sceneColor, 0))")
	mustNotContain(t, "gles", esFrag, "_sceneSize")
	prismvalidate.Shader(t, "glslangValidator", esVert, ".vert", nil)
	prismvalidate.Shader(t, "glslangValidator", esFrag, ".frag", nil)

	metalArt, ok := res.Artifact(selena.TargetMetal)
	if !ok {
		t.Fatal("no Metal artifact")
	}
	mustContain(t, "metal", metalArt.Source, "level(u.radius)")
	mustContain(t, "metal", metalArt.Source, "_sceneColorTex.get_width()")
}

// TestBackdropLODDeclaresHostRequirements is the anti-silent-failure gate for
// sceneColorLevel: without a mipped scene-color target every backend clamps to
// level 0 and the pass renders unblurred instead of failing. The descriptor has
// to say so.
func TestBackdropLODDeclaresHostRequirements(t *testing.T) {
	res := compilePost(t, backdropLODSource)
	req := res.Layout.Requires
	if !req.SceneColorMips {
		t.Fatal("Requires.SceneColorMips = false, want true for a sceneColorLevel material")
	}
	if req.GLSceneSizeUniform != "_sceneSize" {
		t.Fatalf("Requires.GLSceneSizeUniform = %q, want _sceneSize", req.GLSceneSizeUniform)
	}
	found := false
	for _, ext := range req.GLExtensions {
		if ext == "EXT_shader_texture_lod" {
			found = true
		}
	}
	if !found {
		t.Fatalf("Requires.GLExtensions = %v, want to include EXT_shader_texture_lod", req.GLExtensions)
	}
}

// TestBackdropBuiltinsArePostOnly keeps the new builtins off the kinds that
// have no engine backdrop bound.
func TestBackdropBuiltinsArePostOnly(t *testing.T) {
	cases := map[string]string{
		"sceneColorLevel": `material M {
    surface(geo) -> color {
        let c = sceneColorLevel(geo.uv, 2.0)
        return rgb(c.r, c.g, c.b)
    }
}
`,
		"sceneSize": `material M {
    surface(geo) -> color {
        let s = sceneSize()
        return rgb(s.x, s.y, 0.0)
    }
}
`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := selena.Compile([]byte(src), selena.CompileOptions{})
			if err == nil {
				t.Fatalf("%s outside a post material compiled without error", name)
			}
			if !strings.Contains(err.Error(), "post-kind materials") {
				t.Fatalf("unexpected error for %s: %v", name, err)
			}
		})
	}
}
