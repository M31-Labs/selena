package lower

import (
	"errors"
	"strings"
	"testing"

	"m31labs.dev/selena/bindings"
	"m31labs.dev/selena/hir"
	"m31labs.dev/selena/ir"
	"m31labs.dev/selena/parse"
)

const screenBody = `surface(screen) -> color {
        return rgb(screen.uv.x, screen.uv.y, 0.0, 1.0)
    }`

func lowerScreenSrc(t *testing.T, src string) (ir.Module, bindings.Layout, error) {
	t.Helper()
	return lowerTexelSrc(t, src)
}

func wantDiagnostic(t *testing.T, err error, code string, contains ...string) *DiagnosticError {
	t.Helper()
	var d *DiagnosticError
	if !errors.As(err, &d) {
		t.Fatalf("err = %v (%T), want a DiagnosticError", err, err)
	}
	if d.Code != code {
		t.Errorf("code = %s, want %s (%s)", d.Code, code, d.Message)
	}
	for _, c := range contains {
		if !strings.Contains(d.Message, c) {
			t.Errorf("message %q lacks %q", d.Message, c)
		}
	}
	return d
}

func TestScreenDefaultsAreAlphaAndDepthOff(t *testing.T) {
	mod, layout, err := lowerScreenSrc(t, "material M kind screen {\n    "+screenBody+"\n}")
	if err != nil {
		t.Fatal(err)
	}
	if mod.Kind != ir.KindScreen || layout.Kind != bindings.SurfaceKindScreen {
		t.Fatalf("kind = %q / %q", mod.Kind, layout.Kind)
	}
	want := bindings.PipelineState{Blend: "alpha"}
	if layout.Pipeline == nil || *layout.Pipeline != want {
		t.Fatalf("pipeline = %+v, want %+v", layout.Pipeline, want)
	}
	if len(mod.Varyings) != 1 || mod.Varyings[0].Name != "v_uv" || mod.Varyings[0].Type != ir.Vec2 {
		t.Errorf("varyings = %+v", mod.Varyings)
	}
	if len(mod.Uniforms) != 0 || layout.UniformBlock.Size != 0 {
		t.Errorf("a screen material that reads no screenSize() got uniforms: %+v", mod.Uniforms)
	}
}

func TestScreenRenderBlockMapsToPipeline(t *testing.T) {
	for _, tc := range []struct {
		blend, depth string
		want         bindings.PipelineState
	}{
		{"opaque", "off", bindings.PipelineState{Blend: "opaque"}},
		{"alpha", "test", bindings.PipelineState{Blend: "alpha", DepthTest: true}},
		{"premultiplied", "write", bindings.PipelineState{Blend: "premultiplied", DepthTest: true, DepthWrite: true}},
		{"additive", "off", bindings.PipelineState{Blend: "additive"}},
	} {
		t.Run(tc.blend+"-"+tc.depth, func(t *testing.T) {
			_, layout, err := lowerScreenSrc(t, "material M kind screen {\n render { blend "+tc.blend+" depth "+tc.depth+" }\n "+screenBody+"\n}")
			if err != nil {
				t.Fatal(err)
			}
			if layout.Pipeline == nil || *layout.Pipeline != tc.want {
				t.Fatalf("pipeline = %+v, want %+v", layout.Pipeline, tc.want)
			}
		})
	}
}

func TestScreenRenderOrderAndPartialBlocks(t *testing.T) {
	_, layout, err := lowerScreenSrc(t, "material M kind screen {\n render { depth test }\n "+screenBody+"\n}")
	if err != nil {
		t.Fatal(err)
	}
	if got := *layout.Pipeline; got != (bindings.PipelineState{Blend: "alpha", DepthTest: true}) {
		t.Errorf("depth-only block = %+v", got)
	}
	_, layout, err = lowerScreenSrc(t, "material M kind screen {\n render { depth write  blend additive }\n "+screenBody+"\n}")
	if err != nil {
		t.Fatal(err)
	}
	if got := *layout.Pipeline; got != (bindings.PipelineState{Blend: "additive", DepthTest: true, DepthWrite: true}) {
		t.Errorf("reversed block = %+v", got)
	}
}

func TestScreenRenderDiagnostics(t *testing.T) {
	for name, tc := range map[string]struct {
		block string
		want  []string
		at    string
	}{
		"unknown blend": {"render { blend glow }", []string{`unknown blend "glow"`, "opaque, alpha, premultiplied, additive"}, "glow"},
		"unknown depth": {"render { depth always }", []string{`unknown depth "always"`, "off, test, write"}, "always"},
		"unknown key":   {"render { cull back }", []string{`unknown setting "cull"`, "blend or depth"}, "cull"},
		"repeated key":  {"render { blend alpha  blend additive }", []string{`"blend" is set more than once`}, "blend additive"},
	} {
		t.Run(name, func(t *testing.T) {
			src := "material M kind screen {\n    " + tc.block + "\n    " + screenBody + "\n}"
			_, _, err := lowerScreenSrc(t, src)
			d := wantDiagnostic(t, err, CodeInvalidRender, tc.want...)
			word := tc.at
			if i := strings.Index(word, " "); i > 0 && name == "repeated key" {
				word = "blend"
			}
			wantOffset := strings.LastIndex(src, word)
			if name != "repeated key" {
				wantOffset = strings.Index(src, word)
			}
			if d.Span.Start.Offset != wantOffset {
				t.Errorf("span starts at %d, want %d (%q)", d.Span.Start.Offset, wantOffset, word)
			}
		})
	}
}

func TestRenderBlockOnOtherKindsIsRejected(t *testing.T) {
	for _, kind := range []string{"", "kind mesh", "kind post", "kind points"} {
		name := kind
		if name == "" {
			name = "default"
		}
		t.Run(name, func(t *testing.T) {
			_, _, err := lowerScreenSrc(t, "material M "+kind+" {\n    render { blend additive }\n    surface(geo) -> color { return rgb(1.0, 0.0, 0.0, 1.0) }\n}")
			wantDiagnostic(t, err, CodeInvalidRender, "render block is supported only by screen materials", "kind screen")
		})
	}
}

func TestScreenRejectsSceneColor(t *testing.T) {
	for _, call := range []string{"sceneColor(screen.uv)", "sceneDepth(screen.uv).x", "sceneColorLevel(screen.uv, 2.0)", "sceneSize().x"} {
		t.Run(call, func(t *testing.T) {
			_, _, err := lowerScreenSrc(t, "material M kind screen {\n surface(screen) -> color { return rgb("+call+", 0.0, 0.0, 1.0) }\n}")
			wantDiagnostic(t, err, CodeInvalidCall, "only available in post-kind materials")
		})
	}
}

func TestScreenSizeOutsideScreenIsRejected(t *testing.T) {
	for _, kind := range []string{"", "kind post"} {
		_, _, err := lowerScreenSrc(t, "material M "+kind+" {\n surface(geo) -> color { return rgb(screenSize().x, 0.0, 0.0, 1.0) }\n}")
		wantDiagnostic(t, err, CodeInvalidCall, "screenSize is only available in screen-kind materials")
	}
}

func TestScreenSizeBecomesHostUniform(t *testing.T) {
	mod, layout, err := lowerScreenSrc(t, `material M kind screen {
    param k : float = 2.0
    surface(screen) -> color {
        let px = 1.0 / screenSize()
        return rgb(px.x * k, px.y, 0.0, 1.0)
    }
}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(mod.Uniforms) != 2 || mod.Uniforms[1].Name != "screenSize" || mod.Uniforms[1].Type != ir.Vec2 {
		t.Fatalf("uniforms = %+v", mod.Uniforms)
	}
	var f *bindings.Field
	for i := range layout.UniformBlock.Fields {
		if layout.UniformBlock.Fields[i].Name == "screenSize" {
			f = &layout.UniformBlock.Fields[i]
		}
	}
	if f == nil || f.Type != "vec2" || f.Class != "context" || f.Offset%8 != 0 {
		t.Fatalf("screenSize field = %+v", f)
	}
	if len(layout.Context) != 1 || layout.Context[0] != "screenSize" {
		t.Errorf("context = %v", layout.Context)
	}
	for _, d := range layout.UniformBlock.Defaults {
		if d.Name == "screenSize" {
			t.Errorf("screenSize has a default: %+v", d)
		}
	}
}

func TestScreenReservesScreenSizeName(t *testing.T) {
	_, _, err := lowerScreenSrc(t, "material M kind screen {\n param screenSize : vec2\n "+screenBody+"\n}")
	wantDiagnostic(t, err, CodeReservedName, "screenSize")
	_, _, err = lowerScreenSrc(t, "material M kind screen {\n context {\n screenSize : vec2\n }\n "+screenBody+"\n}")
	wantDiagnostic(t, err, CodeReservedName, "screenSize")
	// Other kinds keep the name free.
	if _, _, err := lowerScreenSrc(t, "material M {\n param screenSize : vec2\n surface(geo) -> color { return rgb(screenSize.x, 0.0, 0.0, 1.0) }\n}"); err != nil {
		t.Fatalf("mesh material with a screenSize param: %v", err)
	}
}

func TestScreenParamTypes(t *testing.T) {
	mod, layout, err := lowerScreenSrc(t, `material M kind screen {
    param a : float = 1.0
    param b : vec2
    param c : vec3
    param d : color
    param e : vec4
    param m3 : mat3
    param m4 : mat4
    param n : int
    param levels : array<vec4, 8>
    param lut : texture2d format rgba16float
    surface(screen) -> color {
        let i = n
        let t = load(lut, i, 0i) + sample(lut, screen.uv)
        let p = m4 * vec4f(c, a)
        let q = m3 * c
        return rgb(levels[i].x + t.x + p.x + q.x + b.x + e.x, d.y, 0.0, 1.0)
    }
}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(mod.ArrayUniforms) != 1 || mod.ArrayUniforms[0].Count != 8 {
		t.Errorf("array uniforms = %+v", mod.ArrayUniforms)
	}
	if len(layout.Textures) != 1 || layout.Textures[0].Format != "rgba16float" || layout.Textures[0].SampleMode != "both" {
		t.Errorf("textures = %+v", layout.Textures)
	}
}

func TestScreenRejectsUnsupportedParams(t *testing.T) {
	for name, tc := range map[string]struct{ param, want string }{
		"cube": {"param env : textureCube", "textureCube params are not supported in screen materials"},
		"sun":  {"param light : Sun", "Sun record params are not supported in screen materials"},
	} {
		t.Run(name, func(t *testing.T) {
			_, _, err := lowerScreenSrc(t, "material M kind screen {\n "+tc.param+"\n "+screenBody+"\n}")
			wantDiagnostic(t, err, CodeUnsupportedType, tc.want)
		})
	}
}

func TestScreenRejectsVertexStage(t *testing.T) {
	_, _, err := lowerScreenSrc(t, `material M kind screen {
    vertex(geo) -> vec4 { return vec4f(0.0, 0.0, 0.0, 1.0) }
    `+screenBody+`
}`)
	wantDiagnostic(t, err, CodeUnsupportedFeat, "vertex hooks are not supported in screen materials")
}

func TestScreenSampleRejectsFloat32Format(t *testing.T) {
	_, _, err := lowerScreenSrc(t, `material M kind screen {
    param data : texture2d format r32float
    surface(screen) -> color { return rgb(sample(data, screen.uv).r, 0.0, 0.0, 1.0) }
}`)
	wantDiagnostic(t, err, CodeInvalidCall, "float32-filterable")
}

func TestScreenExtendsInheritsRenderBlock(t *testing.T) {
	program := `material Base kind screen {
    render { blend additive  depth test }
    param k : float = 1.0
    surface(screen) -> color { return rgb(k, 0.0, 0.0, 1.0) }
}
material Child extends Base kind screen {
    surface(screen) -> color { return super.surface(screen) * 0.5 }
}
material Override extends Base kind screen {
    render { blend opaque }
    surface(screen) -> color { return super.surface(screen) }
}`
	for i, want := range []bindings.PipelineState{
		{Blend: "additive", DepthTest: true},
		{Blend: "additive", DepthTest: true},
		{Blend: "opaque"},
	} {
		if i == 0 {
			continue
		}
		t.Run([]string{"base", "child", "override"}[i], func(t *testing.T) {
			p := mustProgram(t, program)
			_, layout, err := LowerProgram(p, i)
			if err != nil {
				t.Fatal(err)
			}
			if layout.Pipeline == nil || *layout.Pipeline != want {
				t.Fatalf("pipeline = %+v, want %+v", layout.Pipeline, want)
			}
		})
	}
}

func TestGeometryFieldsForScreen(t *testing.T) {
	fields := GeometryFields("screen", false)
	if len(fields) != 1 || fields["uv"] != ir.Vec2 {
		t.Fatalf("fields = %v", fields)
	}
}

func TestBuiltinsIncludeScreenSize(t *testing.T) {
	for _, n := range Builtins() {
		if n == "screenSize" {
			return
		}
	}
	t.Fatal("Builtins lacks screenSize")
}

func mustProgram(t *testing.T, src string) hir.Program {
	t.Helper()
	p, err := parse.Program([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestReservesMetalPaddingPrefix(t *testing.T) {
	for _, name := range []string{"_selenaPad0", "_selenaPadX"} {
		_, _, err := lowerScreenSrc(t, "material M kind screen {\n param "+name+" : float\n "+screenBody+"\n}")
		wantDiagnostic(t, err, CodeReservedName, name)
	}
	if _, ok := reservedBackendName("selenaPad"); ok {
		t.Error("selenaPad must stay a legal name")
	}
}
