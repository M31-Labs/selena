package selena

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// screenBlendCases pairs each blend word with a depth word, so the descriptor
// goldens cover every blend value and every depth value.
var screenBlendCases = []struct{ blend, depth string }{
	{"opaque", "off"},
	{"alpha", "test"},
	{"premultiplied", "write"},
	{"additive", "off"},
}

func screenSource(blend, depth string) string {
	return `material Overlay kind screen {
    render { blend ` + blend + `  depth ` + depth + ` }
    param tint : color = rgb(0.2, 0.8, 0.4)
    surface(screen) -> color {
        let px = 1.0 / screenSize()
        return rgb(tint.r, tint.g, tint.b, screen.uv.x + px.y)
    }
}`
}

// The descriptor of a screen material carries kind and pipeline. One golden
// per blend value keeps the contract a host reads under test.
func TestScreenDescriptorGoldens(t *testing.T) {
	for _, tc := range screenBlendCases {
		t.Run(tc.blend, func(t *testing.T) {
			r, err := Compile([]byte(screenSource(tc.blend, tc.depth)), CompileOptions{})
			if err != nil {
				t.Fatal(err)
			}
			js, err := r.Layout.JSON()
			if err != nil {
				t.Fatal(err)
			}
			assertGolden(t, filepath.Join("testdata", "screen", "descriptor-"+tc.blend+".json"), js)
		})
	}
}

func TestScreenDescriptorFields(t *testing.T) {
	for _, tc := range screenBlendCases {
		r, err := Compile([]byte(screenSource(tc.blend, tc.depth)), CompileOptions{})
		if err != nil {
			t.Fatal(err)
		}
		js, _ := r.Layout.JSON()
		for _, w := range []string{`"kind": "screen"`, `"blend": "` + tc.blend + `"`, `"depthTest"`, `"depthWrite"`} {
			if !strings.Contains(js, w) {
				t.Errorf("%s descriptor lacks %s:\n%s", tc.blend, w, js)
			}
		}
	}
}

// Descriptors of every other kind stay free of the new fields.
func TestOtherKindsHaveNoPipelineField(t *testing.T) {
	for name, src := range map[string]string{
		"mesh": `material M { surface(geo) -> color { return rgb(1.0, 0.0, 0.0) } }`,
		"post": `material M kind post { surface(p) -> color { return sceneColor(p.uv) } }`,
	} {
		r, err := Compile([]byte(src), CompileOptions{})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		js, _ := r.Layout.JSON()
		if strings.Contains(js, "pipeline") || strings.Contains(js, "depthTest") {
			t.Errorf("%s descriptor mentions the screen pipeline:\n%s", name, js)
		}
	}
}

// Every target reads uv with the origin at the top left: the vertex stage maps
// clip y = +1 to v = 0.
func TestScreenEmitsPerTarget(t *testing.T) {
	r, err := Compile([]byte(screenSource("additive", "off")), CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		target Target
		want   []string
	}{
		{TargetWGSL, []string{
			"@vertex fn vertexMain(@builtin(vertex_index) vi : u32)",
			"vec2<f32>(0.0, 1.0),", // clip (-1,-1) is the bottom left: v = 1
			"var<uniform> u : Uniforms;",
			"screenSize : vec2<f32>,",
			"(1.0 / u.screenSize)",
		}},
		{TargetMetal, []string{
			"vertex ScreenOut vertexMain(uint vid [[vertex_id]])",
			"const float2 uvs[3] = { {0,1},{2,1},{0,-1} };",
			"constant Uniforms& u [[buffer(0)]]",
			"(1.0 / u.screenSize)",
		}},
		{TargetGLES, []string{
			"#version 300 es",
			"v_uv = uvs[gl_VertexID];",
			"vec2(0.0, 1.0), vec2(2.0, 1.0), vec2(0.0, -1.0)",
			"uniform vec2 screenSize;",
			"(1.0 / screenSize)",
		}},
		{TargetGLSL, []string{
			"attribute vec2 a_position;",
			"v_uv = vec2(a_position.x * 0.5 + 0.5, 0.5 - a_position.y * 0.5);",
			"precision highp float;",
			"uniform vec2 screenSize;",
			"(1.0 / screenSize)",
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
			for _, banned := range []string{"_sceneColor", "sceneColor", "_sceneSize"} {
				if strings.Contains(src, banned) {
					t.Errorf("a screen pass mentions %q:\n%s", banned, src)
				}
			}
		})
	}
	if len(r.Layout.Requires.GLExtensions) != 0 || r.Layout.Requires.GLSceneSizeUniform != "" {
		t.Errorf("requires = %+v", r.Layout.Requires)
	}
}

// A screen material with no uniforms and no textures emits no uniform block.
func TestScreenWithoutParamsHasNoUniformBlock(t *testing.T) {
	r, err := Compile([]byte(`material Plain kind screen {
    surface(screen) -> color { return rgb(screen.uv.x, screen.uv.y, 0.0) }
}`), CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	w, _ := r.Artifact(TargetWGSL)
	if strings.Contains(w.Source, "Uniforms") {
		t.Errorf("WGSL declares a uniform block:\n%s", w.Source)
	}
	m, _ := r.Artifact(TargetMetal)
	if strings.Contains(m.Source, "Uniforms") {
		t.Errorf("Metal declares a uniform block:\n%s", m.Source)
	}
	// A rgb() result has alpha 1.0.
	if !strings.Contains(w.Source, "1.0)") {
		t.Errorf("a color result has no alpha:\n%s", w.Source)
	}
	if r.Layout.UniformBlock.Size != 0 {
		t.Errorf("uniform block size = %d", r.Layout.UniformBlock.Size)
	}
}

// Compile reports a bad render word with its code and the position of the word.
func TestScreenCompileDiagnostics(t *testing.T) {
	for name, tc := range map[string]struct{ src, code, at string }{
		"unknown blend":  {"material M kind screen {\n    render { blend glow }\n    surface(s) -> color { return rgb(1.0, 0.0, 0.0, 1.0) }\n}", "SEL1008", "glow"},
		"unknown depth":  {"material M kind screen {\n    render { depth sometimes }\n    surface(s) -> color { return rgb(1.0, 0.0, 0.0, 1.0) }\n}", "SEL1008", "sometimes"},
		"scene color":    {"material M kind screen {\n    surface(s) -> color { return sceneColor(s.uv) }\n}", "SEL2003", "sceneColor"},
		"render on mesh": {"material M {\n    render { blend alpha }\n    surface(g) -> color { return rgb(1.0, 0.0, 0.0) }\n}", "SEL1008", "render"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Compile([]byte(tc.src), CompileOptions{})
			var ce *CompileError
			if !errors.As(err, &ce) || len(ce.Diagnostics) == 0 {
				t.Fatalf("err = %v, want a CompileError", err)
			}
			d := ce.Diagnostics[0]
			if d.Code != tc.code {
				t.Errorf("code = %s, want %s (%s)", d.Code, tc.code, d.Message)
			}
			if got, want := d.Range.Start.Offset, strings.Index(tc.src, tc.at); got != want {
				t.Errorf("offset = %d, want %d (%q)", got, want, tc.at)
			}
			if d.Hint == "" && tc.code == "SEL1008" {
				t.Errorf("no hint for %s", tc.code)
			}
		})
	}
}

// The conformance materials describe what a host needs to bind.
func TestScreenMeterStripDescriptor(t *testing.T) {
	src, err := os.ReadFile("testdata/conformance/screen-meter-strip.sel")
	if err != nil {
		t.Fatal(err)
	}
	r, err := Compile(src, CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	l := r.Layout
	if l.Pipeline == nil || l.Pipeline.Blend != "additive" || l.Pipeline.DepthTest || l.Pipeline.DepthWrite {
		t.Errorf("pipeline = %+v", l.Pipeline)
	}
	var arrays int
	for _, f := range l.UniformBlock.Fields {
		if f.Count == 128 && f.Stride == 16 && f.Type == "vec4" {
			arrays++
		}
	}
	if arrays != 2 {
		t.Errorf("got %d vec4[128] fields, want 2: %+v", arrays, l.UniformBlock.Fields)
	}
	if l.WGSL.Group != 0 || l.WGSL.Binding != 0 || l.Metal.Buffer != 0 {
		t.Errorf("uniform block binding = wgsl %+v metal %+v", l.WGSL, l.Metal)
	}
}
