package wgsl

import (
	"fmt"
	"strings"

	"m31labs.dev/selena/emit/internal"
	"m31labs.dev/selena/ir"
)

// SurfaceParamsGroup and SurfaceParamsBinding locate a surface hook's param
// uniform struct (selenaParams). The host binds group 0 and 1 itself.
const (
	SurfaceParamsGroup   = 2
	SurfaceParamsBinding = 0
)

// SurfaceRecords declares the SelenaSurfaceGeo and SelenaSurface records of
// the surface hook ABI (bindings.SurfaceABI).
func SurfaceRecords() string {
	var b strings.Builder
	writeRecord(&b, "SelenaSurfaceGeo", ir.SurfaceGeoFields)
	writeRecord(&b, "SelenaSurface", ir.SurfaceFields)
	return b.String()
}

func writeRecord(b *strings.Builder, name string, fields []ir.SurfaceField) {
	fmt.Fprintf(b, "struct %s {\n", name)
	for _, f := range fields {
		fmt.Fprintf(b, "  %s : %s,\n", f.Name, typeName(f.Type))
	}
	b.WriteString("};\n")
}

// emitSurface writes a surface hook: the ABI records, the param uniform
// struct, and fn selenaSurface(selenaGeo) -> SelenaSurface. Textures are not
// declared; the host declares selenaSurfaceTex<i> and its Sampler, or renames
// them to its own bindings.
func emitSurface(m ir.Module) (string, error) {
	var b strings.Builder
	b.WriteString(SurfaceRecords())
	if len(m.Uniforms) > 0 || len(m.ArrayUniforms) > 0 {
		b.WriteString("struct SelenaSurfaceParams {\n")
		for _, u := range m.Uniforms {
			fmt.Fprintf(&b, "  %s : %s,\n", u.Name, typeName(u.Type))
		}
		for _, au := range m.ArrayUniforms {
			fmt.Fprintf(&b, "  %s : array<%s, %d>,\n", au.Name, typeName(au.ElemType), au.Count)
		}
		fmt.Fprintf(&b, "};\n@group(%d) @binding(%d) var<uniform> selenaParams : SelenaSurfaceParams;\n",
			SurfaceParamsGroup, SurfaceParamsBinding)
	}
	fs := internal.SurfaceResolver(internal.NewQualified(prismDialect, m, true), m, "selenaParams.")
	b.WriteString("fn selenaSurface(selenaGeo : SelenaSurfaceGeo) -> SelenaSurface {\n")
	internal.EmitStmtList(&b, m.Fragment.Body, fs, "  ", true)
	fmt.Fprintf(&b, "  return SelenaSurface(%s);\n}\n", strings.Join(m.SurfaceOutputs, ", "))
	return b.String(), nil
}

// SurfaceHarness wraps a surface hook in a complete fragment module so a WGSL
// validator can check it: it declares textures textures (2D, at group 1) and a
// fragmentMain that calls the hook. Hosts splice the bare hook instead.
func SurfaceHarness(hook string, textures int) string {
	var b strings.Builder
	for i := 0; i < textures; i++ {
		name := ir.SurfaceTextureName(i)
		fmt.Fprintf(&b, "@group(1) @binding(%d) var %s : texture_2d<f32>;\n", 1+2*i, name)
		fmt.Fprintf(&b, "@group(1) @binding(%d) var %sSampler : sampler;\n", 2+2*i, name)
	}
	b.WriteString(hook)
	b.WriteString(`
@fragment
fn fragmentMain(@location(0) worldPos : vec3<f32>, @location(1) worldNormal : vec3<f32>, @location(2) uv : vec2<f32>) -> @location(0) vec4<f32> {
  let n = normalize(worldNormal);
  let s = selenaSurface(SelenaSurfaceGeo(worldPos, n, uv, vec3<f32>(1.0, 0.0, 0.0), vec3<f32>(0.0, 0.0, 1.0), vec3<f32>(0.0, 0.0, 1.0), vec4<f32>(1.0)));
  return vec4<f32>(s.baseColor * s.occlusion + s.emissive + s.normal * s.roughness * s.metallic + vec3<f32>(s.clearcoat + s.clearcoatRoughness + s.sheen + s.transmission), s.alpha);
}
`)
	return b.String()
}
