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

func TestParseParamDefaults(t *testing.T) {
	m, err := Material([]byte(`material Defaults {
    param baseColor : color = rgb(0.25, 0.5, 0.75)
    param gain : float = 1.5
    surface(geo) -> color { return baseColor * gain }
}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Params) != 2 {
		t.Fatalf("params = %d, want 2", len(m.Params))
	}
	if m.Params[0].Default == nil || m.Params[1].Default == nil {
		t.Fatalf("defaults were not parsed: %+v", m.Params)
	}
	if m.Params[0].Span.Start.Line != 2 {
		t.Fatalf("param span start line = %d, want 2", m.Params[0].Span.Start.Line)
	}
}

// TestParseExampleFiles parses the checked-in .sel examples end to end.
func TestParseExampleFiles(t *testing.T) {
	for _, f := range []string{"../examples/directional-diffuse.sel", "../examples/textured.sel", "../examples/defaults.sel"} {
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

// TestParseComposedFunc proves composition: a user function inlines to the
// exact same shader as the hand-written DirectionalDiffuse.
func TestParseComposedFunc(t *testing.T) {
	p, err := Program([]byte(`
fn diffuse(n: vec3, dir: vec3, ambient: float) -> float {
    return ambient + max(dot(n, dir), 0.0)
}
material Composed {
    param baseColor : color
    param light : Sun
    surface(geo) -> color {
        let n = normalize(geo.worldNormal)
        return baseColor * diffuse(n, light.dir, light.ambient)
    }
}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Funcs) != 1 || len(p.Materials) != 1 {
		t.Fatalf("program = %d funcs, %d materials", len(p.Funcs), len(p.Materials))
	}
	mod, _, err := lower.LowerWith(p.Materials[0], p.Funcs)
	if err != nil {
		t.Fatal(err)
	}
	src, _ := wgsl.Emit(mod)
	want := "return vec4<f32>((u.baseColor * (u.light_ambient + max(dot(n, u.light_dir), 0.0))), 1.0);"
	if !strings.Contains(src, want) {
		t.Errorf("composed fn did not inline to the expected shader\n--- got ---\n%s", src)
	}
}

// TestParseExtends covers material inheritance: Tinted extends Base, adds a
// param, and reuses the parent surface via super.surface(geo).
func TestParseExtends(t *testing.T) {
	p, err := Program([]byte(`
material Base {
    param baseColor : color
    param light : Sun
    surface(geo) -> color {
        let n = normalize(geo.worldNormal)
        return baseColor * (light.ambient + max(dot(n, light.dir), 0))
    }
}
material Tinted extends Base {
    param tint : color
    surface(geo) -> color { return super.surface(geo) * tint }
}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Materials) != 2 {
		t.Fatalf("materials = %d, want 2", len(p.Materials))
	}
	mod, _, err := lower.LowerProgram(p, 1) // the derived material
	if err != nil {
		t.Fatal(err)
	}
	var hasTint, hasBase bool
	for _, u := range mod.Uniforms {
		switch u.Name {
		case "tint":
			hasTint = true
		case "baseColor":
			hasBase = true
		}
	}
	if !hasTint || !hasBase {
		t.Fatalf("merged uniforms missing tint/baseColor: %+v", mod.Uniforms)
	}
	src, _ := wgsl.Emit(mod)
	if !strings.Contains(src, "u.tint") || !strings.Contains(src, "normalize(in.vWorldNormal)") {
		t.Errorf("extends/super did not inline the base surface * tint\n--- got ---\n%s", src)
	}
}
