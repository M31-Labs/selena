package metal

import (
	"strings"
	"testing"

	"m31labs.dev/selena/ir"
)

func TestEmitDirectionalDiffuse(t *testing.T) {
	src, err := Emit(ir.DirectionalDiffuse())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"#include <metal_stdlib>",
		"float4x4 mvp;",
		"float3 position [[attribute(0)]];",
		"float4 position [[position]];",
		"vertex VertexOut vertexMain(VertexIn in [[stage_in]], constant Uniforms& u [[buffer(0)]])",
		"out.vNormal = normalize((u.normalMatrix * in.normal));",
		"out.position = (u.mvp * float4(in.position, 1.0));",
		"fragment float4 fragmentMain(",
		"float diff = max(dot(n, normalize(float3(u.lightDir))), 0.0);",
		"return float4(rgb, 1.0);",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("MSL missing %q\n--- got ---\n%s", want, src)
		}
	}
}

func uniformStruct(t *testing.T, m ir.Module) (string, error) {
	t.Helper()
	var b strings.Builder
	err := writeUniformStruct(&b, "Uniforms", m)
	return b.String(), err
}

func TestUniformStructPacksVec3AndPads(t *testing.T) {
	m := ir.Module{
		Uniforms: []ir.Binding{
			{Name: "a", Type: ir.Float},
			{Name: "v", Type: ir.Vec3},
			{Name: "w", Type: ir.Vec4},
		},
		ArrayUniforms: []ir.ArrayBinding{{Name: "items", ElemType: ir.Vec3, Count: 2}},
	}
	got, err := uniformStruct(t, m)
	if err != nil {
		t.Fatal(err)
	}
	// std140: a at 0, v at 16 (12 bytes of padding), w at 32 (4 bytes after
	// v), items at 48 with stride 16.
	want := "struct Uniforms {\n  float a;\n  char _selenaPad0[12];\n  packed_float3 v;\n  char _selenaPad1[4];\n  float4 w;\n  float3 items[2];\n};\n\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestUniformStructRejectsDisagreement(t *testing.T) {
	for name, m := range map[string]ir.Module{
		"mat4 array": {ArrayUniforms: []ir.ArrayBinding{{Name: "xs", ElemType: ir.Mat4, Count: 4}}},
		"pad prefix": {Uniforms: []ir.Binding{{Name: "_selenaPad0", Type: ir.Float}}},
		"bool":       {Uniforms: []ir.Binding{{Name: "on", Type: ir.Bool}}},
	} {
		if _, err := uniformStruct(t, m); err == nil {
			t.Errorf("%s: want an error, got none", name)
		}
	}
}

func TestPackedVec3ReadKeepsFloat3Type(t *testing.T) {
	src, err := Emit(ir.DirectionalDiffuse())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(src, "packed_float3 lightDir;") {
		t.Errorf("lightDir not declared packed_float3:\n%s", src)
	}
}

func TestNarrowUniformArraysKeepStride16(t *testing.T) {
	m := ir.Module{
		Uniforms: []ir.Binding{{Name: "s", Type: ir.Float}},
		ArrayUniforms: []ir.ArrayBinding{
			{Name: "f", ElemType: ir.Float, Count: 2},
			{Name: "i", ElemType: ir.Int, Count: 2},
			{Name: "u", ElemType: ir.Uint, Count: 2},
			{Name: "v", ElemType: ir.Vec2, Count: 2},
		},
	}
	got, err := uniformStruct(t, m)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"float4 f[2];", "int4 i[2];", "uint4 u[2];", "float4 v[2];"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	r := qualified(m, true)
	read := ir.Print(ir.Index{Arr: ir.Ref{Name: "v"}, Idx: ir.Ref{Name: "k"}}, r)
	if read != "u.v[k].xy" {
		t.Errorf("vec2 read = %q", read)
	}
	if read := ir.Print(ir.Index{Arr: ir.Ref{Name: "f"}, Idx: ir.Ref{Name: "k"}}, r); read != "u.f[k].x" {
		t.Errorf("float read = %q", read)
	}
}

func TestMatrixUniformArrayErrorMessage(t *testing.T) {
	for _, typ := range []ir.Type{ir.Mat3, ir.Mat4} {
		m := ir.Module{ArrayUniforms: []ir.ArrayBinding{{Name: "bones", ElemType: typ, Count: 2}}}
		_, err := uniformStruct(t, m)
		want := `uniform "bones": uniform arrays of mat3 or mat4 are not supported: the binding descriptor cannot describe their layout`
		if err == nil || err.Error() != want {
			t.Errorf("%s: err = %v, want %q", typ, err, want)
		}
	}
}
