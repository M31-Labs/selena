// Package validate compile-checks selena's emitted shaders with real offline
// shader compilers — naga for WGSL and glslangValidator for GLSL ES — so the
// emitters are proven to produce *valid* shaders, not merely structurally
// plausible strings. Each check skips cleanly where its tool isn't installed,
// so it runs as a real gate wherever the tools are present (CI/dev) and never
// blocks otherwise. (Metal is validated via gosx-native's iOS CI; no
// cross-platform MSL compiler exists here.)
package validate

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"m31labs.dev/selena/emit/gles"
	"m31labs.dev/selena/emit/wgsl"
	"m31labs.dev/selena/hir"
	"m31labs.dev/selena/lower"
)

func materials(t *testing.T) map[string]hir.Material {
	t.Helper()
	return map[string]hir.Material{
		"DirectionalDiffuse": hir.DirectionalDiffuse(),
		"Textured":           hir.Textured(),
	}
}

func TestGLESCompilesWithGlslang(t *testing.T) {
	gv, err := exec.LookPath("glslangValidator")
	if err != nil {
		t.Skip("glslangValidator not installed; skipping GLSL ES compile-check")
	}
	dir := t.TempDir()
	for name, m := range materials(t) {
		mod, _, err := lower.Lower(m)
		if err != nil {
			t.Fatal(err)
		}
		vert, frag, err := gles.Emit(mod)
		if err != nil {
			t.Fatal(err)
		}
		for ext, src := range map[string]string{"vert": vert, "frag": frag} {
			f := filepath.Join(dir, name+"."+ext)
			if err := os.WriteFile(f, []byte(src), 0o644); err != nil {
				t.Fatal(err)
			}
			if out, err := exec.Command(gv, f).CombinedOutput(); err != nil {
				t.Errorf("glslang rejected %s.%s:\n%s", name, ext, out)
			}
		}
	}
}

func TestWGSLCompilesWithNaga(t *testing.T) {
	naga, err := exec.LookPath("naga")
	if err != nil {
		t.Skip("naga not installed; skipping WGSL compile-check")
	}
	dir := t.TempDir()
	for name, m := range materials(t) {
		mod, _, err := lower.Lower(m)
		if err != nil {
			t.Fatal(err)
		}
		src, err := wgsl.Emit(mod)
		if err != nil {
			t.Fatal(err)
		}
		f := filepath.Join(dir, name+".wgsl")
		if err := os.WriteFile(f, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		if out, err := exec.Command(naga, f).CombinedOutput(); err != nil {
			t.Errorf("naga rejected %s WGSL:\n%s", name, out)
		}
	}
}
