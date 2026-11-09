package validate

import (
	"os"
	"testing"

	prismvalidate "m31labs.dev/prism/validate"
	"m31labs.dev/selena"
	"m31labs.dev/selena/materiallib"
)

// The scene lighting example (every light, both cascaded shadow slots and fog
// through the lighting module) must pass the real shader compilers on every
// target that has one.
func TestSceneLightingShadersValidate(t *testing.T) {
	src, err := os.ReadFile("../examples/scene-lit.sel")
	if err != nil {
		t.Fatal(err)
	}
	r, err := materiallib.Compile(src, selena.CompileOptions{SpecularMRT: true}, materiallib.Lighting)
	if err != nil {
		t.Fatal(err)
	}
	w, _ := r.Artifact(selena.TargetWGSL)
	t.Run("wgsl", func(t *testing.T) { prismvalidate.Shader(t, "naga", w.Source, ".wgsl", nil) })
	t.Run("wgsl-specular", func(t *testing.T) { prismvalidate.Shader(t, "naga", w.SpecularMRT.Source, ".wgsl", nil) })
	g, _ := r.Artifact(selena.TargetGLSL)
	t.Run("essl100-vert", func(t *testing.T) { prismvalidate.Shader(t, "glslangValidator", essl100(g.Vertex), ".vert", nil) })
	t.Run("essl100-frag", func(t *testing.T) { prismvalidate.Shader(t, "glslangValidator", essl100(g.Fragment), ".frag", nil) })
	e, _ := r.Artifact(selena.TargetGLES)
	t.Run("essl300-vert", func(t *testing.T) { prismvalidate.Shader(t, "glslangValidator", e.Vertex, ".vert", nil) })
	t.Run("essl300-frag", func(t *testing.T) { prismvalidate.Shader(t, "glslangValidator", e.Fragment, ".frag", nil) })
	t.Run("essl300-specular", func(t *testing.T) {
		prismvalidate.Shader(t, "glslangValidator", e.SpecularMRT.Fragment, ".frag", nil)
	})
}
