package lower

import (
	"errors"
	"strings"
	"testing"

	"m31labs.dev/selena/bindings"
	"m31labs.dev/selena/ir"
	"m31labs.dev/selena/parse"
)

// lowerTexelSrc parses src and lowers it, returning the module and descriptor.
func lowerTexelSrc(t *testing.T, src string) (ir.Module, bindings.Layout, error) {
	t.Helper()
	program, err := parse.Program([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	return LowerProgram(program, 0)
}

func TestLoadLowersToIR(t *testing.T) {
	mod, layout, err := lowerTexelSrc(t, `material M {
    param spectrum : texture2d format r16float
    surface(geo) -> color {
        let sz = size(spectrum)
        let m = load(spectrum, int(geo.uv.x * sz.x), 2i).r
        return rgb(m, m, m)
    }
}`)
	if err != nil {
		t.Fatal(err)
	}
	var sawSize, sawLoad bool
	for _, s := range mod.Fragment.Body {
		switch v := s.Value.(type) {
		case ir.TextureSize:
			sawSize = v.Texture == "spectrum"
		}
	}
	for _, s := range mod.Fragment.Body {
		if sw, ok := s.Value.(ir.Swizzle); ok {
			if l, ok := sw.E.(ir.Load); ok && l.Texture == "spectrum" {
				sawLoad = true
			}
		}
	}
	if !sawSize || !sawLoad {
		t.Fatalf("size node %v, load node %v in %+v", sawSize, sawLoad, mod.Fragment.Body)
	}
	if mod.Textures[0].Format != "r16float" {
		t.Errorf("module format = %q", mod.Textures[0].Format)
	}
	tex := layout.Textures[0]
	if tex.Format != "r16float" || tex.SampleMode != "load" {
		t.Errorf("descriptor format %q sampleMode %q", tex.Format, tex.SampleMode)
	}
	if got := layout.Requires.GLTexSizeUniforms; len(got) != 1 || got[0] != "_texSize_spectrum" {
		t.Errorf("GLTexSizeUniforms = %v", got)
	}
}

func TestSampleModeAndFormatDefaults(t *testing.T) {
	_, layout, err := lowerTexelSrc(t, `material M {
    param a : texture2d
    param b : texture2d format rgba16float
    param c : texture2d format rgba16float
    surface(geo) -> color {
        let x = sample(a, geo.uv).r
        let y = sample(b, geo.uv).r + load(b, 0i, 0i).r
        let z = load(c, 0i, 0i).r
        return rgb(x, y, z)
    }
}`)
	if err != nil {
		t.Fatal(err)
	}
	want := []struct{ name, format, mode string }{
		{"a", "", ""}, // sampled only, no format: the descriptor is as before
		{"b", "rgba16float", "both"},
		{"c", "rgba16float", "load"},
	}
	for i, w := range want {
		g := layout.Textures[i]
		if g.Name != w.name || g.Format != w.format || g.SampleMode != w.mode {
			t.Errorf("texture %d = %s/%q/%q, want %s/%q/%q", i, g.Name, g.Format, g.SampleMode, w.name, w.format, w.mode)
		}
	}
	if len(layout.Requires.GLTexSizeUniforms) != 2 {
		t.Errorf("GLTexSizeUniforms = %v, want entries for b and c only", layout.Requires.GLTexSizeUniforms)
	}
}

func TestLoadAllowedInAuthoredVertex(t *testing.T) {
	mod, layout, err := lowerTexelSrc(t, `material M {
    param gains : texture2d format r32float
    varying level : float
    vertex(geo) -> vec4 {
        level = load(gains, 3i, 0i).r + size(gains).x
        return mvp * vec4f(geo.position, 1.0)
    }
    surface(geo) -> color { return rgb(geo.level, 0.0, 0.0) }
}`)
	if err != nil {
		t.Fatal(err)
	}
	if got := ir.VertexSampledTextures(mod); len(got) != 1 || got[0] != "gains" {
		t.Errorf("vertex textures = %v", got)
	}
	if !layout.Textures[0].VertexStage {
		t.Error("gains is not marked vertex-visible")
	}
}

func TestTexelDiagnostics(t *testing.T) {
	cases := []struct {
		name, body, want string
	}{
		{"load arity", "let v = load(t, 1i)", "load(texture, ix, iy) takes 3 arguments"},
		{"size arity", "let v = size(t, 1i)", "size(texture) takes 1 argument"},
		{"float coordinate", "let v = load(t, 1.0, 2i)", "ix must be an int texel coordinate, got float"},
		{"float y coordinate", "let v = load(t, 1i, geo.uv.y)", "iy must be an int texel coordinate, got float"},
		{"not a texture", "let v = load(k, 1i, 2i)", "first argument must be a texture2d param"},
		{"size not a texture", "let v = size(k)", "first argument must be a texture2d param"},
		{"cube load", "let v = load(c, 1i, 2i)", "c is a textureCube; cube textures have no texel load"},
		{"cube size", "let v = size(c)", "c is a textureCube"},
		{"non ref", "let v = load(vec2f(1.0, 2.0), 1i, 2i)", "first argument must be a texture2d param"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := lowerTexelSrc(t, `material M {
    param t : texture2d
    param c : textureCube
    param k : float
    surface(geo) -> color {
        `+tc.body+`
        return rgb(1.0, 1.0, 1.0)
    }
}`)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want it to contain %q", err, tc.want)
			}
			var de *DiagnosticError
			if !errors.As(err, &de) {
				t.Fatalf("error %T is not a DiagnosticError", err)
			}
		})
	}
}

func TestFormatDiagnostics(t *testing.T) {
	_, _, err := lowerTexelSrc(t, `material M {
    param t : texture2d format rgb10
    surface(geo) -> color { return rgb(1.0, 1.0, 1.0) }
}`)
	if err == nil || !strings.Contains(err.Error(), `unknown texture format "rgb10"`) ||
		!strings.Contains(err.Error(), "rgba8unorm, rgba16float, rgba32float, r16float, r32float") {
		t.Fatalf("error = %v", err)
	}
	_, _, err = lowerTexelSrc(t, `material M {
    param x : float format r16float
    surface(geo) -> color { return rgb(x, x, x) }
}`)
	if err == nil || !strings.Contains(err.Error(), "a format applies only to texture2d params") {
		t.Fatalf("error = %v", err)
	}
}

func TestSampleOfFloat32TextureIsRejected(t *testing.T) {
	for _, format := range []string{"r32float", "rgba32float"} {
		for _, call := range []string{"sample(t, geo.uv)", "sampleLevel(t, geo.uv, 0.0)"} {
			_, _, err := lowerTexelSrc(t, `material M {
    param t : texture2d format `+format+`
    surface(geo) -> color {
        let v = `+call+`
        return rgb(v.r, v.g, v.b)
    }
}`)
			if err == nil || !strings.Contains(err.Error(), "float32-filterable") ||
				!strings.Contains(err.Error(), "load(t, ix, iy)") {
				t.Errorf("%s with %s: error = %v", call, format, err)
			}
		}
	}
	// 16-bit float formats filter everywhere, and load() on a float32 texture is
	// the supported read.
	if _, _, err := lowerTexelSrc(t, `material M {
    param a : texture2d format rgba16float
    param b : texture2d format r32float
    surface(geo) -> color {
        let v = sample(a, geo.uv).r + load(b, 0i, 0i).r
        return rgb(v, v, v)
    }
}`); err != nil {
		t.Fatal(err)
	}
}

func TestLocalsNamedSizeAndLoadStayLegal(t *testing.T) {
	if _, _, err := lowerTexelSrc(t, `material M {
    param size : float = 1.0
    surface(geo) -> color {
        let load = size * 2.0
        return rgb(load, size, 0.0)
    }
}`); err != nil {
		t.Fatal(err)
	}
}
