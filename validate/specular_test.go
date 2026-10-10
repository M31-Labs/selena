package validate

import (
	prismvalidate "m31labs.dev/prism/validate"
	"m31labs.dev/selena"
	"testing"
)

// Validate actual typed MRT shaders, including coverage-changing early returns
// and texture/derivative use in only the companion, not just source substrings.
func TestSpecularMRTCompiles(t *testing.T) {
	fixtures := map[string]string{
		"coverage": `material M {
  param albedo: texture2d
  param env: textureCube
  surface(g)->vec4 {
   if (g.uv.x<0.1) {discard}
   if (g.uv.y<0.2) {return vec4f(0.4,0.3,0.2,0.25)}
   return vec4f(0.4,0.3,0.2,0.5)
  }
  specular(h)->color {
   let width=fwidth(h.uv.x)
   let glow=sample(albedo,h.uv).rgb + sampleCubeLevel(env,h.worldNormal,0.0).rgb
   if (h.uv.y>0.5) {return glow * width}
   return glow
  }
 }`,
		"zero": `material M {surface(g)->vec4{return vec4f(10.0,9.0,8.0,0.2)}}`,
		"vertex": `material M {
  varying shine: vec3
  state field
  vertex(g)->vec4 {shine=g.normal return mvp*vec4f(g.position,1.0)}
  surface(s)->color{return rgb(0.5,0.5,0.5)}
  specular(q)->color{return q.shine + stateAt(vec2f(0.5,0.5)).rgb}
 }`,
	}
	for name, source := range fixtures {
		t.Run(name, func(t *testing.T) {
			r, err := selena.Compile([]byte(source), selena.CompileOptions{SpecularMRT: true})
			if err != nil {
				t.Fatal(err)
			}
			for _, a := range r.Artifacts {
				t.Run(string(a.Target), func(t *testing.T) {
					switch a.Target {
					case selena.TargetWGSL:
						prismvalidate.Shader(t, "naga", a.SpecularMRT.Source, ".wgsl", nil)
					case selena.TargetGLSL:
						prismvalidate.Shader(t, "glslangValidator", essl100(a.Vertex), ".vert", nil)
						// glslang does not implement GL_EXT_draw_buffers. The actual
					// WebGL1 compile/link and pixel contract is exercised by
					// TestSpecularMRTWebGL1 with SELENA_CHROME set.
					case selena.TargetMetal:
						t.Skip("MSL requires the native Apple compiler; source contract tested separately")
					case selena.TargetGLES:
						prismvalidate.Shader(t, "glslangValidator", a.Vertex, ".vert", nil)
						prismvalidate.Shader(t, "glslangValidator", a.SpecularMRT.Fragment, ".frag", nil)
					}
				})
			}
		})
	}
}
