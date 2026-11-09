package validate

import (
	"testing"

	prismvalidate "m31labs.dev/prism/validate"
	"m31labs.dev/selena"
	"m31labs.dev/selena/emit/gles"
	"m31labs.dev/selena/emit/wgsl"
)

const surfaceSource = `material Rock {
    param albedo : texture2d
    param detail : texture2d
    param tint : color = rgb(0.9, 0.85, 0.8)
    param tiles : array<vec4, 2>
    context { time : float }

    surface(geo) -> Surface {
        let c = sample(albedo, geo.uv).rgb * tint * sample(detail, geo.uv * tiles[0i].xy).r
        var s = pbr(c, 0.8, 0.0)
        if (sin(time) > 0.5) {
            s = withRoughness(s, s.roughness * 0.3)
        }
        let moss = withEmissive(pbr(rgb(0.2, 0.4, 0.1), 0.9, 0.0), geo.color.rgb * 0.0)
        let coat = withClearcoat(withSheen(withTransmission(withAlpha(moss, 0.5), 0.0), 0.2), 1.0, 0.1)
        return withNormal(mixSurface(s, coat, clamp(geo.worldNormal.y, 0.0, 1.0)), normalize(geo.worldNormal + geo.tangent * 0.1))
    }
}`

// Surface hooks are fragments, not modules. The harnesses wrap them the way a
// host does (declare the textures, call the hook from an entry point) so the
// real compilers check every emitted statement.
func TestSurfaceHooksValidate(t *testing.T) {
	r, err := selena.Compile([]byte(surfaceSource), selena.CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	w, _ := r.Artifact(selena.TargetWGSL)
	t.Run("wgsl", func(t *testing.T) {
		prismvalidate.Shader(t, "naga", wgsl.SurfaceHarness(w.Source, len(r.Layout.Textures)), ".wgsl", nil)
	})
	g, _ := r.Artifact(selena.TargetGLES)
	t.Run("essl300-frag", func(t *testing.T) {
		prismvalidate.Shader(t, "glslangValidator", gles.SurfaceHarness(g.Fragment, len(r.Layout.Textures)), ".frag", nil)
	})
}
