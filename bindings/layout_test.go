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
		{"mvp", ir.Mat4},
		{"normalMatrix", ir.Mat3},
		{"baseColor", ir.Vec3},
		{"light_dir", ir.Vec3},
		{"light_ambient", ir.Float},
	})
	if block.Size != 144 {
		t.Fatalf("block size = %d, want 144", block.Size)
	}
	want := []Field{
		{"mvp", "mat4", 0, 64},
		{"normalMatrix", "mat3", 64, 48},
		{"baseColor", "vec3", 112, 12},
		{"light_dir", "vec3", 128, 12},
		{"light_ambient", "float", 140, 4},
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
	b := ComputeUniformBlock([]NamedType{{"c", ir.Vec3}, {"a", ir.Float}})
	if b.Fields[0].Offset != 0 || b.Fields[1].Offset != 12 {
		t.Fatalf("vec3+float offsets = %d,%d want 0,12", b.Fields[0].Offset, b.Fields[1].Offset)
	}
	if b.Size != 16 {
		t.Fatalf("size = %d want 16 (struct aligned to 16)", b.Size)
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
