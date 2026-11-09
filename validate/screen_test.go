package validate

import (
	"os"
	"testing"

	prismvalidate "m31labs.dev/prism/validate"
	"m31labs.dev/selena"
)

// The screen conformance materials (a 128-strip array meter, and a data
// texture read with load, size and sample) must pass naga and glslangValidator
// on every target that has one. Both tools skip cleanly when not installed.
func TestScreenShadersValidate(t *testing.T) {
	for _, name := range []string{"screen-meter-strip", "screen-data-texture"} {
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile("../testdata/conformance/" + name + ".sel")
			if err != nil {
				t.Fatal(err)
			}
			r, err := selena.Compile(src, selena.CompileOptions{})
			if err != nil {
				t.Fatal(err)
			}
			w, _ := r.Artifact(selena.TargetWGSL)
			t.Run("wgsl", func(t *testing.T) { prismvalidate.Shader(t, "naga", w.Source, ".wgsl", nil) })
			g, _ := r.Artifact(selena.TargetGLSL)
			t.Run("essl100-vert", func(t *testing.T) { prismvalidate.Shader(t, "glslangValidator", essl100(g.Vertex), ".vert", nil) })
			t.Run("essl100-frag", func(t *testing.T) { prismvalidate.Shader(t, "glslangValidator", essl100(g.Fragment), ".frag", nil) })
			e, _ := r.Artifact(selena.TargetGLES)
			t.Run("essl300-vert", func(t *testing.T) { prismvalidate.Shader(t, "glslangValidator", e.Vertex, ".vert", nil) })
			t.Run("essl300-frag", func(t *testing.T) { prismvalidate.Shader(t, "glslangValidator", e.Fragment, ".frag", nil) })
		})
	}
}
