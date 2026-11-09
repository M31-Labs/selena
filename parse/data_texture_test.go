package parse

import (
	"testing"

	"m31labs.dev/selena/hir"
)

func TestParseTextureFormat(t *testing.T) {
	m, err := Material([]byte(`material M {
    param spectrum : texture2d format r16float
    param plain    : texture2d
    param gain     : float = 1.0
    surface(geo) -> color {
        let format = gain
        return rgb(format, format, format)
    }
}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Params) != 3 {
		t.Fatalf("params = %+v", m.Params)
	}
	if p := m.Params[0]; p.Name != "spectrum" || p.Type != hir.Texture2D || p.Format != "r16float" {
		t.Errorf("spectrum = %+v", p)
	}
	if p := m.Params[1]; p.Format != "" {
		t.Errorf("plain texture got format %q", p.Format)
	}
	if p := m.Params[2]; p.Format != "" || p.Default == nil {
		t.Errorf("gain = %+v", p)
	}
}

func TestParseTextureFormatRequiresName(t *testing.T) {
	_, err := Material([]byte(`material M {
    param t : texture2d format
    surface(geo) -> color { return rgb(1.0, 1.0, 1.0) }
}`))
	if err == nil {
		t.Fatal("a format with no name parsed")
	}
}
