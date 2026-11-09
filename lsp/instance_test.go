package lsp

import (
	"slices"
	"strings"
	"testing"
)

const instanceSample = `material NoteQuad {
    instance rect : vec4
    instance region : int
    varying vTone : vec4
    vertex(geo) -> vec4 {
        let corner = vec2f(float(vertexIndex & 1u), float(vertexIndex >> 1u))
        let p = rect.xy + corner * rect.zw + vec2f(float(instanceIndex), float(region))
        vTone = vec4f(p, 0.0, 1.0)
        return mvp * vec4f(p, 0.0, 1.0)
    }
    surface(geo) -> color { return geo.vTone.rgb }
}
`

func TestInstanceKeywordAndNoDiagnostics(t *testing.T) {
	if !slices.Contains(keywordList, "instance") {
		t.Error("keywordList lacks instance")
	}
	_, diags := open(t, instanceSample)
	if len(diags) != 0 {
		t.Fatalf("diagnostics = %+v", diags)
	}
}

func TestInstanceHoverAndCompletion(t *testing.T) {
	s, _ := open(t, instanceSample)
	uri := "file:///tmp/m.sel"
	h := s.Hover(uri, at(t, instanceSample, "rect.xy", 0, 1))
	if h == nil || !strings.Contains(h.Contents.Value, "(instance) rect : vec4") {
		t.Fatalf("rect hover = %+v", h)
	}
	h = s.Hover(uri, at(t, instanceSample, "instanceIndex)", 0, 3))
	if h == nil || !strings.Contains(h.Contents.Value, "(builtin) instanceIndex : uint") || !strings.Contains(h.Contents.Value, "GLSL ES 1.00") {
		t.Fatalf("instanceIndex hover = %+v", h)
	}
	const typed = "material M {\n    instance rect : vec4\n    vertex() -> vec4 {\n        let a = \n        return mvp * rect\n    }\n    surface(geo) -> color { return rgb(1.0, 1.0, 1.0) }\n}\n"
	s.Update(uri, typed)
	items := labels(s.Completion(uri, at(t, typed, "let a = ", 0, 8)))
	for _, want := range []string{"rect", "vertexIndex", "instanceIndex"} {
		if !slices.Contains(items, want) {
			t.Errorf("vertex scope completion lacks %q", want)
		}
	}
	// The builtins are vertex-only, so surface() does not offer them.
	const frag = "material M {\n    surface(geo) -> color {\n        let a = \n        return rgb(1.0, 1.0, 1.0)\n    }\n}\n"
	s.Update(uri, frag)
	items = labels(s.Completion(uri, at(t, frag, "let a = ", 0, 8)))
	if slices.Contains(items, "instanceIndex") || slices.Contains(items, "vertexIndex") {
		t.Errorf("surface completion offers vertex builtins: %v", items)
	}
}

func TestInstanceDiagnosticHasCodeAndRange(t *testing.T) {
	const src = "material M {\n    instance a : mat4\n    vertex() -> vec4 { return mvp * vec4f(1.0, 1.0, 1.0, 1.0) }\n    surface(geo) -> color { return rgb(1.0, 1.0, 1.0) }\n}\n"
	_, diags := open(t, src)
	if len(diags) == 0 || diags[0].Code != "SEL1003" {
		t.Fatalf("diagnostics = %+v", diags)
	}
}
