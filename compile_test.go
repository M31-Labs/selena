package selena

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestCompileDefaultsToAllTargets(t *testing.T) {
	src, err := os.ReadFile("examples/textured.sel")
	if err != nil {
		t.Fatal(err)
	}

	res, err := Compile(src, CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Material.Name != "Textured" || res.Module.Name != "Textured" {
		t.Fatalf("compiled material = %s/%s, want Textured", res.Material.Name, res.Module.Name)
	}
	if len(res.Artifacts) != len(AllTargets()) {
		t.Fatalf("artifacts = %d, want %d", len(res.Artifacts), len(AllTargets()))
	}
	if len(res.Layout.Textures) != 1 || res.Layout.Textures[0].Name != "albedo" {
		t.Fatalf("textures = %+v, want albedo", res.Layout.Textures)
	}

	wgsl, ok := res.Artifact(TargetWGSL)
	if !ok {
		t.Fatal("missing WGSL artifact")
	}
	if !strings.Contains(wgsl.Source, "textureSample(albedo, albedoSampler, in.vUv)") {
		t.Fatalf("WGSL artifact does not contain texture sample:\n%s", wgsl.Source)
	}
	glsl, ok := res.Artifact(TargetGLSL)
	if !ok {
		t.Fatal("missing GLSL artifact")
	}
	if glsl.Vertex == "" || glsl.Fragment == "" || glsl.Source != "" {
		t.Fatalf("GLSL artifact shape = %+v, want split vertex/fragment", glsl)
	}
}

func TestCompileMaterialSelectsNamedMaterial(t *testing.T) {
	src, err := os.ReadFile("examples/tinted.sel")
	if err != nil {
		t.Fatal(err)
	}

	base, err := Compile(src, CompileOptions{
		Material: "Base",
		Targets:  []Target{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if base.Module.Name != "Base" {
		t.Fatalf("module = %s, want Base", base.Module.Name)
	}
	if hasUniform(base, "tint") {
		t.Fatalf("Base should not include child tint uniform: %+v", base.Module.Uniforms)
	}

	derived, err := Compile(src, CompileOptions{Targets: []Target{}})
	if err != nil {
		t.Fatal(err)
	}
	if derived.Module.Name != "Tinted" {
		t.Fatalf("default module = %s, want Tinted", derived.Module.Name)
	}
	if !hasUniform(derived, "tint") || !hasUniform(derived, "baseColor") {
		t.Fatalf("Tinted uniforms missing inherited or child params: %+v", derived.Module.Uniforms)
	}
	if len(derived.Artifacts) != 0 {
		t.Fatalf("Targets: []Target{} should suppress emission, got %+v", derived.Artifacts)
	}

	again, err := CompileMaterial(derived.Program, "Base", CompileOptions{Targets: []Target{TargetWGSL}})
	if err != nil {
		t.Fatal(err)
	}
	if again.Module.Name != "Base" || len(again.Artifacts) != 1 {
		t.Fatalf("CompileMaterial result = %s/%d artifacts, want Base/1", again.Module.Name, len(again.Artifacts))
	}
}

func TestCompileRejectsUnknownMaterialAndTarget(t *testing.T) {
	src, err := os.ReadFile("examples/directional-diffuse.sel")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := Compile(src, CompileOptions{Material: "Missing"}); err == nil || !strings.Contains(err.Error(), `material "Missing" not found`) {
		t.Fatalf("unknown material error = %v", err)
	}
	if _, err := Compile(src, CompileOptions{Targets: []Target{"spirv"}}); err == nil || !strings.Contains(err.Error(), `unknown target "spirv"`) {
		t.Fatalf("unknown target error = %v", err)
	}
}

func TestCompileErrorCarriesSourceRange(t *testing.T) {
	src := []byte(`material Bad {
    surface(geo) -> color {
        return missing
    }
}`)

	_, err := Compile(src, CompileOptions{Targets: []Target{}})
	if err == nil {
		t.Fatal("Compile succeeded, want diagnostic error")
	}
	var ce *CompileError
	if !errors.As(err, &ce) {
		t.Fatalf("error type = %T, want *CompileError", err)
	}
	if len(ce.Diagnostics) != 1 {
		t.Fatalf("diagnostics = %d, want 1", len(ce.Diagnostics))
	}
	d := ce.Diagnostics[0]
	if d.Code != "SEL2001" {
		t.Fatalf("diagnostic code = %s, want SEL2001", d.Code)
	}
	if d.Range.Start.Line != 3 || d.Range.Start.Column != 16 {
		t.Fatalf("range start = %d:%d, want 3:16", d.Range.Start.Line, d.Range.Start.Column)
	}
	if !strings.Contains(err.Error(), "SEL2001 at 3:16") {
		t.Fatalf("error string = %q, want line/column context", err.Error())
	}
}

func TestCompileErrorCarriesInlineCallRange(t *testing.T) {
	src := []byte(`fn tint(c: color) -> color {
    return c
}
material Bad {
    surface(geo) -> color {
        return tint()
    }
}`)

	_, err := Compile(src, CompileOptions{Targets: []Target{}})
	if err == nil {
		t.Fatal("Compile succeeded, want diagnostic error")
	}
	var ce *CompileError
	if !errors.As(err, &ce) {
		t.Fatalf("error type = %T, want *CompileError", err)
	}
	d := ce.Diagnostics[0]
	if d.Code != "SEL2003" {
		t.Fatalf("diagnostic code = %s, want SEL2003", d.Code)
	}
	if d.Range.Start.Line != 6 || d.Range.Start.Column != 16 {
		t.Fatalf("range start = %d:%d, want 6:16", d.Range.Start.Line, d.Range.Start.Column)
	}
	if !strings.Contains(d.Message, "fn tint expects 1 args, got 0") {
		t.Fatalf("diagnostic message = %q", d.Message)
	}
}

func TestCompileErrorRejectsSuperWithoutExtends(t *testing.T) {
	src := []byte(`material Bad {
    surface(geo) -> color {
        return super.surface(geo)
    }
}`)

	_, err := Compile(src, CompileOptions{Targets: []Target{}})
	if err == nil {
		t.Fatal("Compile succeeded, want diagnostic error")
	}
	var ce *CompileError
	if !errors.As(err, &ce) {
		t.Fatalf("error type = %T, want *CompileError", err)
	}
	d := ce.Diagnostics[0]
	if d.Code != "SEL2003" {
		t.Fatalf("diagnostic code = %s, want SEL2003", d.Code)
	}
	if d.Range.Start.Line != 3 || d.Range.Start.Column != 16 {
		t.Fatalf("range start = %d:%d, want 3:16", d.Range.Start.Line, d.Range.Start.Column)
	}
	if !strings.Contains(d.Message, "super.surface used in a material with no parent") {
		t.Fatalf("diagnostic message = %q", d.Message)
	}
}

func hasUniform(res Result, name string) bool {
	for _, u := range res.Module.Uniforms {
		if u.Name == name {
			return true
		}
	}
	return false
}
