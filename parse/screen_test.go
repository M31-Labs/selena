package parse

import (
	"strings"
	"testing"

	"m31labs.dev/selena/hir"
)

func TestParseScreenKindAndRenderBlock(t *testing.T) {
	m, err := Material([]byte(`material M kind screen {
    render { blend additive  depth write }
    param strips : float = 4.0
    surface(screen) -> color {
        return rgb(screen.uv.x, screen.uv.y, strips, 1.0)
    }
}`))
	if err != nil {
		t.Fatal(err)
	}
	if m.Kind != hir.KindScreen {
		t.Fatalf("kind = %q", m.Kind)
	}
	if m.Surface.Geo != "screen" {
		t.Errorf("surface geo = %q", m.Surface.Geo)
	}
	if m.Render == nil || len(m.Render.Settings) != 2 {
		t.Fatalf("render = %+v", m.Render)
	}
	if s := m.Render.Settings[0]; s.Key != "blend" || s.Value != "additive" || s.ValueSpan.IsZero() {
		t.Errorf("setting 0 = %+v", s)
	}
	if s := m.Render.Settings[1]; s.Key != "depth" || s.Value != "write" {
		t.Errorf("setting 1 = %+v", s)
	}
}

func TestParseRenderBlockIsOptionalAndMayBeEmpty(t *testing.T) {
	for name, block := range map[string]string{"absent": "", "empty": "render { }"} {
		t.Run(name, func(t *testing.T) {
			m, err := Material([]byte("material M kind screen {\n" + block + "\nsurface(s) -> color { return rgb(1.0, 0.0, 0.0, 1.0) }\n}"))
			if err != nil {
				t.Fatal(err)
			}
			if name == "absent" && m.Render != nil {
				t.Errorf("render = %+v, want nil", m.Render)
			}
			if name == "empty" && (m.Render == nil || len(m.Render.Settings) != 0) {
				t.Errorf("render = %+v, want an empty block", m.Render)
			}
		})
	}
}

func TestParseRejectsTwoRenderBlocks(t *testing.T) {
	_, err := Material([]byte(`material M kind screen {
    render { blend alpha }
    render { depth test }
    surface(s) -> color { return rgb(1.0, 0.0, 0.0, 1.0) }
}`))
	if err == nil || !strings.Contains(err.Error(), "more than one render block") {
		t.Fatalf("err = %v", err)
	}
}

// render, blend and depth are not reserved outside a render block: existing
// materials may use them as local names.
func TestRenderWordsStayUsableAsNames(t *testing.T) {
	m, err := Material([]byte(`material M {
    param blend : float = 0.5
    surface(geo) -> color {
        let depth = blend
        let render = depth + 1.0
        return rgb(render, depth, blend)
    }
}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Params) != 1 || m.Params[0].Name != "blend" || m.Render != nil {
		t.Fatalf("material = %+v", m)
	}
}

func TestUnknownKindListsScreen(t *testing.T) {
	_, err := Material([]byte(`material M kind overlay {
    surface(s) -> color { return rgb(1.0, 0.0, 0.0, 1.0) }
}`))
	if err == nil || !strings.Contains(err.Error(), "screen") {
		t.Fatalf("err = %v", err)
	}
}
