// Package validate compile-checks selena's emitted shaders with real offline
// shader compilers — naga for WGSL and glslangValidator for GLSL ES — so the
// emitters are proven to produce *valid* shaders, not merely structurally
// plausible strings. Each check skips cleanly where its tool isn't installed,
// so it runs as a real gate wherever the tools are present (CI/dev) and never
// blocks otherwise. (Metal is validated via gosx-native's iOS CI; no
// cross-platform MSL compiler exists here.)
package validate

import (
	"errors"
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

// TestVertexStageRejectsFragmentOnlyBuiltins is the compile-time half of the
// silent-invalid-shader defect: dpdx/dpdy/fwidth and the sample/sampleLevel/
// sampleCube family used to type-check and resolve inside an authored
// vertex() body, so Selena reported a successful compile while emitting a
// shader that failed validation on every backend — see
// TestPreFixVertexDerivativeWouldHaveFailedNagaValidation and
// TestPreFixVertexSampleWouldHaveFailedNagaValidation /
// TestPreFixVertexSampleWouldHaveFailedGlslangValidation below for the
// empirical proof. lower/resolver.go's and lower/typer.go's inVertexStage
// guard now rejects all six at compile time instead.
func TestVertexStageRejectsFragmentOnlyBuiltins(t *testing.T) {
	cases := []struct {
		name string
		call string
	}{
		{"dpdx", "dpdx(vec3f(1.0, 2.0, 3.0))"},
		{"dpdy", "dpdy(vec3f(1.0, 2.0, 3.0))"},
		{"fwidth", "fwidth(vec3f(1.0, 2.0, 3.0))"},
		{"sample", "sample(albedo, vec2f(0.0, 0.0))"},
		{"sampleLevel", "sampleLevel(albedo, vec2f(0.0, 0.0), 0.0)"},
		{"sampleCube", "sampleCube(sky, vec3f(0.0, 1.0, 0.0))"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			src := []byte(`material Bad {
    param albedo : texture2d
    param sky    : textureCube
    vertex() -> vec4 {
        let s = ` + c.call + `
        return vec4f(1.0, 1.0, 1.0, 1.0)
    }
    surface(geo) -> color {
        return rgb(1.0, 1.0, 1.0)
    }
}`)
			_, err := selena.Compile(src, selena.CompileOptions{Targets: []selena.Target{}})
			if err == nil {
				t.Fatalf("%s in vertex() compiled, want a diagnostic (it used to reach every emitter and fail validation there)", c.name)
			}
			var ce *selena.CompileError
			if !errors.As(err, &ce) {
				t.Fatalf("error type = %T, want *selena.CompileError", err)
			}
			if len(ce.Diagnostics) == 0 || ce.Diagnostics[0].Code != "SEL2003" {
				t.Fatalf("diagnostics = %+v, want a SEL2003", ce.Diagnostics)
			}
		})
	}
}

// TestPreFixVertexDerivativeWouldHaveFailedNagaValidation is not a test of
// current Selena output — the guard above means Selena can no longer produce
// this shader — it is the empirical proof that the guard is warranted. This
// WGSL is exactly the shape emitMeshAuthored would render for a `dpdx(...)`
// call inside vertex() (ir.Print's generic Call path has no stage awareness;
// see wgsl.go's emitMeshAuthored), and naga rejects it: WGSL's derivative
// builtins are fragment-stage only. Confirmed with naga 29.0.3.
func TestPreFixVertexDerivativeWouldHaveFailedNagaValidation(t *testing.T) {
	src := `struct VertexOutput {
  @builtin(position) position : vec4<f32>,
};

@vertex
fn vertexMain() -> VertexOutput {
  var out : VertexOutput;
  let d = dpdx(vec3<f32>(1.0, 2.0, 3.0));
  out.position = vec4<f32>(d, 1.0);
  return out;
}
`
	r, err := prismvalidate.Run("naga", src, ".wgsl", nil)
	if r.Skipped {
		t.Skip("naga: skipped (not on PATH)")
	}
	if err == nil {
		t.Fatalf("naga accepted dpdx() in a vertex stage, want a validation error\n%s", r.Output)
	}
}

// TestPreFixVertexSampleWouldHaveFailedNagaValidation is the sample()
// counterpart: naga rejects textureSample (implicit-derivative sampling)
// outside the fragment stage. This is exactly the WGSL emitMeshAuthored would
// render for `sample(tex, uv)` inside vertex() (resolver.go's Sample case
// lowers to the same ir.Sample regardless of stage; wgsl.go's Dialect.Sample
// always renders textureSample). Confirmed with naga 29.0.3.
func TestPreFixVertexSampleWouldHaveFailedNagaValidation(t *testing.T) {
	src := `@group(0) @binding(1) var tex : texture_2d<f32>;
@group(0) @binding(2) var texSampler : sampler;

struct VertexOutput {
  @builtin(position) position : vec4<f32>,
};

@vertex
fn vertexMain() -> VertexOutput {
  var out : VertexOutput;
  let c = textureSample(tex, texSampler, vec2<f32>(0.5, 0.5));
  out.position = vec4<f32>(c.xyz, 1.0);
  return out;
}
`
	r, err := prismvalidate.Run("naga", src, ".wgsl", nil)
	if r.Skipped {
		t.Skip("naga: skipped (not on PATH)")
	}
	if err == nil {
		t.Fatalf("naga accepted sample() (textureSample) in a vertex stage, want a validation error\n%s", r.Output)
	}
}

// TestPreFixVertexIndexInControlFlowWouldHaveFailedNagaValidation is not a
// test of current Selena output — lower/lower_vertex.go now derives
// UsesVertexIndex from ir.StageUsesVertexIndexBuiltin, which walks the
// exhaustive ir/uses.go statement/expression walker and finds a vertexIndex
// read anywhere in the body — it is the empirical proof that fix was
// warranted.
//
// Before that fix, UsesVertexIndex came from a hand-rolled walker
// (lower/lower_vertex.go's former irStageUsesVertexIndex) that only scanned
// CF-less statements' Value field. A vertexIndex read reassigned inside an
// `if`, like `x = float(vertexIndex) * 0.01`, was invisible to it: the vertex
// entry point's @builtin(vertex_index) parameter was omitted while the body
// still referenced vertexIndex. This WGSL is exactly that shape — the
// material was:
//
//	material ProceduralAssign {
//	    vertex() -> vec4 {
//	        var x = 0.0
//	        x = float(vertexIndex) * 0.01
//	        return mvp * vec4f(x, 0.0, 0.0, 1.0)
//	    }
//	    surface(geo) -> color { return rgb(1.0, 1.0, 1.0) }
//	}
//
// — and naga rejects the emitted vertexMain: vertexIndex has no definition in
// scope. Confirmed with naga 29.0.3.
func TestPreFixVertexIndexInControlFlowWouldHaveFailedNagaValidation(t *testing.T) {
	src := `struct Uniforms {
  mvp : mat4x4<f32>,
  normalMatrix : mat3x3<f32>,
};
@group(0) @binding(0) var<uniform> u : Uniforms;

struct VertexOutput {
  @builtin(position) position : vec4<f32>,
};

@vertex
fn vertexMain() -> VertexOutput {
  var out : VertexOutput;
  var x : f32 = 0.0;
  x = (f32(vertexIndex) * 0.01);
  out.position = (u.mvp * vec4<f32>(x, 0.0, 0.0, 1.0));
  return out;
}

@fragment
fn fragmentMain(in : VertexOutput) -> @location(0) vec4<f32> {
  return vec4<f32>(vec3<f32>(1.0, 1.0, 1.0), 1.0);
}
`
	r, err := prismvalidate.Run("naga", src, ".wgsl", nil)
	if r.Skipped {
		t.Skip("naga: skipped (not on PATH)")
	}
	if err == nil {
		t.Fatalf("naga accepted an undeclared vertexIndex reference, want a validation error\n%s", r.Output)
	}
}

// TestPreFixVertexSampleWouldHaveFailedGlslangValidation is the GLSL ES 1.00
// counterpart, and the reason the fix rejects sample()/sampleLevel()/
// sampleCube() rather than trying to "emit correctly": none of the three
// non-WGSL authored-vertex emitters declare a texture/sampler binding in the
// vertex source at all (glsl.go's and gles.go's emitVertexAuthored, and
// metal.go's emitMeshAuthored, only loop over m.Textures for the FRAGMENT
// half). A sample() call in vertex() referenced an undeclared identifier —
// this .vert source is exactly that shape. Confirmed with glslangValidator
// 11:15.1.0.
func TestPreFixVertexSampleWouldHaveFailedGlslangValidation(t *testing.T) {
	src := `attribute vec3 position;

void main() {
  vec4 c = texture2D(tex, vec2(0.5, 0.5));
  gl_Position = vec4(c.xyz + position, 1.0);
}
`
	r, err := prismvalidate.Run("glslangValidator", src, ".vert", nil)
	if r.Skipped {
		t.Skip("glslangValidator: skipped (not on PATH)")
	}
	if err == nil {
		t.Fatalf("glslangValidator accepted an undeclared sampler in the vertex stage, want a validation error\n%s", r.Output)
	}
}
