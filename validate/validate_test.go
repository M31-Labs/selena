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
	"path/filepath"
	"strings"
	"testing"

	prismvalidate "m31labs.dev/prism/validate"

	selena "m31labs.dev/selena"
	"m31labs.dev/selena/emit/gles"
	"m31labs.dev/selena/emit/glsl"
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

// pointsMaterial returns a minimal points-kind material for validation.
func pointsMaterial() hir.Material {
	return hir.Material{
		Name: "GlowPointsValidate",
		Kind: hir.KindPoints,
		Params: []hir.Param{
			{Name: "fogColor", Type: hir.Vec3, Default: hir.Call{Func: "rgb", Args: []hir.Expr{hir.Lit{Value: 0}, hir.Lit{Value: 0}, hir.Lit{Value: 0}}}},
		},
		Surface: hir.Func{
			Geo: "pt",
			Body: []hir.Let{
				{Name: "centered", Value: hir.Binary{Op: "-",
					L: hir.Member{E: hir.Ref{Name: "pt"}, Field: "pointUV"},
					R: hir.Call{Func: "vec2f", Args: []hir.Expr{hir.Lit{Value: 0.5}, hir.Lit{Value: 0.5}}},
				}},
				{Name: "radial", Value: hir.Binary{Op: "*",
					L: hir.Call{Func: "length", Args: []hir.Expr{hir.Ref{Name: "centered"}}},
					R: hir.Lit{Value: 2.0},
				}},
				{Name: "core", Value: hir.Call{Func: "exp", Args: []hir.Expr{
					hir.Unary{Op: "-", E: hir.Binary{Op: "*",
						L: hir.Ref{Name: "radial"},
						R: hir.Binary{Op: "*", L: hir.Ref{Name: "radial"}, R: hir.Lit{Value: 4.2}},
					}},
				}}},
				{Name: "a", Value: hir.Binary{Op: "*",
					L: hir.Ref{Name: "core"},
					R: hir.Member{E: hir.Ref{Name: "pt"}, Field: "alpha"},
				}},
			},
			Result: hir.Call{Func: "rgb", Args: []hir.Expr{
				hir.Member{E: hir.Member{E: hir.Ref{Name: "pt"}, Field: "color"}, Field: "r"},
				hir.Member{E: hir.Member{E: hir.Ref{Name: "pt"}, Field: "color"}, Field: "g"},
				hir.Member{E: hir.Member{E: hir.Ref{Name: "pt"}, Field: "color"}, Field: "b"},
				hir.Ref{Name: "a"},
			}},
		},
	}
}

// postMaterial returns a minimal post-kind material for validation.
func postMaterial() hir.Material {
	return hir.Material{
		Name: "PassthroughValidate",
		Kind: hir.KindPost,
		Surface: hir.Func{
			Geo: "p",
			Result: hir.Call{Func: "sceneColor", Args: []hir.Expr{
				hir.Member{E: hir.Ref{Name: "p"}, Field: "uv"},
			}},
		},
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

func TestPointsWGSLCompilesWithNaga(t *testing.T) {
	mod, _, err := lower.Lower(pointsMaterial())
	if err != nil {
		t.Fatal(err)
	}
	src, err := wgsl.Emit(mod)
	if err != nil {
		t.Fatal(err)
	}
	prismvalidate.Shader(t, "naga", src, ".wgsl", nil)
}

func TestPostWGSLCompilesWithNaga(t *testing.T) {
	mod, _, err := lower.Lower(postMaterial())
	if err != nil {
		t.Fatal(err)
	}
	src, err := wgsl.Emit(mod)
	if err != nil {
		t.Fatal(err)
	}
	prismvalidate.Shader(t, "naga", src, ".wgsl", nil)
}

func TestPointsGLESCompilesWithGlslang(t *testing.T) {
	mod, _, err := lower.Lower(pointsMaterial())
	if err != nil {
		t.Fatal(err)
	}
	vert, frag, err := gles.Emit(mod)
	if err != nil {
		t.Fatal(err)
	}
	prismvalidate.Shader(t, "glslangValidator", vert, ".vert", nil)
	prismvalidate.Shader(t, "glslangValidator", frag, ".frag", nil)
}

func TestPostGLESCompilesWithGlslang(t *testing.T) {
	mod, _, err := lower.Lower(postMaterial())
	if err != nil {
		t.Fatal(err)
	}
	vert, frag, err := gles.Emit(mod)
	if err != nil {
		t.Fatal(err)
	}
	prismvalidate.Shader(t, "glslangValidator", vert, ".vert", nil)
	prismvalidate.Shader(t, "glslangValidator", frag, ".frag", nil)
}

// conformanceSelFiles returns all .sel files in testdata/conformance relative
// to this package. The path is resolved relative to the module root two levels up.
func conformanceSelFiles(t *testing.T) []string {
	t.Helper()
	// validate/ is one level below the module root; testdata/ is at root level.
	pattern := filepath.Join("..", "testdata", "conformance", "*.sel")
	files, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no conformance .sel files found at " + pattern)
	}
	return files
}

// TestConformanceWGSLCompilesWithNaga compiles every .sel file in the
// conformance corpus and validates the emitted WGSL with naga. This catches
// type-level bugs (e.g. vec3 vs vec4 mismatches) that string-compare golden
// tests cannot detect.
func TestConformanceWGSLCompilesWithNaga(t *testing.T) {
	for _, file := range conformanceSelFiles(t) {
		file := file
		name := strings.TrimSuffix(filepath.Base(file), ".sel")
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			res, err := selena.Compile(src, selena.CompileOptions{})
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			a, ok := res.Artifact(selena.TargetWGSL)
			if !ok {
				t.Fatal("no WGSL artifact")
			}
			prismvalidate.Shader(t, "naga", a.Source, ".wgsl", nil)
		})
	}
}

// TestConformanceGLSLCompilesWithGlslang compiles every .sel file in the
// conformance corpus and validates the emitted GLSL ES vertex+fragment with
// glslangValidator.
func TestConformanceGLSLCompilesWithGlslang(t *testing.T) {
	for _, file := range conformanceSelFiles(t) {
		file := file
		name := strings.TrimSuffix(filepath.Base(file), ".sel")
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			// Use the GLSL emitter directly so we exercise the WebGL1 path too.
			res, compErr := selena.Compile(src, selena.CompileOptions{})
			if compErr != nil {
				t.Fatalf("compile: %v", compErr)
			}
			mod := res.Module
			vert, frag, err := glsl.Emit(mod)
			if err != nil {
				t.Fatalf("glsl.Emit: %v", err)
			}
			t.Run("vert", func(t *testing.T) {
				prismvalidate.Shader(t, "glslangValidator", vert, ".vert", nil)
			})
			t.Run("frag", func(t *testing.T) {
				prismvalidate.Shader(t, "glslangValidator", frag, ".frag", nil)
			})
		})
	}
}

// TestConformanceGLESCompilesWithGlslang compiles every .sel file in the
// conformance corpus and validates the emitted GLES3 vertex+fragment with
// glslangValidator.
func TestConformanceGLESCompilesWithGlslang(t *testing.T) {
	for _, file := range conformanceSelFiles(t) {
		file := file
		name := strings.TrimSuffix(filepath.Base(file), ".sel")
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			res, compErr := selena.Compile(src, selena.CompileOptions{})
			if compErr != nil {
				t.Fatalf("compile: %v", compErr)
			}
			mod := res.Module
			vert, frag, err := gles.Emit(mod)
			if err != nil {
				t.Fatalf("gles.Emit: %v", err)
			}
			t.Run("vert", func(t *testing.T) {
				prismvalidate.Shader(t, "glslangValidator", vert, ".vert", nil)
			})
			t.Run("frag", func(t *testing.T) {
				prismvalidate.Shader(t, "glslangValidator", frag, ".frag", nil)
			})
		})
	}
}
