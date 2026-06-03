// Package validate compile-checks selena's emitted shaders with real offline
// shader compilers — naga for WGSL and glslangValidator for GLSL ES — so the
// emitters are proven to produce *valid* shaders, not merely structurally
// plausible strings. Each check skips cleanly where its tool isn't installed,
// so it runs as a real gate wherever the tools are present (CI/dev) and never
// blocks otherwise. (Metal is validated via gosx-native's iOS CI; no
// cross-platform MSL compiler exists here.)
package validate

import (
	"testing"

	prismvalidate "m31labs.dev/prism/validate"

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
	for name, m := range materials(t) {
		mod, _, err := lower.Lower(m)
		if err != nil {
			t.Fatal(err)
		}
		vert, frag, err := gles.Emit(mod)
		if err != nil {
			t.Fatal(err)
		}
		t.Run(name+".vert", func(t *testing.T) {
			prismvalidate.Shader(t, "glslangValidator", vert, ".vert", nil)
		})
		t.Run(name+".frag", func(t *testing.T) {
			prismvalidate.Shader(t, "glslangValidator", frag, ".frag", nil)
		})
	}
}

func TestWGSLCompilesWithNaga(t *testing.T) {
	for name, m := range materials(t) {
		mod, _, err := lower.Lower(m)
		if err != nil {
			t.Fatal(err)
		}
		src, err := wgsl.Emit(mod)
		if err != nil {
			t.Fatal(err)
		}
		t.Run(name, func(t *testing.T) {
			prismvalidate.Shader(t, "naga", src, ".wgsl", nil)
		})
	}
}
