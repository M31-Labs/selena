package parse

import (
	"os"
	"strings"
	"testing"

	"m31labs.dev/selena/emit/wgsl"
	"m31labs.dev/selena/lower"
)

// TestParseDirectionalDiffuse parses the .sel source and confirms it lowers to
// the same shader the hand-built HIR did — i.e. the front-end reproduces M1.
func TestParseDirectionalDiffuse(t *testing.T) {
	m, err := Material([]byte(`material DirectionalDiffuse {
    param baseColor : color
    param light     : Sun
    surface(geo) -> color {
        let n = normalize(geo.worldNormal)
        return baseColor * (light.ambient + max(dot(n, light.dir), 0))
    }
}`))
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "DirectionalDiffuse" || len(m.Params) != 2 {
		t.Fatalf("parsed material = %+v", m)
	}
	mod, _, err := lower.Lower(m)
	if err != nil {
		t.Fatal(err)
	}
	src, _ := wgsl.Emit(mod)
	for _, want := range []string{
		"out.vWorldNormal = normalize((u.normalMatrix * in.normal));",
		"let n = normalize(in.vWorldNormal);",
		"return vec4<f32>((u.baseColor * (u.light_ambient + max(dot(n, u.light_dir), 0.0))), 1.0);",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("WGSL missing %q\n--- got ---\n%s", want, src)
		}
	}
}

// TestParseTextured covers the M2 features through the front-end: a texture
// param, sample(), a swizzle, and the uv interpolant.
func TestParseTextured(t *testing.T) {
	m, err := Material([]byte(`material Textured {
    param albedo : texture2d
    param light  : Sun
    surface(geo) -> color {
        let c = sample(albedo, geo.uv).rgb
        return c * (light.ambient + max(dot(normalize(geo.worldNormal), light.dir), 0))
    }
}`))
	if err != nil {
		t.Fatal(err)
	}
	mod, layout, err := lower.Lower(m)
	if err != nil {
		t.Fatal(err)
	}
	if len(layout.Textures) != 1 || layout.Textures[0].Name != "albedo" {
		t.Fatalf("textures = %+v", layout.Textures)
	}
	src, _ := wgsl.Emit(mod)
	if !strings.Contains(src, "textureSample(albedo, albedoSampler, in.vUv).rgb") {
		t.Errorf("WGSL missing texture sample\n%s", src)
	}
}

// TestParseExampleFiles parses the checked-in .sel examples end to end.
func TestParseExampleFiles(t *testing.T) {
	for _, f := range []string{"../examples/directional-diffuse.sel", "../examples/textured.sel"} {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		m, err := Material(src)
		if err != nil {
			t.Fatalf("parse %s: %v", f, err)
		}
		if _, _, err := lower.Lower(m); err != nil {
			t.Fatalf("lower %s: %v", f, err)
		}
	}
}
