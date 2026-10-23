package lower

import (
	"errors"
	"strings"
	"testing"

	"m31labs.dev/selena/bindings"
)

const engineTextureSrc = `material Water {
    context {
        planarReflection : texture2d
        opaqueColor      : texture2d
        opaqueDepth      : textureDepth2d
    }
    param bump : texture2d
    surface(geo) -> color {
        let uv = vec2f(0.5, 0.5)
        let refl = sampleLevel(planarReflection, uv, 0.0).rgb
        let under = sample(opaqueColor, uv).rgb
        let d = sampleDepth(opaqueDepth, uv)
        let b = sample(bump, uv).r
        return refl * d + under * (1.0 - d) * b
    }
}`

func TestEngineTexturesInContextDescriptor(t *testing.T) {
	layout, err := lowerEngineSrc(t, engineTextureSrc)
	if err != nil {
		t.Fatal(err)
	}
	if layout.EngineVersion != bindings.EngineVersion {
		t.Errorf("EngineVersion = %q", layout.EngineVersion)
	}
	want := []struct{ name, dim, engine string }{
		{"bump", "2d", ""}, // author params first, then context textures in declaration order
		{"planarReflection", "2d", "planarReflection"},
		{"opaqueColor", "2d", "opaqueColor"},
		{"opaqueDepth", "depth2d", "opaqueDepth"},
	}
	if len(layout.Textures) != len(want) {
		t.Fatalf("textures = %+v", layout.Textures)
	}
	for i, w := range want {
		got := layout.Textures[i]
		if got.Name != w.name || got.Dimension != w.dim || got.Engine != w.engine {
			t.Errorf("texture %d = %+v, want %+v", i, got, w)
		}
		if got.WGSL.TextureBinding != 1+2*i || got.WGSL.SamplerBinding != 2+2*i || got.GL.Unit != i || got.Metal.Texture != i {
			t.Errorf("texture %d bindings = %+v", i, got)
		}
	}
	if len(layout.Context) != 0 {
		t.Errorf("Context lists uniform fields only; got %v", layout.Context)
	}
	for _, f := range layout.UniformBlock.Fields {
		if f.Name == "planarReflection" || f.Name == "opaqueDepth" {
			t.Errorf("texture %s leaked into the uniform block", f.Name)
		}
	}
}

func TestEngineTextureDiagnostics(t *testing.T) {
	cases := []struct {
		name, src, code, want string
	}{
		{"unregistered texture", `material M { context { foo : texture2d } surface(geo) -> color { return rgb(1.0, 0.0, 0.0) } }`,
			CodeUnsupportedType, "engine textures"},
		{"wrong type for engine texture", `material M { context { opaqueColor : vec3 } surface(geo) -> color { return rgb(1.0, 0.0, 0.0) } }`,
			CodeEngineInput, "engine input opaqueColor is texture2d"},
		{"depth declared as color texture", `material M { context { opaqueDepth : texture2d } surface(geo) -> color { return rgb(1.0, 0.0, 0.0) } }`,
			CodeEngineInput, "engine input opaqueDepth is textureDepth2d"},
		{"cube in context", `material M { context { sky : textureCube } surface(geo) -> color { return rgb(1.0, 0.0, 0.0) } }`,
			CodeUnsupportedType, "engine textures"},
		{"points kind", `material P kind points { context { opaqueColor : texture2d } surface(pt) -> color { return rgb(pt.color.r, pt.color.g, pt.color.b, pt.alpha) } }`,
			CodeUnsupportedType, "mesh materials"},
		{"sampleDepth on a color texture", `material M { context { opaqueColor : texture2d } surface(geo) -> color { return rgb(sampleDepth(opaqueColor, vec2f(0.5, 0.5)), 0.0, 0.0) } }`,
			CodeInvalidCall, "textureDepth2d"},
		{"sample on a depth texture", `material M { context { opaqueDepth : textureDepth2d } surface(geo) -> color { return rgb(sample(opaqueDepth, vec2f(0.5, 0.5)).r, 0.0, 0.0) } }`,
			CodeInvalidCall, "sampleDepth"},
		{"sampleLevel on a depth texture", `material M { context { opaqueDepth : textureDepth2d } surface(geo) -> color { return rgb(sampleLevel(opaqueDepth, vec2f(0.5, 0.5), 0.0).r, 0.0, 0.0) } }`,
			CodeInvalidCall, "sampleDepth"},
		{"depth texture as a param", `material M { param d : textureDepth2d surface(geo) -> color { return rgb(1.0, 0.0, 0.0) } }`,
			CodeUnsupportedType, "textureDepth2d"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := lowerEngineSrc(t, c.src)
			var de *DiagnosticError
			if !errors.As(err, &de) {
				t.Fatalf("err = %v, want a DiagnosticError", err)
			}
			if de.Code != c.code || !strings.Contains(de.Message, c.want) {
				t.Fatalf("got %s %q, want %s containing %q", de.Code, de.Message, c.code, c.want)
			}
		})
	}
}

func TestEngineDepthTextureInVertexStage(t *testing.T) {
	layout, err := lowerEngineSrc(t, `material M {
    context { opaqueDepth : textureDepth2d  opaqueColor : texture2d }
    vertex(geo) -> vec4 {
        let d = sampleDepth(opaqueDepth, geo.uv)
        return mvp * vec4f(geo.position * (1.0 + d), 1.0)
    }
    surface(geo) -> color { return sample(opaqueColor, vec2f(0.5, 0.5)).rgb }
}`)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]bindings.Texture{}
	for _, tx := range layout.Textures {
		byName[tx.Name] = tx
	}
	if !byName["opaqueDepth"].VertexStage || byName["opaqueColor"].VertexStage {
		t.Errorf("vertexStage: depth %v color %v", byName["opaqueDepth"].VertexStage, byName["opaqueColor"].VertexStage)
	}
	if layout.Requires.GLVertexTextureUnits != 1 {
		t.Errorf("GLVertexTextureUnits = %d, want 1", layout.Requires.GLVertexTextureUnits)
	}
}
