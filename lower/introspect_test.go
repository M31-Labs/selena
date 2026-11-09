package lower

import (
	"slices"
	"testing"

	"m31labs.dev/selena/hir"
	"m31labs.dev/selena/ir"
)

func TestBuiltinsListsTheTableAndStateReads(t *testing.T) {
	names := Builtins()
	if !slices.IsSorted(names) {
		t.Fatalf("Builtins is not sorted: %v", names)
	}
	for _, want := range []string{"dot", "sample", "sceneColor", "state", "stateAt", "vec4f"} {
		if !slices.Contains(names, want) {
			t.Errorf("Builtins lacks %q", want)
		}
	}
	if len(names) != len(stdlib.builtins)+2 {
		t.Fatalf("Builtins has %d names, want %d", len(names), len(stdlib.builtins)+2)
	}
}

func TestGeometryFieldsFollowKindAndStage(t *testing.T) {
	if got := GeometryFields(hir.KindMesh, false)["worldNormal"]; got != ir.Vec3 {
		t.Fatalf("mesh worldNormal = %q", got)
	}
	if _, ok := GeometryFields(hir.KindMesh, true)["worldNormal"]; ok {
		t.Fatal("the authored vertex stage reads plain attributes only")
	}
	if got := GeometryFields(hir.KindPost, false); len(got) != 1 || got["uv"] != ir.Vec2 {
		t.Fatalf("post fields = %v", got)
	}
	if _, ok := GeometryFields(hir.KindPoints, false)["pointUV"]; !ok {
		t.Fatal("points lack pointUV")
	}
	fields := GeometryFields(hir.KindMesh, false)
	fields["worldNormal"] = ir.Float
	if GeometryFields(hir.KindMesh, false)["worldNormal"] != ir.Vec3 {
		t.Fatal("GeometryFields exposes the registry map")
	}
	if got := RecordFields(hir.Sun); got["dir"] != ir.Vec3 || got["ambient"] != ir.Float {
		t.Fatalf("Sun fields = %v", got)
	}
	if RecordFields(hir.Float) != nil {
		t.Fatal("float is not a record")
	}
}
