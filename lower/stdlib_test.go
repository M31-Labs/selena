package lower

import (
	"testing"

	"m31labs.dev/selena/ir"
)

func TestStdlibRegistryCoversCurrentRecordsAndGeometry(t *testing.T) {
	if tpe, ok := stdlib.recordField("Sun", "dir"); !ok || tpe != ir.Vec3 {
		t.Fatalf("Sun.dir = %s/%v, want vec3/true", tpe, ok)
	}
	if tpe, ok := stdlib.recordField("Sun", "ambient"); !ok || tpe != ir.Float {
		t.Fatalf("Sun.ambient = %s/%v, want float/true", tpe, ok)
	}

	for name, want := range map[string]ir.Type{
		"position":    ir.Vec3,
		"normal":      ir.Vec3,
		"uv":          ir.Vec2,
		"worldNormal": ir.Vec3,
	} {
		got, ok := stdlib.geometryField(name)
		if !ok {
			t.Fatalf("geometry field %s is not registered", name)
		}
		if got.typ != want {
			t.Fatalf("geometry field %s type = %s, want %s", name, got.typ, want)
		}
		if got.varying == "" || got.build == nil {
			t.Fatalf("geometry field %s missing varying/build: %+v", name, got)
		}
	}
}
