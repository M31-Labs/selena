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
			Body: []hir.Stmt{
				hir.Let{Name: "centered", Value: hir.Binary{Op: "-",
					L: hir.Member{E: hir.Ref{Name: "pt"}, Field: "pointUV"},
					R: hir.Call{Func: "vec2f", Args: []hir.Expr{hir.Lit{Value: 0.5}, hir.Lit{Value: 0.5}}},
				}},
				hir.Let{Name: "radial", Value: hir.Binary{Op: "*",
					L: hir.Call{Func: "length", Args: []hir.Expr{hir.Ref{Name: "centered"}}},
					R: hir.Lit{Value: 2.0},
				}},
				hir.Let{Name: "core", Value: hir.Call{Func: "exp", Args: []hir.Expr{
					hir.Unary{Op: "-", E: hir.Binary{Op: "*",
						L: hir.Ref{Name: "radial"},
						R: hir.Binary{Op: "*", L: hir.Ref{Name: "radial"}, R: hir.Lit{Value: 4.2}},
					}},
				}}},
				hir.Let{Name: "a", Value: hir.Binary{Op: "*",
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
			// Pin GLSL ES 1.00. Without a #version directive glslang validates
			// against desktop GLSL 110, where derivatives are core and no
			// extension is required — so the ES 1.00 rules the browser applies
			// went unchecked. See essl100 in post_surface_test.go.
			t.Run("vert", func(t *testing.T) {
				prismvalidate.Shader(t, "glslangValidator", essl100(vert), ".vert", nil)
			})
			t.Run("frag", func(t *testing.T) {
				prismvalidate.Shader(t, "glslangValidator", essl100(frag), ".frag", nil)
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

// TestWGSLSampleInsideNonUniformEarlyReturnValidatesWithNaga checks the naga
// uniformity risk the Phase 1 investigation flagged: existing guidance
// (ir.go's SampleLevel doc) warns that a sample() call living inside
// non-uniform (data-dependent) control flow can fail naga's "must be called
// from uniform control flow" validation for implicit-derivative texture
// functions, and recommends sampleLevel() as the safe alternative. An early
// return does not change that risk — the branch was already non-uniform
// before the return existed — but Phase 1 makes this shape newly reachable
// (`if (non-uniform) { return sample(...) }`), so it is verified here rather
// than left as a deduction. With naga 29.0.3 (as installed for this repo)
// this specific shape validates successfully; the test pins that empirical
// result and will fail loudly (not silently) if a naga upgrade changes it.
func TestWGSLSampleInsideNonUniformEarlyReturnValidatesWithNaga(t *testing.T) {
	src := []byte(`material NonUniformEarlyReturnSample {
    param tex : texture2d

    surface(geo) -> color {
        if (geo.uv.x < 0.5) {
            return sample(tex, geo.uv)
        }
        return rgb(0.0, 0.0, 0.0)
    }
}`)
	res, err := selena.Compile(src, selena.CompileOptions{Targets: []selena.Target{selena.TargetWGSL}})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	a, ok := res.Artifact(selena.TargetWGSL)
	if !ok {
		t.Fatal("no WGSL artifact")
	}
	prismvalidate.Shader(t, "naga", a.Source, ".wgsl", nil)
}

// TestWGSLSampleLevelInsideNonUniformEarlyReturnValidatesWithNaga is the
// companion to the sample() case above, using the documented-safe
// alternative (ir.go's SampleLevel guidance): sampleLevel() bypasses the
// implicit-derivative LOD selection, so it is legal inside non-uniform
// control flow on every backend, including an early-return branch.
func TestWGSLSampleLevelInsideNonUniformEarlyReturnValidatesWithNaga(t *testing.T) {
	src := []byte(`material NonUniformEarlyReturnSampleLevel {
    param tex : texture2d

    surface(geo) -> color {
        if (geo.uv.x < 0.5) {
            return sampleLevel(tex, geo.uv, 0.0)
        }
        return rgb(0.0, 0.0, 0.0)
    }
}`)
	res, err := selena.Compile(src, selena.CompileOptions{Targets: []selena.Target{selena.TargetWGSL}})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	a, ok := res.Artifact(selena.TargetWGSL)
	if !ok {
		t.Fatal("no WGSL artifact")
	}
	prismvalidate.Shader(t, "naga", a.Source, ".wgsl", nil)
}

// TestFeedbackEarlyReturnValidatesWithNaga checks the one WGSL shape whose
// early return cannot use a plain `return val;` (the entry is a void compute
// kernel): the ReturnFn wired in emit/wgsl writes outState[cellIndex] then
// bare-returns. Validated with naga rather than left to a string-compare
// golden alone, since a malformed compute kernel is exactly the kind of
// defect a golden test can't catch (see TestConformanceWGSLCompilesWithNaga's
// doc comment for the same rationale).
func TestFeedbackEarlyReturnValidatesWithNaga(t *testing.T) {
	src := []byte(`material FeedbackEarlyReturnValidate kind feedback {
    param cutoff : float = 0.5
    state water

    feedback(cell) -> vec4 {
        let here = state(0, 0)
        let uv = cell.uv
        if (uv.x < cutoff) {
            return vec4f(0.0, 0.0, 0.0, 0.0)
        }
        return here
    }
}`)
	res, err := selena.Compile(src, selena.CompileOptions{Targets: []selena.Target{selena.TargetWGSL}})
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	a, ok := res.Artifact(selena.TargetWGSL)
	if !ok {
		t.Fatal("no WGSL artifact")
	}
	prismvalidate.Shader(t, "naga", a.Source, ".wgsl", nil)
}
