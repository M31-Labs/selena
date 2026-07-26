package selena

import (
	"errors"
	"os"
	"strings"
	"testing"

	"m31labs.dev/selena/hir"
)

func TestCompileDefaultsToAllTargets(t *testing.T) {
	src, err := os.ReadFile("examples/textured.sel")
	if err != nil {
		t.Fatal(err)
	}

	res, err := Compile(src, CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Material.Name != "Textured" || res.Module.Name != "Textured" {
		t.Fatalf("compiled material = %s/%s, want Textured", res.Material.Name, res.Module.Name)
	}
	if len(res.Artifacts) != len(AllTargets()) {
		t.Fatalf("artifacts = %d, want %d", len(res.Artifacts), len(AllTargets()))
	}
	if len(res.Layout.Textures) != 1 || res.Layout.Textures[0].Name != "albedo" {
		t.Fatalf("textures = %+v, want albedo", res.Layout.Textures)
	}

	wgsl, ok := res.Artifact(TargetWGSL)
	if !ok {
		t.Fatal("missing WGSL artifact")
	}
	if !strings.Contains(wgsl.Source, "textureSample(albedo, albedoSampler, in.vUv)") {
		t.Fatalf("WGSL artifact does not contain texture sample:\n%s", wgsl.Source)
	}
	glsl, ok := res.Artifact(TargetGLSL)
	if !ok {
		t.Fatal("missing GLSL artifact")
	}
	if glsl.Vertex == "" || glsl.Fragment == "" || glsl.Source != "" {
		t.Fatalf("GLSL artifact shape = %+v, want split vertex/fragment", glsl)
	}
}

func TestCompileMaterialSelectsNamedMaterial(t *testing.T) {
	src, err := os.ReadFile("examples/tinted.sel")
	if err != nil {
		t.Fatal(err)
	}

	base, err := Compile(src, CompileOptions{
		Material: "Base",
		Targets:  []Target{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if base.Module.Name != "Base" {
		t.Fatalf("module = %s, want Base", base.Module.Name)
	}
	if hasUniform(base, "tint") {
		t.Fatalf("Base should not include child tint uniform: %+v", base.Module.Uniforms)
	}

	derived, err := Compile(src, CompileOptions{Targets: []Target{}})
	if err != nil {
		t.Fatal(err)
	}
	if derived.Module.Name != "Tinted" {
		t.Fatalf("default module = %s, want Tinted", derived.Module.Name)
	}
	if !hasUniform(derived, "tint") || !hasUniform(derived, "baseColor") {
		t.Fatalf("Tinted uniforms missing inherited or child params: %+v", derived.Module.Uniforms)
	}
	if len(derived.Artifacts) != 0 {
		t.Fatalf("Targets: []Target{} should suppress emission, got %+v", derived.Artifacts)
	}

	again, err := CompileMaterial(derived.Program, "Base", CompileOptions{Targets: []Target{TargetWGSL}})
	if err != nil {
		t.Fatal(err)
	}
	if again.Module.Name != "Base" || len(again.Artifacts) != 1 {
		t.Fatalf("CompileMaterial result = %s/%d artifacts, want Base/1", again.Module.Name, len(again.Artifacts))
	}
}

func TestCompileRejectsUnknownMaterialAndTarget(t *testing.T) {
	src, err := os.ReadFile("examples/directional-diffuse.sel")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := Compile(src, CompileOptions{Material: "Missing"}); err == nil || !strings.Contains(err.Error(), `material "Missing" not found`) {
		t.Fatalf("unknown material error = %v", err)
	}
	if _, err := Compile(src, CompileOptions{Targets: []Target{"spirv"}}); err == nil || !strings.Contains(err.Error(), `unknown target "spirv"`) {
		t.Fatalf("unknown target error = %v", err)
	}
}

func TestCompileErrorCarriesSourceRange(t *testing.T) {
	src := []byte(`material Bad {
    surface(geo) -> color {
        return missing
    }
}`)

	_, err := Compile(src, CompileOptions{Targets: []Target{}})
	if err == nil {
		t.Fatal("Compile succeeded, want diagnostic error")
	}
	var ce *CompileError
	if !errors.As(err, &ce) {
		t.Fatalf("error type = %T, want *CompileError", err)
	}
	if len(ce.Diagnostics) != 1 {
		t.Fatalf("diagnostics = %d, want 1", len(ce.Diagnostics))
	}
	d := ce.Diagnostics[0]
	if d.Code != "SEL2001" {
		t.Fatalf("diagnostic code = %s, want SEL2001", d.Code)
	}
	if d.Range.Start.Line != 3 || d.Range.Start.Column != 16 {
		t.Fatalf("range start = %d:%d, want 3:16", d.Range.Start.Line, d.Range.Start.Column)
	}
	if !strings.Contains(err.Error(), "SEL2001 at 3:16") {
		t.Fatalf("error string = %q, want line/column context", err.Error())
	}
}

func TestCompileErrorCarriesInlineCallRange(t *testing.T) {
	src := []byte(`fn tint(c: color) -> color {
    return c
}
material Bad {
    surface(geo) -> color {
        return tint()
    }
}`)

	_, err := Compile(src, CompileOptions{Targets: []Target{}})
	if err == nil {
		t.Fatal("Compile succeeded, want diagnostic error")
	}
	var ce *CompileError
	if !errors.As(err, &ce) {
		t.Fatalf("error type = %T, want *CompileError", err)
	}
	d := ce.Diagnostics[0]
	if d.Code != "SEL2003" {
		t.Fatalf("diagnostic code = %s, want SEL2003", d.Code)
	}
	if d.Range.Start.Line != 6 || d.Range.Start.Column != 16 {
		t.Fatalf("range start = %d:%d, want 6:16", d.Range.Start.Line, d.Range.Start.Column)
	}
	if !strings.Contains(d.Message, "fn tint expects 1 args, got 0") {
		t.Fatalf("diagnostic message = %q", d.Message)
	}
}

func TestCompileErrorRejectsSuperWithoutExtends(t *testing.T) {
	src := []byte(`material Bad {
    surface(geo) -> color {
        return super.surface(geo)
    }
}`)

	_, err := Compile(src, CompileOptions{Targets: []Target{}})
	if err == nil {
		t.Fatal("Compile succeeded, want diagnostic error")
	}
	var ce *CompileError
	if !errors.As(err, &ce) {
		t.Fatalf("error type = %T, want *CompileError", err)
	}
	d := ce.Diagnostics[0]
	if d.Code != "SEL2003" {
		t.Fatalf("diagnostic code = %s, want SEL2003", d.Code)
	}
	if d.Range.Start.Line != 3 || d.Range.Start.Column != 16 {
		t.Fatalf("range start = %d:%d, want 3:16", d.Range.Start.Line, d.Range.Start.Column)
	}
	if !strings.Contains(d.Message, "super.surface used in a material with no parent") {
		t.Fatalf("diagnostic message = %q", d.Message)
	}
}

func TestCompileErrorRejectsReservedNames(t *testing.T) {
	src := []byte(`material Bad {
    param var : float
    surface(geo) -> color { return rgb(var, var, var) }
}`)

	_, err := Compile(src, CompileOptions{Targets: []Target{}})
	if err == nil {
		t.Fatal("Compile succeeded, want diagnostic error")
	}
	var ce *CompileError
	if !errors.As(err, &ce) {
		t.Fatalf("error type = %T, want *CompileError", err)
	}
	d := ce.Diagnostics[0]
	if d.Code != "SEL1004" {
		t.Fatalf("diagnostic code = %s, want SEL1004", d.Code)
	}
	if d.Range.Start.Line != 2 || d.Range.Start.Column != 5 {
		t.Fatalf("range start = %d:%d, want 2:5", d.Range.Start.Line, d.Range.Start.Column)
	}
	if !strings.Contains(d.Message, `param "var" is reserved for WGSL binding keyword`) {
		t.Fatalf("diagnostic message = %q", d.Message)
	}
}

// TestCompileProgramRejectsVertexHookOnNonMesh verifies the non-mesh kinds still
// reject author vertex() hooks (mesh/general supports them as of B4).
func TestCompileProgramRejectsVertexHookOnNonMesh(t *testing.T) {
	_, err := CompileProgram(hir.Program{Materials: []hir.Material{{
		Name: "Bad",
		Kind: hir.KindPoints,
		Vertex: &hir.Func{
			Span:   hir.Span{Start: hir.Position{Line: 2, Column: 5}},
			Geo:    "geo",
			Result: hir.Call{Func: "vec4f", Args: []hir.Expr{hir.Lit{Value: 0}, hir.Lit{Value: 0}, hir.Lit{Value: 0}, hir.Lit{Value: 1}}},
		},
		Surface: hir.Func{
			Geo:    "geo",
			Result: hir.Call{Func: "rgb", Args: []hir.Expr{hir.Lit{Value: 1}, hir.Lit{Value: 1}, hir.Lit{Value: 1}}},
		},
	}}}, CompileOptions{Targets: []Target{}})
	if err == nil {
		t.Fatal("CompileProgram succeeded, want diagnostic error")
	}
	var ce *CompileError
	if !errors.As(err, &ce) {
		t.Fatalf("error type = %T, want *CompileError", err)
	}
	d := ce.Diagnostics[0]
	if d.Code != "SEL1005" {
		t.Fatalf("diagnostic code = %s, want SEL1005", d.Code)
	}
	if d.Range.Start.Line != 2 || d.Range.Start.Column != 5 {
		t.Fatalf("range start = %d:%d, want 2:5", d.Range.Start.Line, d.Range.Start.Column)
	}
	if !strings.Contains(d.Message, "not supported in points") {
		t.Fatalf("diagnostic message = %q", d.Message)
	}
}

// TestCompileUniformBlockOffsetsMatchEmittedWGSLOrderForScalarAfterArray is an
// end-to-end regression test for a real bug: bindings.ComputeUniformBlock used
// to pack uniform-block fields in raw declaration order, but every backend
// emitter (WGSL/GLSL/GLES/Metal) renders scalars/vectors/matrices first and
// fixed-size arrays last (see emit/wgsl/wgsl.go emitMesh, which writes
// m.Uniforms then m.ArrayUniforms into one struct). A material declaring a
// scalar AFTER an array — like this one's "b" — used to get a descriptor
// whose "b" offset landed after "arr", even though the emitted struct puts it
// before. This locks the offsets to agree with the emitted struct's actual
// member order.
func TestCompileUniformBlockOffsetsMatchEmittedWGSLOrderForScalarAfterArray(t *testing.T) {
	src := []byte(`material ScalarAfterArray {
    param a : float
    param arr : array<vec4, 4>
    param b : float

    surface(geo) -> color {
        let item = arr[0i]
        return rgb(a + b, item.x, item.y)
    }
}`)
	res, err := Compile(src, CompileOptions{Targets: []Target{TargetWGSL}})
	if err != nil {
		t.Fatal(err)
	}

	wgslArtifact, ok := res.Artifact(TargetWGSL)
	if !ok {
		t.Fatal("missing WGSL artifact")
	}

	// Confirm the emitted struct really does put a,b (scalars) before arr
	// (array) — pins the emitter behavior this test's invariant depends on.
	idxA := strings.Index(wgslArtifact.Source, "\n  a : f32,\n")
	idxB := strings.Index(wgslArtifact.Source, "\n  b : f32,\n")
	idxArr := strings.Index(wgslArtifact.Source, "\n  arr : array<vec4<f32>, 4>,\n")
	if idxA < 0 || idxB < 0 || idxArr < 0 {
		t.Fatalf("could not locate uniform members in emitted WGSL:\n%s", wgslArtifact.Source)
	}
	if !(idxA < idxArr && idxB < idxArr) {
		t.Fatalf("expected emitted WGSL struct to order scalars a,b before array arr; got a=%d b=%d arr=%d in:\n%s", idxA, idxB, idxArr, wgslArtifact.Source)
	}

	// The compiled descriptor's byte offsets must agree with that emitted
	// order: a and b must be offset before arr.
	offsets := map[string]int{}
	for _, f := range res.Layout.UniformBlock.Fields {
		offsets[f.Name] = f.Offset
	}
	oa, ok := offsets["a"]
	if !ok {
		t.Fatal("descriptor missing field a")
	}
	ob, ok := offsets["b"]
	if !ok {
		t.Fatal("descriptor missing field b")
	}
	oarr, ok := offsets["arr"]
	if !ok {
		t.Fatal("descriptor missing field arr")
	}
	if !(oa < oarr && ob < oarr) {
		t.Fatalf("descriptor offsets disagree with emitted struct order: a@%d b@%d arr@%d (want a,b before arr)", oa, ob, oarr)
	}
}

func hasUniform(res Result, name string) bool {
	for _, u := range res.Module.Uniforms {
		if u.Name == name {
			return true
		}
	}
	return false
}

// A render material's statefield read must be TEXTURE-backed on WGSL, and the layout must
// say so, because the host binds the resource the layout names.
//
// Render materials tap stateAt() in dependent chains -- each coordinate derived from the
// value just read -- so the loads are scattered. Through a storage buffer they bypass the
// texture cache entirely, and past a certain grid size the working set stops fitting: the
// gosx water demo's surface shader burned ~16ms/frame on this, while the SAME shader on
// WebGL2 -- which has no storage buffers and so always sampled a texture -- never degraded
// at all. WebGL was right; WGSL was the outlier.
//
// Feedback materials keep their storage buffers: they WRITE the out buffer, and their
// reads are coherent neighbour taps rather than dependent chains.
func TestWGSLStatefieldReadIsTextureBackedForRenderMaterials(t *testing.T) {
	src := []byte(`material StateRead kind mesh {
    param gridResolution : float = 64.0
    state height
    varying vUv : vec2
    vertex() -> vec4 {
        let uv = vec2f(0.5, 0.5)
        let info = stateAt(uv)
        vUv = uv
        return vec4f(info.x, 0.0, 0.0, 1.0)
    }
    surface(geo) -> color {
        let s = stateAt(geo.vUv)
        return vec4f(s.x, s.y, s.z, 1.0)
    }
}`)
	res, err := Compile(src, CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}

	var wgsl string
	for _, a := range res.Artifacts {
		if a.Target == TargetWGSL {
			wgsl = a.Source
		}
	}
	if wgsl == "" {
		t.Fatal("no WGSL artifact")
	}

	if strings.Contains(wgsl, "var<storage, read> _inState") {
		t.Errorf("render statefield read is a storage buffer; it must be a texture:\n%s", wgsl)
	}
	if !strings.Contains(wgsl, "var _inState : texture_2d<f32>") {
		t.Errorf("render statefield is not bound as texture_2d<f32>:\n%s", wgsl)
	}
	// textureLoad, not textureSample: integer texel selection is bit-identical to the
	// flat-index read it replaces, and it carries no implicit derivative, so it stays
	// legal in the vertex stage -- where a mesh material displaces geometry by the
	// heightfield.
	if !strings.Contains(wgsl, "textureLoad(_inState") {
		t.Errorf("stateAt() does not lower to textureLoad:\n%s", wgsl)
	}
	if strings.Contains(wgsl, "textureSample(_inState") {
		t.Errorf("stateAt() must not lower to textureSample (needs a sampler; illegal in vertex):\n%s", wgsl)
	}

	if len(res.Layout.States) != 1 {
		t.Fatalf("layout states = %d, want 1", len(res.Layout.States))
	}
	st := res.Layout.States[0]
	if st.WGSL.InKind != "texture" {
		t.Errorf("layout WGSL.InKind = %q, want %q -- the host binds what the layout names", st.WGSL.InKind, "texture")
	}
	if st.WGSL.OutBinding >= 0 {
		t.Errorf("render statefield has an out binding (%d); it is read-only", st.WGSL.OutBinding)
	}
	// WebGL sampled a texture all along; that must not have changed.
	if st.GL.Uniform != "stateTex" {
		t.Errorf("GL state uniform = %q, want stateTex", st.GL.Uniform)
	}
}

// break exits the innermost for loop, on every backend.
//
// Without it a bounded search must be a fixed-trip loop with a predicated body, so every
// lane grinds through every iteration even after it has the answer. On a GPU that is not a
// stylistic loss: a ray-march's trip count is data-dependent, so a warp whose lanes finish
// at different steps pays the WORST lane's cost across all of them. The gosx water demo's
// surface shader marched 30 steps x 64 segments per fragment with no way out, and its cost
// swung with how disturbed the water was purely because disturbance made the lanes diverge.
func TestBreakExitsLoopOnEveryBackend(t *testing.T) {
	src := []byte(`material BreakTest kind post {
    surface(geo) -> color {
        var acc = 0.0
        var done = 0.0
        for (var i = 0i; i < 30i; i = i + 1i) {
            if (done > 0.5) { break }
            acc = acc + 0.01
            if (acc > 0.1) { done = 1.0 }
        }
        return vec4f(acc, acc, acc, 1.0)
    }
}`)
	res, err := Compile(src, CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	emitted := 0
	for _, a := range res.Artifacts {
		// Compile returns empty GLSL/GLES artifacts for a post material (pre-existing;
		// those targets are produced through the emit path directly). Assert on the
		// artifacts that actually carry source.
		if a.Source == "" {
			continue
		}
		emitted++
		if !strings.Contains(a.Source, "break;") {
			t.Errorf("%s: break not emitted\n%s", a.Target, a.Source)
		}
	}
	if emitted == 0 {
		t.Fatal("no artifact carried source; the assertion above proved nothing")
	}
}

// break outside a loop is a diagnostic, not invalid backend code.
func TestBreakOutsideLoopIsRejected(t *testing.T) {
	src := []byte(`material BadBreak kind post {
    surface(geo) -> color {
        break
        return vec4f(1.0, 1.0, 1.0, 1.0)
    }
}`)
	if _, err := Compile(src, CompileOptions{}); err == nil {
		t.Fatal("break outside a loop compiled; want a diagnostic")
	}
}

// TestCompileRejectsUnreachableReturn is the regression test for a real
// silent miscompile: parse.blockBody assigned result = e for every
// return_stmt with no check, so a second sibling return silently overwrote
// the first one, and Selena emitted a valid-but-wrong shader with exit code
// 0 and no diagnostic. Two sibling top-level returns must now fail to
// compile, anchored at the second (unreachable) return.
func TestCompileRejectsUnreachableReturn(t *testing.T) {
	src := []byte(`material TwoReturn kind mesh {
    surface(geo) -> color {
        return rgb(1.0, 0.0, 0.0)
        return rgb(0.0, 1.0, 0.0)
    }
}`)
	_, err := Compile(src, CompileOptions{Targets: []Target{}})
	if err == nil {
		t.Fatal("Compile succeeded with two sibling returns; want a diagnostic, not a silent miscompile")
	}
	var ce *CompileError
	if !errors.As(err, &ce) {
		t.Fatalf("error type = %T, want *CompileError", err)
	}
	d := ce.Diagnostics[0]
	if !strings.Contains(d.Message, "unreachable statement after return") {
		t.Fatalf("diagnostic message = %q, want it to name the unreachable statement", d.Message)
	}
	if d.Range.Start.Line != 4 {
		t.Fatalf("range start line = %d, want 4 (the second, unreachable return)", d.Range.Start.Line)
	}
}

// TestCompileRejectsStatementAfterReturn covers the other shape of the same
// defect: a statement between two returns used to be hoisted into executed
// code (dead code that ran anyway) while only the final return's value
// escaped. Any statement after a return must fail to compile, not just a
// second return.
func TestCompileRejectsStatementAfterReturn(t *testing.T) {
	src := []byte(`material FlatReturn kind mesh {
    surface(geo) -> color {
        return rgb(1.0, 0.0, 0.0)
        let later = 0.25
        return rgb(0.0, later, 0.0)
    }
}`)
	_, err := Compile(src, CompileOptions{Targets: []Target{}})
	if err == nil {
		t.Fatal("Compile succeeded with a let after a return; want a diagnostic, not silently hoisted dead code")
	}
	var ce *CompileError
	if !errors.As(err, &ce) {
		t.Fatalf("error type = %T, want *CompileError", err)
	}
	d := ce.Diagnostics[0]
	if !strings.Contains(d.Message, "unreachable statement after return") {
		t.Fatalf("diagnostic message = %q, want it to name the unreachable statement", d.Message)
	}
	if d.Range.Start.Line != 4 {
		t.Fatalf("range start line = %d, want 4 (the let after the return)", d.Range.Start.Line)
	}
}

// TestCompileSingleReturnStillCompiles pins the non-regressed case: a
// surface body ending in exactly one return must keep compiling and emitting
// the same shader as before Defect A's fix.
func TestCompileSingleReturnStillCompiles(t *testing.T) {
	src := []byte(`material OneReturn kind mesh {
    surface(geo) -> color {
        return rgb(1.0, 0.0, 0.0)
    }
}`)
	res, err := Compile(src, CompileOptions{Targets: []Target{TargetWGSL}})
	if err != nil {
		t.Fatalf("single return should still compile: %v", err)
	}
	wgsl, ok := res.Artifact(TargetWGSL)
	if !ok {
		t.Fatal("missing WGSL artifact")
	}
	if !strings.Contains(wgsl.Source, "return vec4<f32>(vec3<f32>(1.0, 0.0, 0.0), 1.0);") {
		t.Fatalf("WGSL fragment does not return the expected color:\n%s", wgsl.Source)
	}
}

// fragmentSource returns the fragment-stage source for a compiled artifact:
// Source for the single-file backends (WGSL, Metal), Fragment for the
// split-file backends (GLSL, GLES).
func fragmentSource(a Artifact) string {
	if a.Source != "" {
		return a.Source
	}
	return a.Fragment
}

// TestCompileEarlyReturnInIfProducesTwoReturnSites is the Phase 1 acceptance
// test: `if (c) { return X }` followed by a trailing `return Y` must compile
// on every backend, with X's return reachable only inside the if branch and
// Y's return reachable only after it — a real conditional branch with two
// distinct return sites, not a hoisted/overwritten single value (the exact
// silent miscompile Phase 0 closed for the flat-sibling-return shape).
func TestCompileEarlyReturnInIfProducesTwoReturnSites(t *testing.T) {
	src := []byte(`material EarlyReturnIf kind mesh {
    param cutoff : float = 0.5

    surface(geo) -> color {
        if (geo.uv.x < cutoff) {
            return rgb(1.0, 0.0, 0.0)
        }
        return rgb(0.0, 1.0, 0.0)
    }
}`)
	res, err := Compile(src, CompileOptions{Targets: AllTargets()})
	if err != nil {
		t.Fatalf("return inside if followed by a trailing return should compile: %v", err)
	}
	for _, target := range AllTargets() {
		a, ok := res.Artifact(target)
		if !ok {
			t.Fatalf("missing %s artifact", target)
		}
		frag := fragmentSource(a)
		if !strings.Contains(frag, "if (") {
			t.Fatalf("%s fragment has no branch:\n%s", target, frag)
		}
		if !strings.Contains(frag, "1.0, 0.0, 0.0") {
			t.Fatalf("%s fragment is missing the early (red) return:\n%s", target, frag)
		}
		if !strings.Contains(frag, "0.0, 1.0, 0.0") {
			t.Fatalf("%s fragment is missing the trailing (green) return:\n%s", target, frag)
		}
	}
}

// TestCompileEarlyReturnInForLoop covers a conditional return nested inside a
// for body, alongside an unrelated author `break` in the same loop — pinning
// that return exits the whole surface (not just the loop) while break only
// exits the loop, and that the two don't interfere with each other's lowering
// (loopDepth tracking, in particular).
func TestCompileEarlyReturnInForLoop(t *testing.T) {
	src := []byte(`material EarlyReturnLoop kind mesh {
    param limit : float = 3.0

    surface(geo) -> color {
        var hits = 0.0
        for (var i = 0.0; i < 8.0; i = i + 1.0) {
            if (i > limit) {
                return rgb(1.0, 0.0, 0.0)
            }
            hits = hits + 1.0
            if (hits > 6.0) {
                break
            }
        }
        return rgb(0.0, hits / 8.0, 0.0)
    }
}`)
	res, err := Compile(src, CompileOptions{Targets: AllTargets()})
	if err != nil {
		t.Fatalf("early return inside a for body should compile: %v", err)
	}
	for _, target := range AllTargets() {
		a, ok := res.Artifact(target)
		if !ok {
			t.Fatalf("missing %s artifact", target)
		}
		frag := fragmentSource(a)
		if !strings.Contains(frag, "for (") {
			t.Fatalf("%s fragment has no for loop:\n%s", target, frag)
		}
		if !strings.Contains(frag, "break;") {
			t.Fatalf("%s fragment is missing the author break:\n%s", target, frag)
		}
		if !strings.Contains(frag, "1.0, 0.0, 0.0") {
			t.Fatalf("%s fragment is missing the early (red) return:\n%s", target, frag)
		}
	}
}

// TestCompileFeedbackEarlyReturnWritesOutStateThenReturns covers the one
// backend shape that cannot use a plain `return val;`: WGSL/Metal feedback
// compute kernels return void, so an early ReturnCF must write outState then
// bare-return (emit/wgsl and emit/metal's feedback ReturnFn), matching the
// existing unconditional write at the end of the kernel.
func TestCompileFeedbackEarlyReturnWritesOutStateThenReturns(t *testing.T) {
	src := []byte(`material FeedbackEarlyReturn kind feedback {
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
	res, err := Compile(src, CompileOptions{Targets: []Target{TargetWGSL, TargetMetal}})
	if err != nil {
		t.Fatalf("early return in a feedback body should compile: %v", err)
	}
	wgsl, ok := res.Artifact(TargetWGSL)
	if !ok {
		t.Fatal("missing WGSL artifact")
	}
	if !strings.Contains(wgsl.Source, "outState[cellIndex] = vec4<f32>(0.0, 0.0, 0.0, 0.0); return;") {
		t.Fatalf("WGSL compute kernel does not write outState before the early return:\n%s", wgsl.Source)
	}
	metalArt, ok := res.Artifact(TargetMetal)
	if !ok {
		t.Fatal("missing Metal artifact")
	}
	if !strings.Contains(metalArt.Source, "outState[cellIndex] = float4(0.0, 0.0, 0.0, 0.0); return;") {
		t.Fatalf("Metal compute kernel does not write outState before the early return:\n%s", metalArt.Source)
	}
}

// TestCompileVertexEarlyReturnRejected pins the vertex() restriction the
// design deliberately keeps: a varying assigned after an early return would
// be undefined, so authored vertex() bodies reject Return at lowering time
// even though the parser now accepts it structurally in every if/for body.
func TestCompileVertexEarlyReturnRejected(t *testing.T) {
	src := []byte(`material VertexEarlyReturn {
    vertex(geo) -> vec4 {
        if (geo.position.x > 0.0) {
            return mvp * vec4f(geo.position, 1.0)
        }
        return mvp * vec4f(geo.position, 1.0)
    }

    surface(geo) -> color {
        return rgb(1.0, 1.0, 1.0)
    }
}`)
	_, err := Compile(src, CompileOptions{Targets: []Target{}})
	if err == nil {
		t.Fatal("Compile succeeded with an early return in vertex(); want a diagnostic")
	}
	var ce *CompileError
	if !errors.As(err, &ce) {
		t.Fatalf("error type = %T, want *CompileError", err)
	}
	d := ce.Diagnostics[0]
	if !strings.Contains(d.Message, "vertex()") {
		t.Fatalf("diagnostic message = %q, want it to name vertex()", d.Message)
	}
}

// TestCompileSuperSurfaceChildEarlyReturnCompiles covers the composition
// combination the design supports: the CHILD may use an early return whose
// value calls super.surface(...), as long as the PARENT itself has no
// control flow (the existing Let-only inline gate; see
// TestCompileSuperSurfaceRejectsEarlyReturnInParent for the combination it
// rejects).
func TestCompileSuperSurfaceChildEarlyReturnCompiles(t *testing.T) {
	src := []byte(`material Base {
    param baseColor : color
    surface(geo) -> color {
        return baseColor
    }
}
material Tinted extends Base {
    param tint : color
    surface(geo) -> color {
        if (tint.r > 0.5) {
            return super.surface(geo) * tint
        }
        return baseColor
    }
}`)
	res, err := Compile(src, CompileOptions{Material: "Tinted", Targets: []Target{TargetWGSL}})
	if err != nil {
		t.Fatalf("child early return calling super.surface should compile: %v", err)
	}
	wgsl, ok := res.Artifact(TargetWGSL)
	if !ok {
		t.Fatal("missing WGSL artifact")
	}
	if !strings.Contains(wgsl.Source, "if (") {
		t.Fatalf("WGSL fragment has no branch:\n%s", wgsl.Source)
	}
}

// TestCompileSuperSurfaceRejectsEarlyReturnInParent covers the combination
// the design deliberately does NOT support: a PARENT surface with an early
// return has no statement stream super.surface(...) can splice a child's
// usage into (inline.go's Let-only gate), so it stays rejected — with a
// diagnostic that names control flow / early return instead of Phase 0's
// retired "B2a limitation" wording.
func TestCompileSuperSurfaceRejectsEarlyReturnInParent(t *testing.T) {
	src := []byte(`material Base {
    param baseColor : color
    surface(geo) -> color {
        if (baseColor.r > 0.5) {
            return baseColor
        }
        return rgb(0.0, 0.0, 0.0)
    }
}
material Tinted extends Base {
    param tint : color
    surface(geo) -> color {
        return super.surface(geo) * tint
    }
}`)
	_, err := Compile(src, CompileOptions{Material: "Tinted", Targets: []Target{}})
	if err == nil {
		t.Fatal("Compile succeeded with an early return in the parent surface; want a diagnostic")
	}
	var ce *CompileError
	if !errors.As(err, &ce) {
		t.Fatalf("error type = %T, want *CompileError", err)
	}
	d := ce.Diagnostics[0]
	if !strings.Contains(d.Message, "early return") {
		t.Fatalf("diagnostic message = %q, want it to name early returns", d.Message)
	}
	if strings.Contains(d.Message, "B2a") {
		t.Fatalf("diagnostic message = %q, must not reference the retired B2a limitation label", d.Message)
	}
}

// TestCompileRejectsDiscardAfterReturn closes a deduction from the Phase 0
// investigation (never separately executed): a discard statement placed
// after a return used to be hoisted into executed code by the same defect
// two sibling returns exploited. Phase 0's generic "return must be last"
// check covers any statement kind, including discard — verify it here.
func TestCompileRejectsDiscardAfterReturn(t *testing.T) {
	src := []byte(`material DiscardAfterReturn kind mesh {
    surface(geo) -> color {
        return rgb(1.0, 0.0, 0.0)
        discard
    }
}`)
	_, err := Compile(src, CompileOptions{Targets: []Target{}})
	if err == nil {
		t.Fatal("Compile succeeded with a discard after a return; want a diagnostic, not silently hoisted dead code")
	}
	var ce *CompileError
	if !errors.As(err, &ce) {
		t.Fatalf("error type = %T, want *CompileError", err)
	}
	d := ce.Diagnostics[0]
	if !strings.Contains(d.Message, "unreachable statement after return") {
		t.Fatalf("diagnostic message = %q, want it to name the unreachable statement", d.Message)
	}
}

// TestCompileRejectsUnreachableStatementAfterNestedReturn extends Phase 0's
// protection to nested blocks: a statement after a return inside an if body
// is exactly as unreachable as one after a top-level return, and must be
// rejected the same way. This is the case Phase 1 could have accidentally
// weakened by legalizing return-in-if; it must not have.
func TestCompileRejectsUnreachableStatementAfterNestedReturn(t *testing.T) {
	src := []byte(`material UnreachableInIf kind mesh {
    surface(geo) -> color {
        if (geo.uv.x < 0.5) {
            return rgb(1.0, 0.0, 0.0)
            let dead = 0.25
        }
        return rgb(0.0, 1.0, 0.0)
    }
}`)
	_, err := Compile(src, CompileOptions{Targets: []Target{}})
	if err == nil {
		t.Fatal("Compile succeeded with a let after a nested return; want a diagnostic")
	}
	var ce *CompileError
	if !errors.As(err, &ce) {
		t.Fatalf("error type = %T, want *CompileError", err)
	}
	d := ce.Diagnostics[0]
	if !strings.Contains(d.Message, "unreachable statement after return") {
		t.Fatalf("diagnostic message = %q, want it to name the unreachable statement", d.Message)
	}
	if d.Range.Start.Line != 5 {
		t.Fatalf("range start line = %d, want 5 (the let after the nested return)", d.Range.Start.Line)
	}
}

// TestCompileUnreachableReturnDiagnosticNoLongerClaimsSingleReturn locks in
// the Phase 0 diagnostic narrowing Phase 1 requires: the message used to say
// "a surface body may only return once, as its final statement", which
// became false once early returns shipped. The message must still identify
// the unreachable statement, but must no longer make that now-false claim.
func TestCompileUnreachableReturnDiagnosticNoLongerClaimsSingleReturn(t *testing.T) {
	src := []byte(`material TwoReturn kind mesh {
    surface(geo) -> color {
        return rgb(1.0, 0.0, 0.0)
        return rgb(0.0, 1.0, 0.0)
    }
}`)
	_, err := Compile(src, CompileOptions{Targets: []Target{}})
	if err == nil {
		t.Fatal("Compile succeeded with two sibling returns; want a diagnostic")
	}
	var ce *CompileError
	if !errors.As(err, &ce) {
		t.Fatalf("error type = %T, want *CompileError", err)
	}
	d := ce.Diagnostics[0]
	if strings.Contains(d.Message, "may only return once") {
		t.Fatalf("diagnostic message = %q, must not claim a body may only return once now that early returns are supported", d.Message)
	}
}
