package materiallib

import (
	"slices"
	"strings"
	"testing"
)

func TestModulesListsEveryBundledModule(t *testing.T) {
	got := Modules()
	for _, want := range []Module{BRDF, Procedural, Shapes, Normals, Effects, Sky, Lighting} {
		if !slices.Contains(got, want) {
			t.Errorf("Modules() lacks %q: %v", want, got)
		}
	}
	if !slices.IsSorted(got) {
		t.Fatalf("Modules() is not sorted: %v", got)
	}
	for _, module := range got {
		if _, err := Functions(module); err != nil {
			t.Errorf("listed module %q does not load: %v", module, err)
		}
	}
}

func TestSourceReturnsModuleText(t *testing.T) {
	src, err := Source(Lighting)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), "fn mlFogFactor(") {
		t.Fatalf("lighting source lacks mlFogFactor")
	}
	for _, bad := range []Module{"nope", "../library", "a.b"} {
		if _, err := Source(bad); err == nil {
			t.Errorf("Source(%q) succeeded", bad)
		}
	}
}
