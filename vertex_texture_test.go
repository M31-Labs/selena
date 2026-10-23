package selena

import (
	"strings"
	"testing"
)

const vertexTextureMaterial = `material Displace {
    param heightMap : texture2d
    param tint      : texture2d
    param scale     : float = 0.25
    varying height : float
    vertex(geo) -> vec4 {
        let h = sampleLevel(heightMap, geo.uv, 0.0).x
        height = h
        return mvp * vec4f(geo.position + geo.normal * (h * scale), 1.0)
    }
    surface(geo) -> color {
        let c = sample(tint, vec2f(0.5, 0.5))
        return rgb(c.x * geo.height, c.y, c.z)
    }
}`

// vertexStageOf returns the text of the vertex entry point for a target, so
// assertions cannot be satisfied by the fragment stage.
func vertexStageOf(t *testing.T, target Target) string {
	t.Helper()
	r, err := Compile([]byte(vertexTextureMaterial), CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	a, ok := r.Artifact(target)
	if !ok {
		t.Fatalf("no %s artifact", target)
	}
	switch target {
	case TargetGLSL, TargetGLES:
		return a.Vertex
	case TargetWGSL:
		i := strings.Index(a.Source, "@vertex")
		j := strings.Index(a.Source, "@fragment")
		return a.Source[i:j]
	default: // Metal
		i := strings.Index(a.Source, "vertex VertexOut vertexMain")
		j := strings.Index(a.Source, "fragment float4 fragmentMain")
		return a.Source[i:j]
	}
}

func TestVertexSampleLevelEmitsEveryTarget(t *testing.T) {
	cases := []struct {
		target Target
		want   []string
		not    []string
	}{
		{TargetWGSL, []string{"textureSampleLevel(heightMap, heightMapSampler, in.uv, 0.0)"}, []string{"textureSample(", "tint"}},
		{TargetGLSL, []string{"uniform highp sampler2D heightMap;", "texture2DLod(heightMap, uv, 0.0)"}, []string{"tint"}},
		{TargetGLES, []string{"uniform highp sampler2D heightMap;", "textureLod(heightMap, uv, 0.0)"}, []string{"tint"}},
		{TargetMetal, []string{"texture2d<float> heightMap [[texture(0)]]", "sampler heightMapSampler [[sampler(0)]]",
			"heightMap.sample(heightMapSampler, in.uv, level(0.0))"}, []string{"tint"}},
	}
	for _, c := range cases {
		t.Run(string(c.target), func(t *testing.T) {
			v := vertexStageOf(t, c.target)
			for _, w := range c.want {
				if !strings.Contains(v, w) {
					t.Errorf("vertex stage missing %q\n%s", w, v)
				}
			}
			for _, w := range c.not {
				if strings.Contains(v, w) {
					t.Errorf("vertex stage contains %q, want it fragment-only\n%s", w, v)
				}
			}
		})
	}
}

// The tint texture keeps its bindings (and its place in the fragment stage)
// when a sibling texture is also read by the vertex stage.
func TestVertexSampleLevelDescriptor(t *testing.T) {
	r, err := Compile([]byte(vertexTextureMaterial), CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	js, err := r.Layout.JSON()
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(js), `"vertexStage": true`) + strings.Count(string(js), `"vertexStage":true`); n != 1 {
		t.Fatalf("descriptor marks %d textures vertexStage, want 1:\n%s", n, js)
	}
	if !strings.Contains(string(js), `"glVertexTextureUnits": 1`) && !strings.Contains(string(js), `"glVertexTextureUnits":1`) {
		t.Fatalf("descriptor lacks glVertexTextureUnits=1:\n%s", js)
	}
}
