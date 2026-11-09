package selena

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"m31labs.dev/selena/bindings"
)

func readInstancing(t *testing.T, path string) []byte {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return src
}

func compileNoteQuad(t *testing.T) Result {
	t.Helper()
	// GLSL ES 1.00 has no form of this material, so ask for the other three.
	r, err := Compile(readInstancing(t, "testdata/instancing/note-quad.sel"), CompileOptions{
		Targets: []Target{TargetWGSL, TargetMetal, TargetGLES},
	})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestNoteQuadEmitsPerTarget(t *testing.T) {
	r := compileNoteQuad(t)
	cases := []struct {
		target Target
		want   []string
	}{
		{TargetWGSL, []string{
			"@location(0) rect : vec4<f32>,",
			"@location(1) tone : vec4<f32>,",
			"@location(2) region : i32,",
			"@location(1) @interpolate(flat) vRegion : i32,",
			"@builtin(vertex_index) vertexIndex : u32, @builtin(instance_index) instanceIndex : u32, in : VertexInput",
			"f32((vertexIndex & 1u))",
			"f32((vertexIndex >> 1u))",
			"in.rect.xy",
			"out.vRegion = in.region;",
			"f32((in.vRegion & 3i))",
		}},
		{TargetGLES, []string{
			"in vec4 rect;",
			"in vec4 tone;",
			"in int region;",
			"flat out int vRegion;",
			"flat in int vRegion;",
			"uint instanceIndex = uint(gl_InstanceID);",
			"(vertexIndex & 1u)",
			"(vertexIndex >> 1u)",
			"(instanceIndex & 3u)",
		}},
		{TargetMetal, []string{
			"float4 rect [[attribute(0)]];",
			"float4 tone [[attribute(1)]];",
			"int region [[attribute(2)]];",
			"int vRegion [[flat]];",
			"uint instanceIndex [[instance_id]]",
			"(vertexIndex & 1u)",
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
}

// Instance locations follow the vertex attributes: with geo.position and geo.uv
// read in vertex(), instance attributes start at location 2 on every target.
func TestInstanceLocationsFollowVertexAttributes(t *testing.T) {
	src := `material M {
    instance offset : vec3
    instance weight : float
    vertex(geo) -> vec4 {
        return mvp * vec4f(geo.position + offset * weight, 1.0 + geo.uv.x)
    }
    surface(geo) -> color { return rgb(1.0, 1.0, 1.0) }
}`
	r, err := Compile([]byte(src), CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Layout.Attributes) != 2 {
		t.Fatalf("attributes = %+v", r.Layout.Attributes)
	}
	want := []bindings.InstanceAttribute{
		{Name: "offset", Location: 2, Type: "vec3", Offset: 0},
		{Name: "weight", Location: 3, Type: "float", Offset: 12},
	}
	if len(r.Layout.InstanceAttributes) != 2 || r.Layout.InstanceAttributes[0] != want[0] || r.Layout.InstanceAttributes[1] != want[1] {
		t.Fatalf("instance attributes = %+v, want %+v", r.Layout.InstanceAttributes, want)
	}
	if r.Layout.InstanceStride != 16 {
		t.Errorf("stride = %d, want 16", r.Layout.InstanceStride)
	}
	w, _ := r.Artifact(TargetWGSL)
	for _, s := range []string{"@location(0) position : vec3<f32>", "@location(1) uv : vec2<f32>", "@location(2) offset : vec3<f32>", "@location(3) weight : f32"} {
		if !strings.Contains(w.Source, s) {
			t.Errorf("wgsl lacks %q:\n%s", s, w.Source)
		}
	}
	m, _ := r.Artifact(TargetMetal)
	for _, s := range []string{"float3 offset [[attribute(2)]]", "float weight [[attribute(3)]]"} {
		if !strings.Contains(m.Source, s) {
			t.Errorf("metal lacks %q:\n%s", s, m.Source)
		}
	}
}

func TestNoteQuadDescriptorGolden(t *testing.T) {
	r := compileNoteQuad(t)
	js, err := r.Layout.JSON()
	if err != nil {
		t.Fatal(err)
	}
	assertGolden(t, filepath.Join("testdata", "instancing", "golden", "note-quad.descriptor.json"), js)
	// 16 + 16 + 4: rect, tone, region, packed with no padding.
	if r.Layout.InstanceStride != 36 {
		t.Errorf("stride = %d, want 36", r.Layout.InstanceStride)
	}
	wantOffsets := map[string]int{"rect": 0, "tone": 16, "region": 32}
	for _, a := range r.Layout.InstanceAttributes {
		if a.Offset != wantOffsets[a.Name] {
			t.Errorf("%s offset = %d, want %d", a.Name, a.Offset, wantOffsets[a.Name])
		}
	}
	if got := r.Layout.Requires.GLExtensions; len(got) != 1 || got[0] != "ANGLE_instanced_arrays" {
		t.Errorf("requires.glExtensions = %v", got)
	}
}

func TestNoteQuadGoldens(t *testing.T) {
	r := compileNoteQuad(t)
	for name, got := range goldenOutputs(r) {
		assertGolden(t, filepath.Join("testdata", "instancing", "golden", "note-quad."+name), got)
	}
}

// Without a target list, a material GLSL ES 1.00 cannot express is compiled for
// the other three targets. Asking for glsl by name says why it cannot be built.
func TestNoteQuadSkipsGLSLByDefaultAndRejectsItByName(t *testing.T) {
	src := readInstancing(t, "testdata/instancing/note-quad.sel")
	r, err := Compile(src, CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := r.Artifact(TargetGLSL); ok {
		t.Error("default targets included glsl")
	}
	for _, target := range []Target{TargetWGSL, TargetMetal, TargetGLES} {
		if _, ok := r.Artifact(target); !ok {
			t.Errorf("default targets lack %s", target)
		}
	}
	_, err = Compile(src, CompileOptions{Targets: []Target{TargetGLSL}})
	if err == nil {
		t.Fatal("glsl compiled a material with instanceIndex, an int attribute and bit operators")
	}
	for _, w := range []string{"GLSL ES 1.00", "instanceIndex", "instance attribute region", "bit operators"} {
		if !strings.Contains(err.Error(), w) {
			t.Errorf("error lacks %q: %v", w, err)
		}
	}
}

// Each unsupported feature is rejected on its own for GLSL ES 1.00.
func TestGLSLRejectsEachInstancingFeature(t *testing.T) {
	cases := map[string]struct{ body, want string }{
		"instanceIndex":  {"instance a : vec4\n vertex() -> vec4 { return a * float(instanceIndex) }", "gl_InstanceID"},
		"int attribute":  {"instance a : int\n vertex() -> vec4 { return vec4f(float(a), 0.0, 0.0, 1.0) }", "no integer vertex attributes"},
		"uint attribute": {"instance a : uint\n vertex() -> vec4 { return vec4f(float(a), 0.0, 0.0, 1.0) }", "no integer vertex attributes"},
		"bit operator":   {"instance a : vec4\n vertex() -> vec4 { return a * float(vertexIndex & 1u) }", "bit operators"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			src := "material M {\n " + tc.body + "\n surface(geo) -> color { return rgb(1.0, 1.0, 1.0) }\n}"
			_, err := Compile([]byte(src), CompileOptions{Targets: []Target{TargetGLSL}})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to mention %q", err, tc.want)
			}
			// WGSL, GLES and Metal accept it.
			if _, err := Compile([]byte(src), CompileOptions{Targets: []Target{TargetWGSL, TargetGLES, TargetMetal}}); err != nil {
				t.Fatalf("other targets: %v", err)
			}
		})
	}
}

// A bit operator in a fragment stage (no instancing involved) is also rejected
// for GLSL ES 1.00.
func TestGLSLRejectsBitOperatorInSurface(t *testing.T) {
	src := `material M {
    surface(geo) -> color {
        let n = int(geo.uv.x * 8.0)
        return rgb(float(n & 3i), 0.0, 0.0)
    }
}`
	if _, err := Compile([]byte(src), CompileOptions{Targets: []Target{TargetGLSL}}); err == nil || !strings.Contains(err.Error(), "bit operators") {
		t.Fatalf("err = %v", err)
	}
	if _, err := Compile([]byte(src), CompileOptions{Targets: []Target{TargetGLES, TargetWGSL, TargetMetal}}); err != nil {
		t.Fatal(err)
	}
}

// The float-only conformance material builds for all four targets and records
// ANGLE_instanced_arrays for the GLSL ES 1.00 form.
func TestInstanceQuadFloatOnlyBuildsOnGLSL(t *testing.T) {
	r, err := Compile(readInstancing(t, "testdata/conformance/instance-quad.sel"), CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	g, ok := r.Artifact(TargetGLSL)
	if !ok {
		t.Fatal("no glsl artifact")
	}
	for _, w := range []string{"attribute vec4 rect;", "attribute vec4 tone;", "attribute float spin;", "attribute float a_vertexIndex;"} {
		if !strings.Contains(g.Vertex, w) {
			t.Errorf("glsl vertex lacks %q:\n%s", w, g.Vertex)
		}
	}
	if got := r.Layout.Requires.GLExtensions; len(got) != 1 || got[0] != "ANGLE_instanced_arrays" {
		t.Errorf("requires.glExtensions = %v", got)
	}
	if r.Layout.InstanceStride != 36 {
		t.Errorf("stride = %d, want 36", r.Layout.InstanceStride)
	}
}

// A uint instance attribute maps to u32 / uint, and a uint literal keeps its
// suffix in GLSL ES 3.00 (which has no implicit int to uint conversion).
func TestUintInstanceAttribute(t *testing.T) {
	src := `material M {
    instance atlas : uint
    varying vAtlas : uint
    vertex() -> vec4 {
        vAtlas = atlas
        return vec4f(float(atlas >> 2u), float(atlas == 3u ? 1.0 : 0.0), 0.0, 1.0)
    }
    surface(geo) -> color { return rgb(float(geo.vAtlas), 0.0, 0.0) }
}`
	r, err := Compile([]byte(src), CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := r.Artifact(TargetGLSL); ok {
		t.Error("a uint attribute still got a glsl artifact by default")
	}
	want := map[Target][]string{
		TargetWGSL:  {"@location(0) atlas : u32,", "(in.atlas >> 2u)", "(in.atlas == 3u)", "@interpolate(flat) vAtlas : u32"},
		TargetGLES:  {"in uint atlas;", "(atlas >> 2u)", "(atlas == 3u)", "flat out uint vAtlas;", "flat in uint vAtlas;"},
		TargetMetal: {"uint atlas [[attribute(0)]];", "(in.atlas >> 2u)", "uint vAtlas [[flat]];"},
	}
	for target, subs := range want {
		a, _ := r.Artifact(target)
		out := a.Source + a.Vertex + a.Fragment
		for _, w := range subs {
			if !strings.Contains(out, w) {
				t.Errorf("%s lacks %q:\n%s", target, w, out)
			}
		}
	}
	if r.Layout.InstanceStride != 4 || r.Layout.InstanceAttributes[0].Type != "uint" {
		t.Errorf("layout = %+v stride %d", r.Layout.InstanceAttributes, r.Layout.InstanceStride)
	}
}

func TestNoInstancingLeavesDescriptorAndShadersAlone(t *testing.T) {
	src := readInstancing(t, "testdata/conformance/vertex.sel")
	r, err := Compile(src, CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	js, _ := r.Layout.JSON()
	if strings.Contains(js, "instance") {
		t.Errorf("a material without instance attributes mentions instancing:\n%s", js)
	}
	g, _ := r.Artifact(TargetGLES)
	if strings.Contains(g.Vertex+g.Fragment, "flat") || strings.Contains(g.Fragment, "precision highp int") {
		t.Errorf("gles output changed for a material without instancing:\n%s", g.Vertex+g.Fragment)
	}
}

// A host that binds engine per-instance data at locations 3 to 8 puts the
// authored attributes at 9 and up. The WGSL @location and the descriptor agree.
func TestFirstInstanceLocationRebasesEveryTarget(t *testing.T) {
	src := readInstancing(t, "testdata/instancing/note-quad.sel")
	r, err := Compile(src, CompileOptions{FirstInstanceLocation: 9})
	if err != nil {
		t.Fatal(err)
	}
	for i, a := range r.Layout.InstanceAttributes {
		if a.Location != 9+i {
			t.Errorf("%s location = %d, want %d", a.Name, a.Location, 9+i)
		}
	}
	if r.Layout.InstanceStride != 36 || r.Layout.InstanceAttributes[2].Offset != 32 {
		t.Errorf("the record layout changed: %+v stride %d", r.Layout.InstanceAttributes, r.Layout.InstanceStride)
	}
	w, _ := r.Artifact(TargetWGSL)
	for _, s := range []string{"@location(9) rect : vec4<f32>,", "@location(10) tone : vec4<f32>,", "@location(11) region : i32,"} {
		if !strings.Contains(w.Source, s) {
			t.Errorf("wgsl lacks %q:\n%s", s, w.Source)
		}
	}
	m, _ := r.Artifact(TargetMetal)
	for _, s := range []string{"float4 rect [[attribute(9)]];", "int region [[attribute(11)]];"} {
		if !strings.Contains(m.Source, s) {
			t.Errorf("metal lacks %q", s)
		}
	}
	// Vertex attributes keep their own locations.
	withGeo := `material M {
    instance a : vec4
    vertex(geo) -> vec4 { return mvp * vec4f(geo.position, 1.0) + a }
    surface(geo) -> color { return rgb(1.0, 1.0, 1.0) }
}`
	g, err := Compile([]byte(withGeo), CompileOptions{FirstInstanceLocation: 3})
	if err != nil {
		t.Fatal(err)
	}
	gw, _ := g.Artifact(TargetWGSL)
	if !strings.Contains(gw.Source, "@location(0) position") || !strings.Contains(gw.Source, "@location(3) a : vec4<f32>") {
		t.Errorf("wgsl:\n%s", gw.Source)
	}
}

func TestFirstInstanceLocationRules(t *testing.T) {
	twoAttrs := `material M {
    instance a : vec4
    instance b : vec4
    vertex(geo) -> vec4 { return mvp * vec4f(geo.position + geo.normal, 1.0) + a + b }
    surface(geo) -> color { return rgb(1.0, 1.0, 1.0) }
}`
	_, err := Compile([]byte(twoAttrs), CompileOptions{FirstInstanceLocation: 1})
	if err == nil || !strings.Contains(err.Error(), "location 1") || !strings.Contains(err.Error(), "2 vertex attributes") {
		t.Errorf("base below the vertex attribute count: %v", err)
	}
	if _, err := Compile([]byte(twoAttrs), CompileOptions{FirstInstanceLocation: 14}); err != nil {
		t.Errorf("base 14 with 2 attributes ends at 15 and is legal: %v", err)
	}
	_, err = Compile([]byte(twoAttrs), CompileOptions{FirstInstanceLocation: 15})
	if err == nil || !strings.Contains(err.Error(), "location 16") || !strings.Contains(err.Error(), "below 16") {
		t.Errorf("overflow past 16: %v", err)
	}
	// Without instance attributes the option changes nothing.
	plain := readInstancing(t, "testdata/conformance/vertex.sel")
	a, err := Compile(plain, CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	b, err := Compile(plain, CompileOptions{FirstInstanceLocation: 7})
	if err != nil {
		t.Fatal(err)
	}
	wa, _ := a.Artifact(TargetWGSL)
	wb, _ := b.Artifact(TargetWGSL)
	if wa.Source != wb.Source {
		t.Error("the option changed a material without instance attributes")
	}
}
