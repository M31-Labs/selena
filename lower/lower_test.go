package lower

import (
	"strings"
	"testing"

	"m31labs.dev/selena/emit/wgsl"
	"m31labs.dev/selena/hir"
)

func TestLowerDirectionalDiffuse(t *testing.T) {
	mod, layout, err := Lower(hir.DirectionalDiffuse())
	if err != nil {
		t.Fatal(err)
	}

	// --- binding layout: a correct std140 block, 144 bytes ---
	if layout.UniformBlock.Size != 144 {
		t.Errorf("uniform block size = %d, want 144", layout.UniformBlock.Size)
	}
	off := map[string]int{}
	for _, f := range layout.UniformBlock.Fields {
		off[f.Name] = f.Offset
	}
	for name, want := range map[string]int{"mvp": 0, "normalMatrix": 64, "baseColor": 112} {
		if off[name] != want {
			t.Errorf("uniform %s offset = %d, want %d", name, off[name], want)
		}
	}
	if _, ok := off["light_dir"]; !ok {
		t.Error("Sun param did not expand into a light_dir uniform")
	}
	if len(layout.Attributes) != 2 || layout.Attributes[0].Name != "position" || layout.Attributes[0].Location != 0 {
		t.Errorf("attributes = %+v, want position@0 + normal@1", layout.Attributes)
	}

	// --- interpolant inference: worldNormal became a synthesized varying ---
	if len(mod.Varyings) != 1 || mod.Varyings[0].Name != "vWorldNormal" {
		t.Errorf("varyings = %+v, want [vWorldNormal]", mod.Varyings)
	}
	if len(mod.Vertex.Body) != 1 {
		t.Fatalf("vertex body = %d stmts, want 1 (the synthesized worldNormal)", len(mod.Vertex.Body))
	}

	// --- emit WGSL from the lowered LIR and check the key constructs ---
	src, err := wgsl.Emit(mod)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"out.vWorldNormal = normalize((u.normalMatrix * in.normal));",
		"out.position = (u.mvp * vec4<f32>(in.position, 1.0));",
		"let n = normalize(in.vWorldNormal);",
		"u.light_dir",
		"u.light_ambient",
		"return vec4<f32>((u.baseColor * ",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("lowered WGSL missing %q\n--- got ---\n%s", want, src)
		}
	}
}
