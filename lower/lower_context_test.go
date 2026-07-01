package lower

import (
	"testing"

	"m31labs.dev/selena/bindings"
	"m31labs.dev/selena/emit/gles"
	"m31labs.dev/selena/emit/glsl"
	"m31labs.dev/selena/emit/metal"
	"m31labs.dev/selena/emit/wgsl"
	"m31labs.dev/selena/ir"
	"m31labs.dev/selena/parse"
)

// TestContextIsCodegenTransparent is the codegen-transparency proof for the
// "context uniform" concept (design §3.3): a material declaring a field in a
// `context { ... }` block must emit BYTE-IDENTICAL WGSL/GLSL/GLES/Metal to the
// same material with that field declared as an ordinary `param` — context
// fields are ordinary uniform-block members, differing only in Layout
// provenance metadata (Class + Layout.Context), never in the generated shader
// text. This is what lets every emitter stay untouched (C8).
func TestContextIsCodegenTransparent(t *testing.T) {
	const paramSrc = `material ContextParityParam {
    param intensity : float = 1.0
    param time : float
    surface(geo) -> color {
        let v = intensity + time
        return rgb(v, v, v)
    }
}`
	const contextSrc = `material ContextParityContext {
    param intensity : float = 1.0
    context {
        time : float
    }
    surface(geo) -> color {
        let v = intensity + time
        return rgb(v, v, v)
    }
}`

	paramMod, paramLayout := lowerSrc(t, paramSrc)
	ctxMod, ctxLayout := lowerSrc(t, contextSrc)

	// --- shader text: byte-identical across all four backends ---
	paramWGSL, err := wgsl.Emit(paramMod)
	if err != nil {
		t.Fatalf("wgsl.Emit(param): %v", err)
	}
	ctxWGSL, err := wgsl.Emit(ctxMod)
	if err != nil {
		t.Fatalf("wgsl.Emit(context): %v", err)
	}
	if paramWGSL != ctxWGSL {
		t.Fatalf("WGSL differs between param and context forms:\n--- param ---\n%s\n--- context ---\n%s", paramWGSL, ctxWGSL)
	}

	paramMetal, err := metal.Emit(paramMod)
	if err != nil {
		t.Fatalf("metal.Emit(param): %v", err)
	}
	ctxMetal, err := metal.Emit(ctxMod)
	if err != nil {
		t.Fatalf("metal.Emit(context): %v", err)
	}
	if paramMetal != ctxMetal {
		t.Fatalf("Metal differs between param and context forms:\n--- param ---\n%s\n--- context ---\n%s", paramMetal, ctxMetal)
	}

	paramGLSLVert, paramGLSLFrag, err := glsl.Emit(paramMod)
	if err != nil {
		t.Fatalf("glsl.Emit(param): %v", err)
	}
	ctxGLSLVert, ctxGLSLFrag, err := glsl.Emit(ctxMod)
	if err != nil {
		t.Fatalf("glsl.Emit(context): %v", err)
	}
	if paramGLSLVert != ctxGLSLVert {
		t.Fatalf("GLSL vertex differs between param and context forms:\n--- param ---\n%s\n--- context ---\n%s", paramGLSLVert, ctxGLSLVert)
	}
	if paramGLSLFrag != ctxGLSLFrag {
		t.Fatalf("GLSL fragment differs between param and context forms:\n--- param ---\n%s\n--- context ---\n%s", paramGLSLFrag, ctxGLSLFrag)
	}

	paramGLESVert, paramGLESFrag, err := gles.Emit(paramMod)
	if err != nil {
		t.Fatalf("gles.Emit(param): %v", err)
	}
	ctxGLESVert, ctxGLESFrag, err := gles.Emit(ctxMod)
	if err != nil {
		t.Fatalf("gles.Emit(context): %v", err)
	}
	if paramGLESVert != ctxGLESVert {
		t.Fatalf("GLES vertex differs between param and context forms:\n--- param ---\n%s\n--- context ---\n%s", paramGLESVert, ctxGLESVert)
	}
	if paramGLESFrag != ctxGLESFrag {
		t.Fatalf("GLES fragment differs between param and context forms:\n--- param ---\n%s\n--- context ---\n%s", paramGLESFrag, ctxGLESFrag)
	}

	// --- descriptor: the param form never tags "time" as context ---
	paramTime, ok := fieldByName(paramLayout.UniformBlock.Fields, "time")
	if !ok {
		t.Fatal("param form: field \"time\" not found in uniform block")
	}
	if paramTime.Class != "" {
		t.Fatalf("param form: time.Class = %q, want \"\" (empty)", paramTime.Class)
	}
	if len(paramLayout.Context) != 0 {
		t.Fatalf("param form: Layout.Context = %v, want empty", paramLayout.Context)
	}

	// --- descriptor: the context form DOES tag "time" as context ---
	ctxTime, ok := fieldByName(ctxLayout.UniformBlock.Fields, "time")
	if !ok {
		t.Fatal("context form: field \"time\" not found in uniform block")
	}
	if ctxTime.Class != "context" {
		t.Fatalf("context form: time.Class = %q, want \"context\"", ctxTime.Class)
	}
	if len(ctxLayout.Context) != 1 || ctxLayout.Context[0] != "time" {
		t.Fatalf("context form: Layout.Context = %v, want [\"time\"]", ctxLayout.Context)
	}

	// --- descriptor: the ordinary param "intensity" is never tagged context,
	// in EITHER form (context tagging must not leak onto sibling params) ---
	for _, name := range []string{"context form", "param form"} {
		layout := ctxLayout
		if name == "param form" {
			layout = paramLayout
		}
		f, ok := fieldByName(layout.UniformBlock.Fields, "intensity")
		if !ok {
			t.Fatalf("%s: field \"intensity\" not found in uniform block", name)
		}
		if f.Class != "" {
			t.Fatalf("%s: intensity.Class = %q, want \"\" (empty)", name, f.Class)
		}
	}

	// --- byte offsets are identical between the two forms too (context fields
	// occupy the exact same std140 slot a param of the same type/position would) ---
	if paramTime.Offset != ctxTime.Offset || paramTime.Size != ctxTime.Size {
		t.Fatalf("time field layout differs: param=%+v context=%+v", paramTime, ctxTime)
	}
}

// TestContextArrayFieldStd140Layout proves an array-typed context field
// (`context { spheres : array<vec4, 32> }`) lays out identically to the
// equivalent array param at std140 stride (16 bytes/element, §3.1/§3.2), and
// is tagged Class:"context" + listed in Layout.Context — while the array
// param form is not.
func TestContextArrayFieldStd140Layout(t *testing.T) {
	const paramSrc = `material ArrayParitySpheresParam {
    param spheres : array<vec4, 32>
    surface(geo) -> color {
        let s = spheres[0i]
        return s.rgb
    }
}`
	const contextSrc = `material ArrayParitySpheresContext {
    context {
        spheres : array<vec4, 32>
    }
    surface(geo) -> color {
        let s = spheres[0i]
        return s.rgb
    }
}`

	_, paramLayout := lowerSrc(t, paramSrc)
	_, ctxLayout := lowerSrc(t, contextSrc)

	paramSpheres, ok := fieldByName(paramLayout.UniformBlock.Fields, "spheres")
	if !ok {
		t.Fatal("param form: field \"spheres\" not found in uniform block")
	}
	ctxSpheres, ok := fieldByName(ctxLayout.UniformBlock.Fields, "spheres")
	if !ok {
		t.Fatal("context form: field \"spheres\" not found in uniform block")
	}

	// std140 array layout: mvp(64) + normalMatrix(48) = 112, already 16-aligned,
	// so the array starts at offset 112; 32 elements * 16-byte stride = 512 bytes.
	for name, f := range map[string]bindings.Field{"param": paramSpheres, "context": ctxSpheres} {
		if f.Offset != 112 {
			t.Errorf("%s form: spheres.Offset = %d, want 112", name, f.Offset)
		}
		if f.Count != 32 {
			t.Errorf("%s form: spheres.Count = %d, want 32", name, f.Count)
		}
		if f.Stride != 16 {
			t.Errorf("%s form: spheres.Stride = %d, want 16 (std140 array stride)", name, f.Stride)
		}
		if f.Size != 32*16 {
			t.Errorf("%s form: spheres.Size = %d, want %d", name, f.Size, 32*16)
		}
	}

	if paramLayout.UniformBlock.Size != ctxLayout.UniformBlock.Size {
		t.Fatalf("uniform block size differs: param=%d context=%d", paramLayout.UniformBlock.Size, ctxLayout.UniformBlock.Size)
	}

	if paramSpheres.Class != "" {
		t.Errorf("param form: spheres.Class = %q, want \"\" (empty)", paramSpheres.Class)
	}
	if len(paramLayout.Context) != 0 {
		t.Errorf("param form: Layout.Context = %v, want empty", paramLayout.Context)
	}

	if ctxSpheres.Class != "context" {
		t.Errorf("context form: spheres.Class = %q, want \"context\"", ctxSpheres.Class)
	}
	if len(ctxLayout.Context) != 1 || ctxLayout.Context[0] != "spheres" {
		t.Errorf("context form: Layout.Context = %v, want [\"spheres\"]", ctxLayout.Context)
	}
}

// lowerSrc parses and lowers a single-material .sel source, failing the test
// on any error.
func lowerSrc(t *testing.T, src string) (mod ir.Module, layout bindings.Layout) {
	t.Helper()
	m, err := parse.Material([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	irMod, l, err := Lower(m)
	if err != nil {
		t.Fatalf("lower: %v", err)
	}
	return irMod, l
}

func fieldByName(fields []bindings.Field, name string) (bindings.Field, bool) {
	for _, f := range fields {
		if f.Name == name {
			return f, true
		}
	}
	return bindings.Field{}, false
}
