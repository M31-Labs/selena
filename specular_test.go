package selena

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestSpecularMRTIsAdditiveAndCoverageOwnedBySurface(t *testing.T) {
	source := []byte(`fn highlight(x: float) -> color { return rgb(x, x, x) }
 material Coat {
  param gain: float = 0.7
  surface(g) -> vec4 {
   let tint = rgb(0.4, 0.2, 0.1)
   if (g.uv.x < 0.1) { discard }
   if (g.uv.y < 0.2) { return vec4f(tint, 0.25) }
   return vec4f(tint, 0.5)
  }
  specular(h) -> color {
   let tint = highlight(gain)
   if (h.uv.x > 0.5) { return tint * fwidth(h.uv.x) }
   return tint
  }
 }`)
	ordinary, err := Compile(source, CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	mrt, err := Compile(source, CompileOptions{SpecularMRT: true})
	if err != nil {
		t.Fatal(err)
	}
	if ordinary.Layout.SpecularOutput != nil || ordinary.Layout.EntryPoints.FragmentSpecular != "" {
		t.Fatal("ordinary layout advertises MRT")
	}
	if mrt.Module.Specular == nil || mrt.Layout.SpecularOutput == nil || !mrt.Layout.SpecularOutput.Authored {
		t.Fatal("authored companion lost")
	}
	if mrt.Layout.EntryPoints.FragmentSpecular != "fragmentMainSpecular" {
		t.Fatal(mrt.Layout.EntryPoints)
	}
	data, _ := json.Marshal(mrt.Layout.SpecularOutput)
	if string(data) != `{"semantic":"specular","location":1,"type":"vec4","colorSpace":"linear","authored":true,"glExtensions":["WEBGL_draw_buffers"]}` {
		t.Fatalf("contract: %s", data)
	}
	for i, a := range mrt.Artifacts {
		normal := a
		normal.SpecularMRT = nil
		if !reflect.DeepEqual(normal, ordinary.Artifacts[i]) {
			t.Fatalf("%s ordinary artifact changed on opt-in", a.Target)
		}
		if a.SpecularMRT == nil {
			t.Fatalf("%s absent variant", a.Target)
		}
		src := a.SpecularMRT.Source + a.SpecularMRT.Fragment
		for _, part := range []string{"selenaSurface", "selenaSpecular", "color.a", "return"} {
			if !strings.Contains(src, part) {
				t.Fatalf("%s missing %q", a.Target, part)
			}
		}
		if strings.Contains(src, "dot(color") {
			t.Fatal("luminance is not specular")
		}
		switch a.Target {
		case TargetWGSL:
			for _, p := range []string{"@location(0) color", "@location(1) specular", "discard;", "let tint ="} {
				if !strings.Contains(src, p) {
					t.Fatalf("WGSL missing %q", p)
				}
			}
		case TargetGLSL:
			for _, p := range []string{"#extension GL_EXT_draw_buffers : require", "GL_OES_standard_derivatives", "gl_FragData[0] = color", "gl_FragData[1] = vec4(specular, color.a)"} {
				if !strings.Contains(src, p) {
					t.Fatalf("GLSL missing %q", p)
				}
			}
		case TargetGLES:
			if !strings.Contains(src, "layout(location = 1) out vec4 specularColor") {
				t.Fatal(src)
			}
		case TargetMetal:
			if !strings.Contains(src, "float4 specular [[color(1)]]") {
				t.Fatal(src)
			}
		}
	}
}

func TestSpecularMRTWithoutCompanionIsZero(t *testing.T) {
	r, err := Compile([]byte(`material Plain { surface(g)->vec4 { return vec4f(9.0, 8.0, 7.0, 0.2) } }`), CompileOptions{SpecularMRT: true})
	if err != nil {
		t.Fatal(err)
	}
	if r.Layout.SpecularOutput.Authored {
		t.Fatal("zero fallback claims authored specular")
	}
	for _, a := range r.Artifacts {
		src := a.SpecularMRT.Source + a.SpecularMRT.Fragment
		if strings.Contains(src, "selenaSpecular(") || !strings.Contains(src, "specular = ") || !strings.Contains(src, "(0.0)") || !strings.Contains(src, "color.a") {
			t.Fatalf("%s not explicit zero with coverage", a.Target)
		}
	}
}

func TestSpecularRejectsInvalidContracts(t *testing.T) {
	tests := []struct{ name, source, want string }{
		{"declaration", "material M { surface(g)->color{return rgb(1.0,1.0,1.0)} specular(g)->vec4{return vec4f(1.0,1.0,1.0,1.0)} }", "declare -> color"},
		{"duplicate", "material M { surface(g)->color{return rgb(1.0,1.0,1.0)} specular(g)->color{return rgb(0.0,0.0,0.0)} specular(h)->color{return rgb(0.0,0.0,0.0)} }", "duplicate specular"},
		{"post", "material M kind post { surface(g)->color{return rgb(1.0,1.0,1.0)} specular(g)->color{return rgb(0.0,0.0,0.0)} }", "only by mesh"},
		{"scalar", "material M { surface(g)->color{return rgb(1.0,1.0,1.0)} specular(g)->color{return 1.0} }", "must return color/vec3"},
		{"rgba", "material M { surface(g)->color{return rgb(1.0,1.0,1.0)} specular(g)->color{return vec4f(1.0,1.0,1.0,1.0)} }", "must return color/vec3"},
		{"earlyRGBA", "material M { surface(g)->color{return rgb(1.0,1.0,1.0)} specular(g)->color{ if (g.uv.x>0.5){return vec4f(1.0,1.0,1.0,1.0)} return rgb(0.0,0.0,0.0)} }", "must return color/vec3"},
		{"discard", "material M { surface(g)->color{return rgb(1.0,1.0,1.0)} specular(g)->color{discard return rgb(0.0,0.0,0.0)} }", "discard"},
		{"locals", "material M { surface(g)->color{let local=rgb(1.0,1.0,1.0) return local} specular(g)->color{return local} }", "local"},
		{"arrays", "material M { surface(g)->color{var a: array<vec3, 1> a[0]=rgb(1.0,1.0,1.0) return a[0]} specular(g)->color{return a[0]} }", "a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Compile([]byte(tt.source), CompileOptions{SpecularMRT: true})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %v, want %s", err, tt.want)
			}
		})
	}
}

func TestSpecularInterfaceInheritanceAndVertex(t *testing.T) {
	source := []byte(`material Base {
 param gain: float = 0.3
 surface(g)->color { return rgb(0.5, 0.5, 0.5) }
 specular(h)->color { return h.worldNormal * gain }
 }
 material Child extends Base { surface(g)->color { return super.surface(g) } }
 material Override extends Base {
 surface(g)->color { return super.surface(g) }
 specular(h)->color { return rgb(0.0,0.0,0.0) }
 }`)
	r, err := Compile(source, CompileOptions{Material: "Child", SpecularMRT: true})
	if err != nil {
		t.Fatal(err)
	}
	if r.Module.Specular == nil || len(r.Module.Varyings) != 1 || r.Module.Varyings[0].Name != "vWorldNormal" {
		t.Fatalf("companion interface lost: %+v", r.Module.Varyings)
	}
	r, err = Compile(source, CompileOptions{Material: "Override", SpecularMRT: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Module.Varyings) != 0 {
		t.Fatalf("override retained unused parent: %+v", r.Module.Varyings)
	}
	r, err = Compile([]byte(`material Authored {
 varying shine: vec3
 vertex(g)->vec4 { shine = g.normal return mvp * vec4f(g.position, 1.0) }
 surface(s)->color { return rgb(0.5,0.5,0.5) }
 specular(q)->color { return q.shine }
 }`), CompileOptions{SpecularMRT: true})
	if err != nil {
		t.Fatal(err)
	}
	if !r.Module.VertexAuthored || r.Module.Specular == nil {
		t.Fatal("authored vertex companion lost")
	}
}

func TestSpecularRequestLeavesNonMeshAndLegacyJSONUnchanged(t *testing.T) {
	for _, source := range []string{
		`material Plain {surface(g)->color{return rgb(0.3,0.2,0.1)}}`,
		`material Post kind post {surface(g)->color{return rgb(0.3,0.2,0.1)}}`,
	} {
		ordinary, err := Compile([]byte(source), CompileOptions{})
		if err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(ordinary.Artifacts)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "SpecularMRT") {
			t.Fatalf("legacy JSON grew an absent field: %s", data)
		}
		requested, err := Compile([]byte(source), CompileOptions{SpecularMRT: true})
		if err != nil {
			t.Fatal(err)
		}
		if ordinary.Module.Kind == "post" && (!reflect.DeepEqual(ordinary.Artifacts, requested.Artifacts) || !reflect.DeepEqual(ordinary.Layout, requested.Layout)) {
			t.Fatal("non-mesh request changed its artifacts/descriptor")
		}
	}
}

func TestSpecularMRTFusesInlinedCommonMath(t *testing.T) {
	r, err := Compile([]byte(`fn energy(x: vec3)->color {let y=normalize(x) return y*0.5}
 material M {
 surface(g)->color {let n=energy(g.worldNormal) let specular=n*0.2 return specular}
 specular(q)->color {let normalValue=energy(q.worldNormal) let specular=normalValue*0.8 return specular}
 }`), CompileOptions{SpecularMRT: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range r.Artifacts {
		src := a.SpecularMRT.Source + a.SpecularMRT.Fragment
		// One normalize in vertex normal transform and one in the shared authored
		// function, independent of the inliner's differently numbered local names.
		want := 1
		if a.Source != "" {
			want = 2
		}
		if got := strings.Count(src, "normalize("); got != want {
			t.Fatalf("%s normalize count %d want %d", a.Target, got, want)
		}
		if strings.Contains(src, "selenaSurface(") || strings.Contains(src, "selenaSpecular(") {
			t.Fatalf("%s duplicated straight-line stage evaluation", a.Target)
		}
		if !strings.Contains(src, "selenaMRTColor.a") {
			t.Fatalf("%s lost coverage", a.Target)
		}
	}
}
