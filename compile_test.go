package selena

import (
	"errors"
	"os"
	"strings"
	"testing"

	"m31labs.dev/selena/hir"
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

func TestCompileErrorRejectsReservedNames(t *testing.T) {
	src := []byte(`material Bad {
    param var : float
    surface(geo) -> color { return rgb(var, var, var) }
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
	if d.Code != "SEL1004" {
		t.Fatalf("diagnostic code = %s, want SEL1004", d.Code)
	}
	if d.Range.Start.Line != 2 || d.Range.Start.Column != 5 {
		t.Fatalf("range start = %d:%d, want 2:5", d.Range.Start.Line, d.Range.Start.Column)
	}
	if !strings.Contains(d.Message, `param "var" is reserved for WGSL binding keyword`) {
		t.Fatalf("diagnostic message = %q", d.Message)
	}
}

// TestCompileProgramRejectsVertexHookOnNonMesh verifies the non-mesh kinds still
// reject author vertex() hooks (mesh/general supports them as of B4).
func TestCompileProgramRejectsVertexHookOnNonMesh(t *testing.T) {
	_, err := CompileProgram(hir.Program{Materials: []hir.Material{{
		Name: "Bad",
		Kind: hir.KindPoints,
		Vertex: &hir.Func{
			Span:   hir.Span{Start: hir.Position{Line: 2, Column: 5}},
			Geo:    "geo",
			Result: hir.Call{Func: "vec4f", Args: []hir.Expr{hir.Lit{Value: 0}, hir.Lit{Value: 0}, hir.Lit{Value: 0}, hir.Lit{Value: 1}}},
		},
		Surface: hir.Func{
			Geo:    "geo",
			Result: hir.Call{Func: "rgb", Args: []hir.Expr{hir.Lit{Value: 1}, hir.Lit{Value: 1}, hir.Lit{Value: 1}}},
		},
	}}}, CompileOptions{Targets: []Target{}})
	if err == nil {
		t.Fatal("CompileProgram succeeded, want diagnostic error")
	}
	var ce *CompileError
	if !errors.As(err, &ce) {
		t.Fatalf("error type = %T, want *CompileError", err)
	}
	d := ce.Diagnostics[0]
	if d.Code != "SEL1005" {
		t.Fatalf("diagnostic code = %s, want SEL1005", d.Code)
	}
	if d.Range.Start.Line != 2 || d.Range.Start.Column != 5 {
		t.Fatalf("range start = %d:%d, want 2:5", d.Range.Start.Line, d.Range.Start.Column)
	}
	if !strings.Contains(d.Message, "not supported in points") {
		t.Fatalf("diagnostic message = %q", d.Message)
	}
}

// TestCompileUniformBlockOffsetsMatchEmittedWGSLOrderForScalarAfterArray is an
// end-to-end regression test for a real bug: bindings.ComputeUniformBlock used
// to pack uniform-block fields in raw declaration order, but every backend
// emitter (WGSL/GLSL/GLES/Metal) renders scalars/vectors/matrices first and
// fixed-size arrays last (see emit/wgsl/wgsl.go emitMesh, which writes
// m.Uniforms then m.ArrayUniforms into one struct). A material declaring a
// scalar AFTER an array — like this one's "b" — used to get a descriptor
// whose "b" offset landed after "arr", even though the emitted struct puts it
// before. This locks the offsets to agree with the emitted struct's actual
// member order.
func TestCompileUniformBlockOffsetsMatchEmittedWGSLOrderForScalarAfterArray(t *testing.T) {
	src := []byte(`material ScalarAfterArray {
    param a : float
    param arr : array<vec4, 4>
    param b : float

    surface(geo) -> color {
        let item = arr[0i]
        return rgb(a + b, item.x, item.y)
    }
}`)
	res, err := Compile(src, CompileOptions{Targets: []Target{TargetWGSL}})
	if err != nil {
		t.Fatal(err)
	}

	wgslArtifact, ok := res.Artifact(TargetWGSL)
	if !ok {
		t.Fatal("missing WGSL artifact")
	}

	// Confirm the emitted struct really does put a,b (scalars) before arr
	// (array) — pins the emitter behavior this test's invariant depends on.
	idxA := strings.Index(wgslArtifact.Source, "\n  a : f32,\n")
	idxB := strings.Index(wgslArtifact.Source, "\n  b : f32,\n")
	idxArr := strings.Index(wgslArtifact.Source, "\n  arr : array<vec4<f32>, 4>,\n")
	if idxA < 0 || idxB < 0 || idxArr < 0 {
		t.Fatalf("could not locate uniform members in emitted WGSL:\n%s", wgslArtifact.Source)
	}
	if !(idxA < idxArr && idxB < idxArr) {
		t.Fatalf("expected emitted WGSL struct to order scalars a,b before array arr; got a=%d b=%d arr=%d in:\n%s", idxA, idxB, idxArr, wgslArtifact.Source)
	}

	// The compiled descriptor's byte offsets must agree with that emitted
	// order: a and b must be offset before arr.
	offsets := map[string]int{}
	for _, f := range res.Layout.UniformBlock.Fields {
		offsets[f.Name] = f.Offset
	}
	oa, ok := offsets["a"]
	if !ok {
		t.Fatal("descriptor missing field a")
	}
	ob, ok := offsets["b"]
	if !ok {
		t.Fatal("descriptor missing field b")
	}
	oarr, ok := offsets["arr"]
	if !ok {
		t.Fatal("descriptor missing field arr")
	}
	if !(oa < oarr && ob < oarr) {
		t.Fatalf("descriptor offsets disagree with emitted struct order: a@%d b@%d arr@%d (want a,b before arr)", oa, ob, oarr)
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
