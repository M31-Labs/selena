package gosx

import (
	"testing"

	"m31labs.dev/gosx/scene/capability"

	"m31labs.dev/selena/hir"
)

// TestSelenaMaterialServesAllBackends runs the real GoSX WebGPU honesty-gate
// resolver on a selena-produced IRMaterial and proves every backend is served:
// because selena fills both the GLSL and WGSL slots from one source, the gate
// never degrades the material.
func TestSelenaMaterialServesAllBackends(t *testing.T) {
	im, layout, err := Material(hir.DirectionalDiffuse(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if im.Kind != "custom" || im.Name != "DirectionalDiffuse" {
		t.Fatalf("unexpected IRMaterial header: kind=%q name=%q", im.Kind, im.Name)
	}
	for name, v := range map[string]string{
		"CustomVertex":       im.CustomVertex,
		"CustomFragment":     im.CustomFragment,
		"CustomVertexWGSL":   im.CustomVertexWGSL,
		"CustomFragmentWGSL": im.CustomFragmentWGSL,
	} {
		if v == "" {
			t.Errorf("%s is empty — that backend would be degraded by the honesty gate", name)
		}
	}
	if layout.UniformBlock.Size != 144 {
		t.Errorf("descriptor uniform block = %d bytes, want 144", layout.UniformBlock.Size)
	}

	src := capability.CustomMaterialSources{
		GLSL: im.CustomVertex != "" || im.CustomFragment != "",
		WGSL: im.CustomVertexWGSL != "" || im.CustomFragmentWGSL != "",
	}
	served := capability.PresenceResolver{}.Serves(src)
	if len(served) == 0 {
		t.Fatal("resolver returned no backends")
	}
	for b, ok := range served {
		if !ok {
			t.Errorf("backend %v NOT served — the gate would degrade this selena material", b)
		}
	}
}

func TestTexturedAdapts(t *testing.T) {
	im, _, err := Material(hir.Textured(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if im.CustomFragmentWGSL == "" || im.CustomFragment == "" {
		t.Fatalf("textured material slots not filled: %+v", im.Name)
	}
}
