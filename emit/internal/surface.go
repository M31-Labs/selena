package internal

import (
	"strings"

	"m31labs.dev/selena/ir"
)

// SurfaceResolver adapts r for the body of a surface hook (ir.KindSurface):
// params read from uniformPrefix + name, geometry reads become members of the
// hook's selenaGeo parameter, and texture i samples ir.SurfaceTextureName(i).
func SurfaceResolver(r Resolver, m ir.Module, uniformPrefix string) Resolver {
	uniforms := NameSet(m.Uniforms)
	for _, au := range m.ArrayUniforms {
		uniforms[au.Name] = true
	}
	geo := map[string]string{}
	for _, v := range m.Varyings {
		geo[v.Name] = "selenaGeo." + strings.TrimPrefix(v.Name, "selenaGeo_")
	}
	r.RefFn = func(name string) (string, bool) {
		if uniforms[name] {
			return uniformPrefix + name, true
		}
		if s, ok := geo[name]; ok {
			return s, true
		}
		return "", false
	}
	r.TextureNames = make(map[string]string, len(m.Textures))
	for i, t := range m.Textures {
		r.TextureNames[t.Name] = ir.SurfaceTextureName(i)
	}
	r.Fragment = true
	return r
}
