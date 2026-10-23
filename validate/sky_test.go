package validate

import (
	"testing"

	prismvalidate "m31labs.dev/prism/validate"
	"m31labs.dev/selena"
	"m31labs.dev/selena/materiallib"
)

// The physical sky prelude, called from a fragment with engine inputs, must
// pass the real shader compilers on every target that has one.
func TestPhysicalSkyShadersValidate(t *testing.T) {
	r, err := materiallib.Compile([]byte(`material SkyProbe {
    context {
        skyBetaR : vec4
        skyBetaM : vec4
        skySun   : vec4
        skyG     : float
        cameraPos : vec3
    }
    surface(geo) -> color {
        let d = normalize(geo.worldNormal)
        let folded = vec3f(d.x, abs(d.y), d.z)
        return physicalSky(folded, skyBetaR, skyBetaM, skySun, skyG)
    }
}`), selena.CompileOptions{}, materiallib.Sky)
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
}
