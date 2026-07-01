package bindings

import (
	"strings"
	"testing"

	"m31labs.dev/selena/ir"
)

// TestDirectionalDiffuseLayout pins the std140 layout that was hand-computed for
// the demo harness: mvp@0, normalMatrix@64 (48-byte mat3!), baseColor@112,
// light_dir@128, light_ambient@140, total 144.
func TestDirectionalDiffuseLayout(t *testing.T) {
	block := ComputeUniformBlock([]NamedType{
		{Name: "mvp", Type: ir.Mat4},
		{Name: "normalMatrix", Type: ir.Mat3},
		{Name: "baseColor", Type: ir.Vec3},
		{Name: "light_dir", Type: ir.Vec3},
		{Name: "light_ambient", Type: ir.Float},
	})
	if block.Size != 144 {
		t.Fatalf("block size = %d, want 144", block.Size)
	}
	want := []Field{
		{Name: "mvp", Type: "mat4", Offset: 0, Size: 64},
		{Name: "normalMatrix", Type: "mat3", Offset: 64, Size: 48},
		{Name: "baseColor", Type: "vec3", Offset: 112, Size: 12},
		{Name: "light_dir", Type: "vec3", Offset: 128, Size: 12},
		{Name: "light_ambient", Type: "float", Offset: 140, Size: 4},
	}
	if len(block.Fields) != len(want) {
		t.Fatalf("got %d fields, want %d", len(block.Fields), len(want))
	}
	for i, w := range want {
		if block.Fields[i] != w {
			t.Errorf("field %d = %+v, want %+v", i, block.Fields[i], w)
		}
	}
}

func TestVec3PackingAndStructAlign(t *testing.T) {
	// vec3 then float: the float packs into the vec3's 16-byte slot tail.
	b := ComputeUniformBlock([]NamedType{{Name: "c", Type: ir.Vec3}, {Name: "a", Type: ir.Float}})
	if b.Fields[0].Offset != 0 || b.Fields[1].Offset != 12 {
		t.Fatalf("vec3+float offsets = %d,%d want 0,12", b.Fields[0].Offset, b.Fields[1].Offset)
	}
	if b.Size != 16 {
		t.Fatalf("size = %d want 16 (struct aligned to 16)", b.Size)
	}
}

// TestComputeUniformBlockOrdersScalarsBeforeArraysRegardlessOfDeclaration
// locks the fix for a real bug: every backend emitter (WGSL/GLSL/GLES/Metal)
// renders a material's uniform-block struct as scalars/vectors/matrices
// first, then fixed-size arrays (see lower.toBindings/toArrayBindings, which
// split the declaration-ordered field list into exactly those two groups for
// the emitters to concatenate). ComputeUniformBlock must offset fields in
// that SAME order — scalars first, arrays last — not raw declaration order,
// or a scalar declared after an array gets a descriptor offset that disagrees
// with the emitted struct's actual member offset (confirmed empirically on
// the gosx water shader via spirv-dis: scalar dropCount at WGSL offset 4, but
// the pre-fix descriptor put it after a 1024-byte array).
func TestComputeUniformBlockOrdersScalarsBeforeArraysRegardlessOfDeclaration(t *testing.T) {
	// Declaration order: a (scalar), arr (array), b (scalar) — array sandwiched
	// between two scalars, exactly the pattern the bug mishandled.
	block := ComputeUniformBlock([]NamedType{
		{Name: "a", Type: ir.Float},
		{Name: "arr", Type: ir.Vec4, Count: 4},
		{Name: "b", Type: ir.Float},
	})

	// Emitted struct order (scalars-first): a, b, arr.
	want := []Field{
		{Name: "a", Type: "float", Offset: 0, Size: 4},
		{Name: "b", Type: "float", Offset: 4, Size: 4},
		{Name: "arr", Type: "vec4", Offset: 16, Size: 64, Count: 4, Stride: 16},
	}
	if len(block.Fields) != len(want) {
		t.Fatalf("got %d fields, want %d: %+v", len(block.Fields), len(want), block.Fields)
	}
	for i, w := range want {
		if block.Fields[i] != w {
			t.Errorf("field %d = %+v, want %+v", i, block.Fields[i], w)
		}
	}
	if block.Size != 80 {
		t.Fatalf("block size = %d, want 80", block.Size)
	}

	// The invariant the bug violated: scalar offsets come before the array's
	// offset, no matter where the array sits in declaration order.
	byName := map[string]Field{}
	for _, f := range block.Fields {
		byName[f.Name] = f
	}
	if byName["a"].Offset >= byName["arr"].Offset || byName["b"].Offset >= byName["arr"].Offset {
		t.Fatalf("scalar offsets must precede array offset: a=%d b=%d arr=%d", byName["a"].Offset, byName["b"].Offset, byName["arr"].Offset)
	}
}

func TestLayoutJSONIncludesVersions(t *testing.T) {
	layout := Layout{
		Material:     "Versioned",
		UniformBlock: ComputeUniformBlock(nil),
	}
	got, err := layout.JSON()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`"schemaVersion": "selena.descriptor.v1"`,
		`"languageVersion": "selena.lang.v1"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("descriptor JSON missing %s\n%s", want, got)
		}
	}
}
