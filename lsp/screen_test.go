package lsp

import (
	"slices"
	"strings"
	"testing"
)

const screenSample = `material Meter kind screen {
    render {
        blend additive
        depth off
    }
    param levels : array<vec4, 8>
    surface(screen) -> color {
        let size = screenSize()
        let c = screen.
        return rgb(c.x, size.x, 0.0, 1.0)
    }
}
`

func TestScreenKeywordsAndKind(t *testing.T) {
	for _, kw := range []string{"render", "blend", "depth"} {
		if !slices.Contains(keywordList, kw) {
			t.Errorf("keywordList lacks %q", kw)
		}
	}
	if doc := kindDocs["screen"]; !strings.Contains(doc, "render block") {
		t.Errorf("kindDocs[screen] = %q", doc)
	}
	s, _ := open(t, "material M kind \n")
	items := labels(s.Completion("file:///tmp/m.sel", at(t, "material M kind \n", "kind ", 0, 5)))
	if !slices.Contains(items, "screen") {
		t.Errorf("kind completion lacks screen: %v", items)
	}
}

func TestScreenSampleHasNoDiagnosticsOnceComplete(t *testing.T) {
	src := strings.Replace(screenSample, "let c = screen.\n", "let c = screen.uv\n", 1)
	_, diags := open(t, src)
	if len(diags) != 0 {
		t.Fatalf("diagnostics = %+v", diags)
	}
}

func TestRenderBlockCompletion(t *testing.T) {
	s, _ := open(t, screenSample)
	uri := "file:///tmp/m.sel"
	const typed = "material M kind screen {\n    render {\n        blend \n        depth \n        \n    }\n    surface(s) -> color { return rgb(1.0, 0.0, 0.0, 1.0) }\n}\n"
	s.Update(uri, typed)
	blendWords := labels(s.Completion(uri, at(t, typed, "blend ", 0, 6)))
	if !slices.Equal(blendWords, []string{"opaque", "alpha", "premultiplied", "additive"}) {
		t.Errorf("blend completion = %v", blendWords)
	}
	depthWords := labels(s.Completion(uri, at(t, typed, "depth ", 0, 6)))
	if !slices.Equal(depthWords, []string{"off", "test", "write"}) {
		t.Errorf("depth completion = %v", depthWords)
	}
	keys := labels(s.Completion(uri, at(t, typed, "        \n    }", 0, 8)))
	if !slices.Equal(keys, []string{"blend", "depth"}) {
		t.Errorf("render key completion = %v", keys)
	}
}

func TestRenderWordsOutsideTheBlockDoNotOfferBlendWords(t *testing.T) {
	const src = "material M kind screen {\n    param depth : float\n    surface(s) -> color {\n        let d = depth\n        let e = \n        return rgb(d, 0.0, 0.0, 1.0)\n    }\n}\n"
	s, _ := open(t, src)
	items := labels(s.Completion("file:///tmp/m.sel", at(t, src, "let e = ", 0, 8)))
	if slices.Contains(items, "additive") || slices.Contains(items, "premultiplied") {
		t.Errorf("scope completion offers blend words: %v", items)
	}
}

func TestScreenGeometryAndBuiltinHover(t *testing.T) {
	src := strings.Replace(screenSample, "let c = screen.\n", "let c = screen.uv\n", 1)
	s, _ := open(t, src)
	uri := "file:///tmp/m.sel"
	h := s.Hover(uri, at(t, src, "screenSize()", 0, 2))
	if h == nil || !strings.Contains(h.Contents.Value, "screenSize() -> vec2") {
		t.Fatalf("screenSize hover = %+v", h)
	}
	h = s.Hover(uri, at(t, src, "uv\n", 0, 0))
	if h == nil || !strings.Contains(h.Contents.Value, "screen.uv : vec2") || !strings.Contains(h.Contents.Value, "top left") {
		t.Fatalf("screen.uv hover = %+v", h)
	}
	h = s.Hover(uri, at(t, src, "blend additive", 0, 1))
	if h == nil || !strings.Contains(h.Contents.Value, "opaque, alpha, premultiplied or additive") {
		t.Fatalf("blend hover = %+v", h)
	}
}

func TestScreenGeometryCompletion(t *testing.T) {
	s, _ := open(t, screenSample)
	items := labels(s.Completion("file:///tmp/m.sel", at(t, screenSample, "screen.\n", 0, 7)))
	if !slices.Equal(items, []string{"uv"}) {
		t.Errorf("screen. completion = %v", items)
	}
}

func TestRenderDiagnosticHasCodeAndRange(t *testing.T) {
	const src = "material M kind screen {\n    render { blend glow }\n    surface(s) -> color { return rgb(1.0, 0.0, 0.0, 1.0) }\n}\n"
	_, diags := open(t, src)
	if len(diags) == 0 || diags[0].Code != "SEL1008" {
		t.Fatalf("diagnostics = %+v", diags)
	}
	if want := at(t, src, "glow", 0, 0); diags[0].Range.Start != want {
		t.Errorf("range start = %+v, want %+v", diags[0].Range.Start, want)
	}
}
