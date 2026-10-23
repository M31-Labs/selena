package selena

import (
	"slices"
	"strings"
	"testing"
)

const engineDepthMaterial = `material Depthy {
    context {
        opaqueDepth : textureDepth2d
        opaqueColor : texture2d
    }
    vertex(geo) -> vec4 {
        let d = sampleDepth(opaqueDepth, geo.uv)
        return mvp * vec4f(geo.position * (1.0 + d), 1.0)
    }
    surface(geo) -> color {
        let d = sampleDepth(opaqueDepth, vec2f(0.25, 0.75))
        return sample(opaqueColor, vec2f(0.5, 0.5)).rgb * d
    }
}`

func TestEngineDepthEmitsEveryTarget(t *testing.T) {
	r, err := Compile([]byte(engineDepthMaterial), CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		target Target
		decl   string
		read   string
		stage  func(a Artifact) string
	}{
		{TargetWGSL, "var opaqueDepth : texture_depth_2d;", "textureLoad(opaqueDepth, vec2<i32>(clamp(floor(", func(a Artifact) string { return a.Source }},
		{TargetGLSL, "uniform highp sampler2D opaqueDepth;", "texture2D(opaqueDepth, ", func(a Artifact) string { return a.Vertex + a.Fragment }},
		{TargetGLES, "uniform highp sampler2D opaqueDepth;", "textureLod(opaqueDepth, ", func(a Artifact) string { return a.Vertex + a.Fragment }},
		{TargetMetal, "depth2d<float> opaqueDepth [[texture(0)]]", "opaqueDepth.sample(opaqueDepthSampler, ", func(a Artifact) string { return a.Source }},
	} {
		a, _ := r.Artifact(c.target)
		src := c.stage(a)
		if !strings.Contains(src, c.decl) || !strings.Contains(src, c.read) {
			t.Errorf("%s: missing %q or %q\n%s", c.target, c.decl, c.read, src)
		}
	}
	// Vertex stage declares the depth texture on every target.
	for _, target := range []Target{TargetGLSL, TargetGLES} {
		a, _ := r.Artifact(target)
		if !strings.Contains(a.Vertex, "uniform highp sampler2D opaqueDepth;") || !strings.Contains(a.Vertex, "opaqueDepth,") {
			t.Errorf("%s vertex shader does not read opaqueDepth:\n%s", target, a.Vertex)
		}
		if strings.Contains(a.Vertex, "opaqueColor") {
			t.Errorf("%s vertex shader declares the fragment-only opaqueColor", target)
		}
	}
}

func TestEngineDepthWebGL1RequiresDepthTextureExtension(t *testing.T) {
	r, err := Compile([]byte(engineDepthMaterial), CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(r.Layout.Requires.GLExtensions, "WEBGL_depth_texture") {
		t.Fatalf("GLExtensions = %v, want WEBGL_depth_texture", r.Layout.Requires.GLExtensions)
	}
	plain, err := Compile([]byte(`material P { context { opaqueColor : texture2d } surface(geo) -> color { return sample(opaqueColor, vec2f(0.5, 0.5)).rgb } }`), CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(plain.Layout.Requires.GLExtensions, "WEBGL_depth_texture") {
		t.Fatalf("a material with no depth texture asks for WEBGL_depth_texture: %v", plain.Layout.Requires.GLExtensions)
	}
}
