package validate

import (
	"os"
	"testing"

	prismvalidate "m31labs.dev/prism/validate"
	"m31labs.dev/selena"
)

// The instancing materials must pass naga and glslangValidator on every target
// that has one. NoteQuad uses an int attribute, instanceIndex and bit
// operators, so it has no GLSL ES 1.00 form; InstanceQuad is float-only and
// has all four. Both tools skip cleanly when not installed.
func TestInstancingShadersValidate(t *testing.T) {
	cases := []struct {
		name    string
		path    string
		essl100 bool
	}{
		{"note-quad", "../testdata/instancing/note-quad.sel", false},
		{"instance-quad", "../testdata/conformance/instance-quad.sel", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src, err := os.ReadFile(tc.path)
			if err != nil {
				t.Fatal(err)
			}
			targets := []selena.Target{selena.TargetWGSL, selena.TargetGLES}
			if tc.essl100 {
				targets = append(targets, selena.TargetGLSL)
			}
			r, err := selena.Compile(src, selena.CompileOptions{Targets: targets})
			if err != nil {
				t.Fatal(err)
			}
			w, _ := r.Artifact(selena.TargetWGSL)
			t.Run("wgsl", func(t *testing.T) { prismvalidate.Shader(t, "naga", w.Source, ".wgsl", nil) })
			e, _ := r.Artifact(selena.TargetGLES)
			t.Run("essl300-vert", func(t *testing.T) { prismvalidate.Shader(t, "glslangValidator", e.Vertex, ".vert", nil) })
			t.Run("essl300-frag", func(t *testing.T) { prismvalidate.Shader(t, "glslangValidator", e.Fragment, ".frag", nil) })
			if tc.essl100 {
				g, _ := r.Artifact(selena.TargetGLSL)
				t.Run("essl100-vert", func(t *testing.T) { prismvalidate.Shader(t, "glslangValidator", essl100(g.Vertex), ".vert", nil) })
				t.Run("essl100-frag", func(t *testing.T) { prismvalidate.Shader(t, "glslangValidator", essl100(g.Fragment), ".frag", nil) })
			}
		})
	}
}

// A uint instance attribute and a uint varying pass through naga and glslang.
func TestUintInstanceAttributeValidates(t *testing.T) {
	src := `material M {
    instance atlas : uint
    instance rect : vec4
    varying vAtlas : uint
    vertex() -> vec4 {
        vAtlas = atlas
        let corner = vec2f(float(vertexIndex & 1u), float(vertexIndex >> 1u))
        return vec4f(rect.xy + corner * rect.zw, float((atlas ^ 5u) | 1u), 1.0)
    }
    surface(geo) -> color { return rgb(float(geo.vAtlas == 3u ? 1u : 0u), 0.0, 0.0) }
}`
	r, err := selena.Compile([]byte(src), selena.CompileOptions{Targets: []selena.Target{selena.TargetWGSL, selena.TargetGLES}})
	if err != nil {
		t.Fatal(err)
	}
	w, _ := r.Artifact(selena.TargetWGSL)
	t.Run("wgsl", func(t *testing.T) { prismvalidate.Shader(t, "naga", w.Source, ".wgsl", nil) })
	e, _ := r.Artifact(selena.TargetGLES)
	t.Run("essl300-vert", func(t *testing.T) { prismvalidate.Shader(t, "glslangValidator", e.Vertex, ".vert", nil) })
	t.Run("essl300-frag", func(t *testing.T) { prismvalidate.Shader(t, "glslangValidator", e.Fragment, ".frag", nil) })
}
