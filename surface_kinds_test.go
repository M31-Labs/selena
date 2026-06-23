package selena

// Tests for the two new surface kinds: points (billboard particle appearance)
// and post (fullscreen post-process pass).
//
// These complement the conformance_test.go / validate/validate_test.go suites.
// They focus on:
//   1. Correct layout contract (Kind, EntryPoints, uniform group/binding)
//   2. Sema errors for misuse (wrong geo fields, scene samplers in mesh, etc.)
//   3. Parity assertions: glow .sel emits shaders structurally matching the
//      builtin WebGPU WGSL_POINTS_FRAGMENT contract (same bindings/varyings)
//   4. Lens assertion: >=3 independent sceneColor sample calls in the WGSL
//   5. Discriminative: mesh materials must NOT emit points/post scaffold

import (
	"errors"
	"os"
	"strings"
	"testing"

	"m31labs.dev/selena/bindings"
	"m31labs.dev/selena/hir"
)

// ---- layout contract tests ----

func TestPointsLayoutContract(t *testing.T) {
	src, err := os.ReadFile("examples/glow-points.sel")
	if err != nil {
		t.Fatal(err)
	}
	res, err := Compile(src, CompileOptions{Targets: []Target{TargetWGSL, TargetGLSL}})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	layout := res.Layout

	// Kind and entry points.
	if layout.Kind != bindings.SurfaceKindPoints {
		t.Errorf("layout.Kind = %q, want %q", layout.Kind, bindings.SurfaceKindPoints)
	}
	if layout.EntryPoints.Vertex != "vertexMain" || layout.EntryPoints.Fragment != "fragmentMain" {
		t.Errorf("entryPoints = %+v, want {vertexMain, fragmentMain}", layout.EntryPoints)
	}

	// User uniforms at @group(1) @binding(0) in WGSL.
	if layout.WGSL.Group != 1 || layout.WGSL.Binding != 0 {
		t.Errorf("wgsl binding = {Group:%d, Binding:%d}, want {1, 0}", layout.WGSL.Group, layout.WGSL.Binding)
	}

	// No attributes or textures (engine handles attribute plumbing).
	if len(layout.Attributes) != 0 {
		t.Errorf("attributes = %d, want 0 (engine owns attribute layout)", len(layout.Attributes))
	}
	if len(layout.Textures) != 0 {
		t.Errorf("textures = %d, want 0", len(layout.Textures))
	}

	// fogColor param in uniform block.
	if len(layout.UniformBlock.Fields) == 0 {
		t.Errorf("uniform block is empty, want fogColor field")
	}
	found := false
	for _, f := range layout.UniformBlock.Fields {
		if f.Name == "fogColor" && f.Type == "vec3" {
			found = true
		}
	}
	if !found {
		t.Errorf("fogColor : vec3 not found in uniform block: %+v", layout.UniformBlock.Fields)
	}
}

func TestPostLayoutContract(t *testing.T) {
	src, err := os.ReadFile("examples/gravitational-lens.sel")
	if err != nil {
		t.Fatal(err)
	}
	res, err := Compile(src, CompileOptions{Targets: []Target{TargetWGSL, TargetGLSL}})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	layout := res.Layout

	// Kind and entry points.
	if layout.Kind != bindings.SurfaceKindPost {
		t.Errorf("layout.Kind = %q, want %q", layout.Kind, bindings.SurfaceKindPost)
	}
	if layout.EntryPoints.Vertex != "vertexMain" || layout.EntryPoints.Fragment != "fragmentMain" {
		t.Errorf("entryPoints = %+v, want {vertexMain, fragmentMain}", layout.EntryPoints)
	}

	// User uniforms at @group(0) @binding(4) in WGSL (scene textures at 0-3).
	if layout.WGSL.Group != 0 || layout.WGSL.Binding != 4 {
		t.Errorf("wgsl binding = {Group:%d, Binding:%d}, want {0, 4}", layout.WGSL.Group, layout.WGSL.Binding)
	}

	// No mesh attributes.
	if len(layout.Attributes) != 0 {
		t.Errorf("attributes = %d, want 0 for post pass", len(layout.Attributes))
	}
	// No author textures (engine provides sceneColor/sceneDepth at fixed slots).
	if len(layout.Textures) != 0 {
		t.Errorf("textures = %d, want 0 (engine scene textures not in layout)", len(layout.Textures))
	}

	// User params in uniform block.
	paramNames := map[string]bool{}
	for _, f := range layout.UniformBlock.Fields {
		paramNames[f.Name] = true
	}
	for _, want := range []string{"lensCenterX", "lensCenterY", "strength", "softening", "maxBend"} {
		if !paramNames[want] {
			t.Errorf("param %q not in uniform block: %v", want, layout.UniformBlock.Fields)
		}
	}
}

func TestMeshLayoutContract(t *testing.T) {
	// Existing mesh materials must still report kind=mesh.
	src, err := os.ReadFile("examples/textured.sel")
	if err != nil {
		t.Fatal(err)
	}
	res, err := Compile(src, CompileOptions{Targets: []Target{}})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if res.Layout.Kind != bindings.SurfaceKindMesh {
		t.Errorf("layout.Kind = %q, want %q", res.Layout.Kind, bindings.SurfaceKindMesh)
	}
	if res.Layout.WGSL.Group != 0 || res.Layout.WGSL.Binding != 0 {
		t.Errorf("mesh wgsl binding = {%d,%d}, want {0,0}", res.Layout.WGSL.Group, res.Layout.WGSL.Binding)
	}
}

// ---- parity tests ----

// TestGlowPointsWGSLParityContract asserts that the GlowPoints .sel emits
// WGSL shaders structurally matching the builtin WGSL_POINTS_* contract:
//   - Engine bind groups (0,0), (2,0), (2,1) are declared.
//   - PointsOutput varyings (clipPos, v_color, v_fogFactor, v_alpha, v_pointCoord, v_pointSize) are present.
//   - Fragment receives: @location(0) v_color, @location(1) v_fogFactor, etc.
//   - User uniforms at @group(1) @binding(0).
//   - vertexMain and fragmentMain are both present.
func TestGlowPointsWGSLParityContract(t *testing.T) {
	src, err := os.ReadFile("examples/glow-points.sel")
	if err != nil {
		t.Fatal(err)
	}
	res, err := Compile(src, CompileOptions{Targets: []Target{TargetWGSL}})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	wgsl, ok := res.Artifact(TargetWGSL)
	if !ok {
		t.Fatal("missing WGSL artifact")
	}
	src2 := wgsl.Source

	// Engine bind groups must match the builtin contract.
	requiredBindings := []string{
		"@group(0) @binding(0) var<uniform> _frame",
		"@group(2) @binding(0) var<uniform> _points",
		"@group(2) @binding(1) var<storage, read> _particles",
		"@group(1) @binding(0) var<uniform> u",
	}
	for _, b := range requiredBindings {
		if !strings.Contains(src2, b) {
			t.Errorf("WGSL missing binding %q", b)
		}
	}

	// PointsOutput varyings match the builtin PointsOutput struct.
	requiredVaryings := []string{
		"@location(0) v_color",
		"@location(1) v_fogFactor",
		"@location(2) v_alpha",
		"@location(3) v_pointCoord",
		"@location(4) v_pointSize",
	}
	for _, v := range requiredVaryings {
		if !strings.Contains(src2, v) {
			t.Errorf("WGSL missing varying %q", v)
		}
	}

	// Entry points.
	if !strings.Contains(src2, "@vertex fn vertexMain(") {
		t.Error("WGSL missing @vertex fn vertexMain")
	}
	if !strings.Contains(src2, "@fragment fn fragmentMain(") {
		t.Error("WGSL missing @fragment fn fragmentMain")
	}

	// Fragment must use glow-style Gaussian falloff (exp keyword).
	if !strings.Contains(src2, "exp(") {
		t.Error("WGSL fragment missing exp() — glow falloff not emitted")
	}

	// Fog application must be present (mix with foggedRGB).
	if !strings.Contains(src2, "v_fogFactor") {
		t.Error("WGSL fragment missing v_fogFactor — fog application missing")
	}
}

// TestGlowPointsGLSLParityContract checks the WebGL output matches SCENE_POINTS_*.
func TestGlowPointsGLSLParityContract(t *testing.T) {
	src, err := os.ReadFile("examples/glow-points.sel")
	if err != nil {
		t.Fatal(err)
	}
	res, err := Compile(src, CompileOptions{Targets: []Target{TargetGLSL}})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	glsl, ok := res.Artifact(TargetGLSL)
	if !ok {
		t.Fatal("missing GLSL artifact")
	}

	// Vertex must declare engine attributes matching SCENE_POINTS_VERTEX_SOURCE.
	for _, attr := range []string{"attribute vec3 a_position", "attribute float a_size", "attribute vec4 a_color"} {
		if !strings.Contains(glsl.Vertex, attr) {
			t.Errorf("GLSL vertex missing %q", attr)
		}
	}
	for _, uni := range []string{"uniform mat4 u_viewMatrix", "uniform mat4 u_projectionMatrix", "uniform mat4 u_modelMatrix"} {
		if !strings.Contains(glsl.Vertex, uni) {
			t.Errorf("GLSL vertex missing %q", uni)
		}
	}

	// Fragment must handle gl_PointSize and point style.
	if !strings.Contains(glsl.Fragment, "v_fogFactor") {
		t.Error("GLSL fragment missing v_fogFactor")
	}
}

// ---- lens scene-sample tests ----

// TestLensWGSLHasFourSceneSamples asserts the gravitational-lens WGSL contains
// at least 4 independent sceneColor sample calls (colorR, colorG, colorB, rawColor).
func TestLensWGSLHasFourSceneSamples(t *testing.T) {
	src, err := os.ReadFile("examples/gravitational-lens.sel")
	if err != nil {
		t.Fatal(err)
	}
	res, err := Compile(src, CompileOptions{Targets: []Target{TargetWGSL}})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	wgsl, _ := res.Artifact(TargetWGSL)

	// Count textureSample calls in the fragment.
	count := strings.Count(wgsl.Source, "textureSample(_sceneColorTex,")
	if count < 4 {
		t.Errorf("WGSL has %d sceneColor samples, want >= 4", count)
	}
}

// TestPostWGSLEngineBindings checks the post emitter declares engine textures
// at the correct fixed slots (group 0, bindings 0-3).
func TestPostWGSLEngineBindings(t *testing.T) {
	src, err := os.ReadFile("examples/gravitational-lens.sel")
	if err != nil {
		t.Fatal(err)
	}
	res, err := Compile(src, CompileOptions{Targets: []Target{TargetWGSL}})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	wgsl, _ := res.Artifact(TargetWGSL)

	required := []string{
		"@group(0) @binding(0) var _sceneColorTex",
		"@group(0) @binding(1) var _sceneColorSamp",
		"@group(0) @binding(2) var _sceneDepthTex",
		"@group(0) @binding(3) var _sceneDepthSamp",
		"@group(0) @binding(4) var<uniform> u",
	}
	for _, b := range required {
		if !strings.Contains(wgsl.Source, b) {
			t.Errorf("WGSL missing %q", b)
		}
	}
}

// TestPostWGSLFullscreenVertex checks the post vertex stage is a fullscreen
// triangle with no attributes (matches the native postfx pattern).
func TestPostWGSLFullscreenVertex(t *testing.T) {
	src, err := os.ReadFile("examples/gravitational-lens.sel")
	if err != nil {
		t.Fatal(err)
	}
	res, err := Compile(src, CompileOptions{Targets: []Target{TargetWGSL}})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	wgsl, _ := res.Artifact(TargetWGSL)

	if !strings.Contains(wgsl.Source, "@vertex fn vertexMain(@builtin(vertex_index) vi") {
		t.Error("WGSL post vertex not a fullscreen triangle (missing vertex_index)")
	}
	// Must NOT have VertexInput struct (no attributes).
	if strings.Contains(wgsl.Source, "struct VertexInput") {
		t.Error("WGSL post vertex has VertexInput struct — post passes have no vertex attributes")
	}
}

// ---- sema error tests ----

func TestPointsRejectsSceneSamplers(t *testing.T) {
	src := []byte(`material Bad kind points {
    surface(pt) -> color {
        return sceneColor(pt.pointUV)
    }
}`)
	_, err := Compile(src, CompileOptions{Targets: []Target{}})
	if err == nil {
		t.Fatal("expected error for sceneColor in points material")
	}
	if !strings.Contains(err.Error(), "only available in post-kind") {
		t.Errorf("error %q does not mention post-kind restriction", err.Error())
	}
}

func TestPostRejectsMeshGeoFields(t *testing.T) {
	src := []byte(`material Bad kind post {
    surface(p) -> color {
        return sceneColor(p.worldNormal.xy)
    }
}`)
	_, err := Compile(src, CompileOptions{Targets: []Target{}})
	if err == nil {
		t.Fatal("expected error for geo.worldNormal in post material")
	}
	var ce *CompileError
	if errors.As(err, &ce) {
		found := false
		for _, d := range ce.Diagnostics {
			if strings.Contains(d.Message, "geo.worldNormal") {
				found = true
			}
		}
		if !found {
			t.Errorf("error %v does not mention geo.worldNormal", err)
		}
	}
}

func TestPointsRejectsMeshGeoFields(t *testing.T) {
	src := []byte(`material Bad kind points {
    surface(pt) -> color {
        return pt.worldNormal
    }
}`)
	_, err := Compile(src, CompileOptions{Targets: []Target{}})
	if err == nil {
		t.Fatal("expected error for geo.worldNormal in points material")
	}
}

func TestPointsRejectsTexture2DParam(t *testing.T) {
	src := []byte(`material Bad kind points {
    param sprite : texture2d
    surface(pt) -> color {
        return pt.color
    }
}`)
	_, err := Compile(src, CompileOptions{Targets: []Target{}})
	if err == nil {
		t.Fatal("expected error for texture2d param in points material")
	}
	var ce *CompileError
	if errors.As(err, &ce) {
		found := false
		for _, d := range ce.Diagnostics {
			if strings.Contains(d.Message, "texture2d params are not supported in points") {
				found = true
			}
		}
		if !found {
			t.Errorf("error %v does not mention texture2d restriction", err)
		}
	}
}

func TestPostRejectsTexture2DParam(t *testing.T) {
	src := []byte(`material Bad kind post {
    param lut : texture2d
    surface(p) -> color {
        return sceneColor(p.uv)
    }
}`)
	_, err := Compile(src, CompileOptions{Targets: []Target{}})
	if err == nil {
		t.Fatal("expected error for texture2d param in post material")
	}
	if !strings.Contains(err.Error(), "texture2d params are not supported in post") {
		t.Errorf("error %q missing post texture2d message", err.Error())
	}
}

func TestMeshMaterialRejectsSceneSamplers(t *testing.T) {
	// A plain mesh material must not be able to call sceneColor.
	src := []byte(`material Bad {
    surface(geo) -> color {
        return sceneColor(geo.uv)
    }
}`)
	_, err := Compile(src, CompileOptions{Targets: []Target{}})
	if err == nil {
		t.Fatal("expected error for sceneColor in mesh material")
	}
	if !strings.Contains(err.Error(), "only available in post-kind") {
		t.Errorf("error %q does not mention post-kind restriction", err.Error())
	}
}

func TestUnknownKindError(t *testing.T) {
	src := []byte(`material Bad kind volumetric {
    surface(p) -> color { return rgb(1, 0, 0) }
}`)
	_, err := Compile(src, CompileOptions{Targets: []Target{}})
	if err == nil {
		t.Fatal("expected error for unknown kind")
	}
	if !strings.Contains(err.Error(), "unknown kind") {
		t.Errorf("error %q does not mention unknown kind", err.Error())
	}
}

// ---- minimal post pass tests ----

func TestPostPassthroughCompiles(t *testing.T) {
	// Identity post pass: just returns sceneColor(uv).
	src := []byte(`material Passthrough kind post {
    surface(p) -> color { return sceneColor(p.uv) }
}`)
	res, err := Compile(src, CompileOptions{})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if res.Layout.Kind != bindings.SurfaceKindPost {
		t.Errorf("kind = %q, want post", res.Layout.Kind)
	}
	wgsl, ok := res.Artifact(TargetWGSL)
	if !ok || !strings.Contains(wgsl.Source, "textureSample(_sceneColorTex") {
		t.Error("WGSL passthrough does not sample sceneColorTex")
	}
}

func TestPointsMinimalCompiles(t *testing.T) {
	// Simplest possible points material: just return the point color.
	src := []byte(`material SimplePoint kind points {
    surface(pt) -> color { return rgb(pt.color.r, pt.color.g, pt.color.b, pt.alpha) }
}`)
	res, err := Compile(src, CompileOptions{})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if res.Layout.Kind != bindings.SurfaceKindPoints {
		t.Errorf("kind = %q, want points", res.Layout.Kind)
	}
	if len(res.Layout.Attributes) != 0 {
		t.Errorf("points layout should have no attributes, got %v", res.Layout.Attributes)
	}
}

// ---- sceneDepth access test ----

func TestSceneDepthInPost(t *testing.T) {
	src := []byte(`material DepthViz kind post {
    surface(p) -> color {
        let d = sceneDepth(p.uv)
        return rgb(d, d, d)
    }
}`)
	res, err := Compile(src, CompileOptions{Targets: []Target{TargetWGSL}})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	wgsl, _ := res.Artifact(TargetWGSL)
	if !strings.Contains(wgsl.Source, "_sceneDepthTex") {
		t.Error("WGSL depth viz does not reference _sceneDepthTex")
	}
}

// ---- HIR sample (for validation/adapter use) ----

func TestHIRPointsMaterial(t *testing.T) {
	m := hir.Material{
		Name: "TestPoints",
		Kind: hir.KindPoints,
		Surface: hir.Func{
			Geo:    "pt",
			Result: hir.Call{Func: "rgb", Args: []hir.Expr{hir.Lit{Value: 1}, hir.Lit{Value: 0}, hir.Lit{Value: 0}}},
		},
	}
	res, err := CompileProgram(hir.Program{Materials: []hir.Material{m}}, CompileOptions{Targets: []Target{}})
	if err != nil {
		t.Fatalf("lower: %v", err)
	}
	if res.Layout.Kind != bindings.SurfaceKindPoints {
		t.Errorf("kind = %q, want points", res.Layout.Kind)
	}
}
