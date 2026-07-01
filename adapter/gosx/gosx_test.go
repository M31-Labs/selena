package gosx

import (
	"reflect"
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

func TestDefaultsPopulateCustomUniforms(t *testing.T) {
	im, layout, err := Material(hir.Defaults(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(layout.UniformBlock.Defaults) != 2 {
		t.Fatalf("descriptor defaults = %+v, want 2", layout.UniformBlock.Defaults)
	}
	if got := im.CustomUniforms["gain"]; got != float32(1) {
		t.Fatalf("gain default = %#v, want float32(1)", got)
	}
	wantColor := []float32{0.78, 0.42, 0.98}
	if got := im.CustomUniforms["baseColor"]; !reflect.DeepEqual(got, wantColor) {
		t.Fatalf("baseColor default = %#v, want %#v", got, wantColor)
	}
}

// contextDefaultMaterial declares a context field with a fallback default —
// the "context uniform" design's §3.1 native/test-oracle fallback, which must
// NEVER surface as an author-facing customUniforms default (C9): unlike a
// param default, a context default only backstops a host that omits the
// field, and shipping it in customUniforms could shadow a real per-frame
// host-supplied value.
func contextDefaultMaterial() hir.Material {
	return hir.Material{
		Name: "ContextDefaultMaterial",
		Context: []hir.ContextField{
			{Name: "gridResolution", Type: hir.Float, Default: hir.Lit{Value: 64.0}},
		},
		Surface: hir.Func{
			Geo: "geo",
			Result: hir.Call{Func: "rgb", Args: []hir.Expr{
				hir.Ref{Name: "gridResolution"},
				hir.Ref{Name: "gridResolution"},
				hir.Ref{Name: "gridResolution"},
			}},
		},
	}
}

func TestContextDefaultsNeverPopulateCustomUniforms(t *testing.T) {
	im, layout, err := Material(contextDefaultMaterial(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(layout.UniformBlock.Defaults) != 0 {
		t.Fatalf("context field default leaked into UniformBlock.Defaults: %+v", layout.UniformBlock.Defaults)
	}
	if im.CustomUniforms != nil {
		t.Fatalf("context field default leaked into CustomUniforms: %+v", im.CustomUniforms)
	}
	// Sanity: the field itself IS present in the uniform block and tagged context
	// (a context field always occupies a uniform-block slot even with no host
	// value supplied — only its DEFAULT is excluded from customUniforms).
	found := false
	for _, f := range layout.UniformBlock.Fields {
		if f.Name == "gridResolution" {
			found = true
			if f.Class != "context" {
				t.Fatalf("gridResolution.Class = %q, want %q", f.Class, "context")
			}
		}
	}
	if !found {
		t.Fatal("gridResolution not found in uniform block fields")
	}
	if len(layout.Context) != 1 || layout.Context[0] != "gridResolution" {
		t.Fatalf("layout.Context = %v, want [\"gridResolution\"]", layout.Context)
	}
}
