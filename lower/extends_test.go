package lower

import (
	"errors"
	"strings"
	"testing"

	"m31labs.dev/selena/emit/wgsl"
	"m31labs.dev/selena/parse"
)

// TestResolveExtendsInheritsParentVertexStage pins Defect 2's fix: a child
// that extends a parent authoring vertex()/varying/state used to lose all
// three silently. resolveExtends only ever merged Params and inlined
// super.surface; `out := m` already carried the CHILD's own (nil/empty)
// Vertex/Varyings/States, and nothing copied the parent's over, so the
// compile reported success and the emitted shader silently reverted to the
// default (undisplaced) transform. A child that declares no vertex stage of
// its own now inherits the parent's vertex(), its varyings, and its
// statefield, so the parent's geometry work and stateAt(uv) read survive
// `extends` and super.surface keeps resolving the geo fields the inherited
// vertex stage produces.
func TestResolveExtendsInheritsParentVertexStage(t *testing.T) {
	src := `material RippleBase {
    param amp : float = 0.2

    state height

    varying worldPos : vec3

    vertex() -> vec4 {
        let fi = float(vertexIndex)
        let h  = stateAt(vec2f(0.0, 0.0)).x * amp
        let p  = vec3f(fi, h, 0.0)
        worldPos = p
        return mvp * vec4f(p, 1.0)
    }

    surface(geo) -> color {
        return rgb(geo.worldPos.x, geo.worldPos.y, geo.worldPos.z)
    }
}

material RippleTinted extends RippleBase {
    param tint : color = rgb(1.0, 0.8, 0.7)

    surface(geo) -> color {
        return super.surface(geo) * tint
    }
}`
	program, err := parse.Program([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	mod, layout, err := LowerProgram(program, 1)
	if err != nil {
		t.Fatalf("RippleTinted should inherit RippleBase's vertex stage: %v", err)
	}
	if !mod.VertexAuthored {
		t.Fatal("VertexAuthored = false, want true (inherited vertex() stage)")
	}
	if !mod.UsesVertexIndex {
		t.Fatal("UsesVertexIndex = false, want true (inherited procedural geometry)")
	}
	if len(mod.Varyings) != 1 || mod.Varyings[0].Name != "worldPos" {
		t.Fatalf("varyings = %+v, want inherited [worldPos]", mod.Varyings)
	}
	if mod.StateField != "height" {
		t.Fatalf("StateField = %q, want inherited \"height\"", mod.StateField)
	}
	if len(layout.States) != 1 || layout.States[0].Name != "height" {
		t.Fatalf("layout.States = %+v, want inherited [height]", layout.States)
	}

	src2, err := wgsl.Emit(mod)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(src2, "out.worldPos = p;") {
		t.Errorf("WGSL missing the inherited varying write:\n%s", src2)
	}
	if !strings.Contains(src2, "let h = (") || !strings.Contains(src2, "u.amp)") {
		t.Errorf("WGSL missing the inherited stateAt/amp displacement:\n%s", src2)
	}
	if !strings.Contains(src2, "return vec4<f32>((vec3<f32>(in.worldPos.x, in.worldPos.y, in.worldPos.z) * u.tint), 1.0);") {
		t.Errorf("WGSL fragment missing the child's super.surface(geo) * tint composition, reading the inherited varying:\n%s", src2)
	}
}

// TestResolveExtendsRejectsChildAndParentBothDeclaringVertex checks the other
// half of Defect 2: silently preferring either side when both a parent and a
// child declare a vertex() stage would just relocate the silent drop, so the
// combination is rejected with a diagnostic instead — mirroring the existing
// restriction on a super.surface parent with control flow (inline.go).
func TestResolveExtendsRejectsChildAndParentBothDeclaringVertex(t *testing.T) {
	src := `material BaseVertex {
    vertex() -> vec4 {
        return vec4f(0.0, 0.0, 0.0, 1.0)
    }
    surface(geo) -> color {
        return rgb(1.0, 1.0, 1.0)
    }
}

material ChildVertex extends BaseVertex {
    vertex() -> vec4 {
        return vec4f(1.0, 1.0, 1.0, 1.0)
    }
    surface(geo) -> color {
        return rgb(0.0, 0.0, 0.0)
    }
}`
	program, err := parse.Program([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = LowerProgram(program, 1)
	if err == nil {
		t.Fatal("child and parent both declaring vertex() lowered, want a diagnostic")
	}
	var de *DiagnosticError
	if !errors.As(err, &de) {
		t.Fatalf("error type = %T, want *DiagnosticError", err)
	}
	if de.Code != CodeUnsupportedFeat {
		t.Fatalf("code = %s, want %s", de.Code, CodeUnsupportedFeat)
	}
	want := `material "ChildVertex" declares vertex() and extends "BaseVertex", which also declares vertex(); a child material cannot override or merge a parent's vertex stage`
	if de.Message != want {
		t.Fatalf("message = %q, want %q", de.Message, want)
	}
}

// TestResolveExtendsLeavesSurfaceOnlyCompositionUnchanged is a regression
// guard for the common case (neither material declares a vertex stage): the
// new inheritance branch must not fire, and the default-transform mesh path
// (VertexAuthored = false) must still apply, matching testdata/conformance/
// extends.sel's shape.
func TestResolveExtendsLeavesSurfaceOnlyCompositionUnchanged(t *testing.T) {
	src := `material Base {
    param baseColor : color
    surface(geo) -> color {
        return baseColor
    }
}

material Tinted extends Base {
    param tint : color = rgb(1.0, 0.8, 0.7)
    surface(geo) -> color {
        return super.surface(geo) * tint
    }
}`
	program, err := parse.Program([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	mod, _, err := LowerProgram(program, 1)
	if err != nil {
		t.Fatal(err)
	}
	if mod.VertexAuthored {
		t.Fatal("VertexAuthored = true, want false (neither material declares vertex())")
	}
	if len(mod.Varyings) != 0 {
		t.Fatalf("varyings = %+v, want none", mod.Varyings)
	}
}
