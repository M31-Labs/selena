package gles

import (
	"fmt"
	"strings"

	"m31labs.dev/selena/emit/internal"
	"m31labs.dev/selena/ir"
)

// SurfaceUniformPrefix is prepended to every param uniform in a GLES surface
// hook (bindings.SurfaceHook.GLUniformPrefix).
const SurfaceUniformPrefix = "selena_"

// SurfaceRecords declares the SelenaSurfaceGeo and SelenaSurface records of
// the surface hook ABI (bindings.SurfaceABI) in GLSL ES 3.00.
func SurfaceRecords() string {
	var b strings.Builder
	writeRecord(&b, "SelenaSurfaceGeo", ir.SurfaceGeoFields)
	writeRecord(&b, "SelenaSurface", ir.SurfaceFields)
	return b.String()
}

func writeRecord(b *strings.Builder, name string, fields []ir.SurfaceField) {
	fmt.Fprintf(b, "struct %s {\n", name)
	for _, f := range fields {
		fmt.Fprintf(b, "  %s %s;\n", typeName(f.Type), f.Name)
	}
	b.WriteString("};\n")
}

// emitSurface writes a GLSL ES 3.00 surface hook for splicing into a host
// fragment shader after its precision statement: the ABI records, the param
// uniforms (prefixed with SurfaceUniformPrefix), and
// SelenaSurface selenaSurface(SelenaSurfaceGeo selenaGeo). The sampler2D
// uniforms selenaSurfaceTex<i> are not declared; the host declares them or
// #defines them to its own samplers.
func emitSurface(m ir.Module) string {
	var b strings.Builder
	b.WriteString(SurfaceRecords())
	for _, u := range m.Uniforms {
		fmt.Fprintf(&b, "uniform %s %s%s;\n", typeName(u.Type), SurfaceUniformPrefix, u.Name)
	}
	for _, au := range m.ArrayUniforms {
		fmt.Fprintf(&b, "uniform %s %s%s[%d];\n", typeName(au.ElemType), SurfaceUniformPrefix, au.Name, au.Count)
	}
	fs := internal.SurfaceResolver(internal.NewBare(prismDialect), m, SurfaceUniformPrefix)
	fs.TypedUintLiterals = ir.NeedsTypedUintLiterals(m)
	b.WriteString("SelenaSurface selenaSurface(SelenaSurfaceGeo selenaGeo) {\n")
	internal.EmitStmtList(&b, m.Fragment.Body, fs, "  ", false)
	fmt.Fprintf(&b, "  return SelenaSurface(%s);\n}\n", strings.Join(m.SurfaceOutputs, ", "))
	return b.String()
}

// SurfaceHarness wraps a GLES surface hook in a complete GLSL ES 3.00
// fragment shader so glslangValidator can check it: it declares textures
// sampler2D uniforms and a main that calls the hook.
func SurfaceHarness(hook string, textures int) string {
	var b strings.Builder
	b.WriteString("#version 300 es\nprecision highp float;\nprecision highp int;\n")
	for i := 0; i < textures; i++ {
		fmt.Fprintf(&b, "uniform sampler2D %s;\n", ir.SurfaceTextureName(i))
	}
	b.WriteString("in vec3 vWorldPos;\nin vec3 vWorldNormal;\nin vec2 vUv;\nout vec4 fragColor;\n")
	b.WriteString(hook)
	b.WriteString(`void main() {
  vec3 n = normalize(vWorldNormal);
  SelenaSurface s = selenaSurface(SelenaSurfaceGeo(vWorldPos, n, vUv, vec3(1.0, 0.0, 0.0), vec3(0.0, 0.0, 1.0), vec3(0.0, 0.0, 1.0), vec4(1.0)));
  fragColor = vec4(s.baseColor * s.occlusion + s.emissive + s.normal * s.roughness * s.metallic + vec3(s.clearcoat + s.clearcoatRoughness + s.sheen + s.transmission), s.alpha);
}
`)
	return b.String()
}
