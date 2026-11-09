package selena

import (
	"os"
	"strings"
	"testing"
)

const dataTextureMaterial = `material Scope {
    param row : texture2d format r16float
    param lut : texture2d
    surface(geo) -> color {
        let sz = size(row)
        let v = load(row, int(geo.uv.x * sz.x), 1i).r
        let c = sample(lut, geo.uv)
        return rgb(v * c.r, v * c.g, v * c.b)
    }
}`

func TestDataTextureEmitsPerTarget(t *testing.T) {
	r, err := Compile([]byte(dataTextureMaterial), CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		target Target
		want   []string
	}{
		{TargetWGSL, []string{
			"vec2<f32>(textureDimensions(row))",
			"textureLoad(row, vec2<i32>(i32((in.vUv.x * sz.x)), 1i), 0).r",
			"textureSample(lut, lutSampler, in.vUv)",
			"var rowSampler : sampler;", // the sampler stays so binding numbers do not move
		}},
		{TargetGLES, []string{
			"vec2(textureSize(row, 0))",
			"texelFetch(row, ivec2(int((vUv.x * sz.x)), 1), 0).r",
		}},
		{TargetGLSL, []string{
			"uniform highp vec2 _texSize_row;",
			"vec2 sz = _texSize_row;",
			"texture2D(row, (vec2(int((vUv.x * sz.x)), 1) + 0.5) / _texSize_row).r",
		}},
		{TargetMetal, []string{
			"float2(row.get_width(), row.get_height())",
			"row.read(uint2(int((in.vUv.x * sz.x)), 1), 0).r",
		}},
	}
	for _, tc := range cases {
		t.Run(string(tc.target), func(t *testing.T) {
			a, ok := r.Artifact(tc.target)
			if !ok {
				t.Fatal("no artifact")
			}
			src := a.Source + a.Vertex + a.Fragment
			for _, w := range tc.want {
				if !strings.Contains(src, w) {
					t.Errorf("missing %q in:\n%s", w, src)
				}
			}
		})
	}
	// Only the load()ed texture gets a size uniform; lut is sampled only.
	g, _ := r.Artifact(TargetGLSL)
	if strings.Contains(g.Fragment, "_texSize_lut") {
		t.Errorf("a sampled-only texture got a size uniform:\n%s", g.Fragment)
	}
	if got := r.Layout.Requires.GLTexSizeUniforms; len(got) != 1 || got[0] != "_texSize_row" {
		t.Errorf("GLTexSizeUniforms = %v", got)
	}
}

// The GLSL ES 1.00 vertex stage declares the size uniform with the same
// precision as the fragment stage, or the program would fail to link.
func TestDataTextureGLSLVertexAndFragmentAgreeOnSizeUniform(t *testing.T) {
	src, err := os.ReadFile("testdata/conformance/data-texture.sel")
	if err != nil {
		t.Fatal(err)
	}
	r, err := Compile(src, CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	g, _ := r.Artifact(TargetGLSL)
	if !strings.Contains(g.Vertex, "uniform highp vec2 _texSize_gains;") {
		t.Errorf("vertex lacks _texSize_gains:\n%s", g.Vertex)
	}
	for _, name := range []string{"spectrum", "palette"} {
		if !strings.Contains(g.Fragment, "uniform highp vec2 _texSize_"+name+";") {
			t.Errorf("fragment lacks _texSize_%s:\n%s", name, g.Fragment)
		}
	}
	if strings.Contains(g.Fragment, "_texSize_gains") {
		t.Errorf("fragment declares a size uniform it does not use:\n%s", g.Fragment)
	}
}

func TestDataTextureDescriptorJSON(t *testing.T) {
	r, err := Compile([]byte(dataTextureMaterial), CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	js, err := r.Layout.JSON()
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range []string{`"format": "r16float"`, `"sampleMode": "load"`, `"glTexSizeUniforms"`, `"_texSize_row"`} {
		if !strings.Contains(js, w) {
			t.Errorf("descriptor JSON lacks %s:\n%s", w, js)
		}
	}
	if strings.Count(js, `"format"`) != 1 || strings.Count(js, `"sampleMode"`) != 1 {
		t.Errorf("an undeclared format or sampled-only texture got the new fields:\n%s", js)
	}
}
