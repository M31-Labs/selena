package validate

import (
	"os"
	"testing"

	"m31labs.dev/selena"
	"m31labs.dev/selena/internal/shadertest"
)

// The data-texture conformance material (load and size in both stages, three
// float formats) must pass naga and glslangValidator on every target that has
// one. Both tools skip cleanly when they are not installed.
func TestDataTextureShadersValidate(t *testing.T) {
	src, err := os.ReadFile("../testdata/conformance/data-texture.sel")
	if err != nil {
		t.Fatal(err)
	}
	r, err := selena.Compile(src, selena.CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	w, _ := r.Artifact(selena.TargetWGSL)
	t.Run("wgsl", func(t *testing.T) { shadertest.Shader(t, "naga", w.Source, ".wgsl", nil) })
	g, _ := r.Artifact(selena.TargetGLSL)
	t.Run("essl100-vert", func(t *testing.T) { shadertest.Shader(t, "glslangValidator", essl100(g.Vertex), ".vert", nil) })
	t.Run("essl100-frag", func(t *testing.T) { shadertest.Shader(t, "glslangValidator", essl100(g.Fragment), ".frag", nil) })
	e, _ := r.Artifact(selena.TargetGLES)
	t.Run("essl300-vert", func(t *testing.T) { shadertest.Shader(t, "glslangValidator", e.Vertex, ".vert", nil) })
	t.Run("essl300-frag", func(t *testing.T) { shadertest.Shader(t, "glslangValidator", e.Fragment, ".frag", nil) })
}
