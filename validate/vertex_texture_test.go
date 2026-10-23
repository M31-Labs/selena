package validate

import (
	"testing"

	prismvalidate "m31labs.dev/prism/validate"
	"m31labs.dev/selena"
)

// Materials whose vertex() stage samples a texture with sampleLevel. The
// second one samples inside control flow and from two textures, with a
// fragment that also samples — the shapes a displaced surface needs.
var vertexTextureSources = map[string]string{
	"height": `material Displace {
    param heightMap : texture2d
    param tint      : texture2d
    param scale     : float = 0.25
    varying height : float
    vertex(geo) -> vec4 {
        let h = sampleLevel(heightMap, geo.uv, 0.0).x
        height = h
        return mvp * vec4f(geo.position + geo.normal * (h * scale), 1.0)
    }
    surface(geo) -> color {
        let c = sample(tint, vec2f(0.5, 0.5))
        return rgb(c.x * geo.height, c.y, c.z)
    }
}`,
	"control-flow": `material Layers {
    param a : texture2d
    param b : texture2d
    param mixAmount : float = 0.5
    varying d : float
    vertex(geo) -> vec4 {
        var h = sampleLevel(a, geo.uv, 1.0).x
        if (mixAmount > 0.25) {
            h = h + sampleLevel(b, geo.uv * 2.0, mixAmount * 4.0).y
        }
        for (var i = 0i; i < 2i; i = i + 1i) {
            h = h + sampleLevel(a, geo.uv + vec2f(0.1, 0.0), 0.0).x * 0.1
        }
        d = h
        return mvp * vec4f(geo.position + vec3f(0.0, h, 0.0), 1.0)
    }
    surface(geo) -> color {
        return rgb(geo.d, geo.d, geo.d)
    }
}`,
}

func TestVertexSampleLevelShadersValidate(t *testing.T) {
	for name, src := range vertexTextureSources {
		r, err := selena.Compile([]byte(src), selena.CompileOptions{})
		if err != nil {
			t.Fatalf("%s: compile: %v", name, err)
		}
		a, _ := r.Artifact(selena.TargetWGSL)
		t.Run(name+"/wgsl", func(t *testing.T) { prismvalidate.Shader(t, "naga", a.Source, ".wgsl", nil) })
		g, _ := r.Artifact(selena.TargetGLSL)
		t.Run(name+"/essl100-vert", func(t *testing.T) { prismvalidate.Shader(t, "glslangValidator", essl100(g.Vertex), ".vert", nil) })
		t.Run(name+"/essl100-frag", func(t *testing.T) { prismvalidate.Shader(t, "glslangValidator", essl100(g.Fragment), ".frag", nil) })
		e, _ := r.Artifact(selena.TargetGLES)
		t.Run(name+"/essl300-vert", func(t *testing.T) { prismvalidate.Shader(t, "glslangValidator", e.Vertex, ".vert", nil) })
		t.Run(name+"/essl300-frag", func(t *testing.T) { prismvalidate.Shader(t, "glslangValidator", e.Fragment, ".frag", nil) })
	}
}
