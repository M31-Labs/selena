package validate

import (
	prismvalidate "m31labs.dev/prism/validate"
	"m31labs.dev/selena"
	"testing"
)

func TestCubeLevelShadersValidate(t *testing.T) {
	source := []byte(`material Environment {
    param radiance : textureCube
    param roughness : float = 0.5
    surface(geo) -> color {
        return sampleCubeLevel(radiance, normalize(geo.worldNormal), roughness * 6.0).rgb
    }
}`)
	r, err := selena.Compile(source, selena.CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []selena.Target{selena.TargetGLES, selena.TargetGLSL} {
		a, _ := r.Artifact(target)
		t.Run(string(target)+"-vertex", func(t *testing.T) { prismvalidate.Shader(t, "glslangValidator", a.Vertex, ".vert", nil) })
		t.Run(string(target)+"-fragment", func(t *testing.T) { prismvalidate.Shader(t, "glslangValidator", a.Fragment, ".frag", nil) })
	}
	a, _ := r.Artifact(selena.TargetWGSL)
	t.Run("wgsl", func(t *testing.T) { prismvalidate.Shader(t, "naga", a.Source, ".wgsl", nil) })
}
