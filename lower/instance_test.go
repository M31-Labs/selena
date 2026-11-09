package lower

import (
	"strings"
	"testing"

	"m31labs.dev/selena/bindings"
	"m31labs.dev/selena/ir"
	"m31labs.dev/selena/parse"
)

const instSurface = `surface(geo) -> color { return rgb(1.0, 1.0, 1.0) }`

func instMaterial(extra string) string {
	return "material M {\n" + extra + "\n" + instSurface + "\n}"
}

func TestInstanceAttributesLower(t *testing.T) {
	mod, layout, err := lowerTexelSrc(t, instMaterial(`
    instance rect : vec4
    instance tone : vec3
    instance spin : float
    instance slot : uint
    instance lane : int
    instance pair : vec2
    vertex() -> vec4 {
        let a = rect.xy + pair
        let k = float(slot) + float(lane) + float(instanceIndex) + spin + tone.x
        return mvp * vec4f(a, k, 1.0)
    }`))
	if err != nil {
		t.Fatal(err)
	}
	if len(mod.Attributes) != 0 {
		t.Errorf("attributes = %+v", mod.Attributes)
	}
	if !mod.UsesInstanceIndex || mod.UsesVertexIndex {
		t.Errorf("UsesInstanceIndex = %v, UsesVertexIndex = %v", mod.UsesInstanceIndex, mod.UsesVertexIndex)
	}
	wantTypes := []ir.Type{ir.Vec4, ir.Vec3, ir.Float, ir.Uint, ir.Int, ir.Vec2}
	for i, a := range mod.InstanceAttributes {
		if a.Type != wantTypes[i] {
			t.Errorf("instance %d (%s) type = %s, want %s", i, a.Name, a.Type, wantTypes[i])
		}
	}
	want := []bindings.InstanceAttribute{
		{Name: "rect", Location: 0, Type: "vec4", Offset: 0},
		{Name: "tone", Location: 1, Type: "vec3", Offset: 16},
		{Name: "spin", Location: 2, Type: "float", Offset: 28},
		{Name: "slot", Location: 3, Type: "uint", Offset: 32},
		{Name: "lane", Location: 4, Type: "int", Offset: 36},
		{Name: "pair", Location: 5, Type: "vec2", Offset: 40},
	}
	if len(layout.InstanceAttributes) != len(want) {
		t.Fatalf("instance attributes = %+v", layout.InstanceAttributes)
	}
	for i := range want {
		if layout.InstanceAttributes[i] != want[i] {
			t.Errorf("instance attribute %d = %+v, want %+v", i, layout.InstanceAttributes[i], want[i])
		}
	}
	if layout.InstanceStride != 48 || layout.InstanceStride%4 != 0 {
		t.Errorf("stride = %d, want 48", layout.InstanceStride)
	}
	// uint and int instance attributes have no GLSL ES 1.00 form, so the
	// descriptor does not ask for the extension that form needs.
	if got := layout.Requires.GLExtensions; len(got) != 0 {
		t.Errorf("requires.glExtensions = %v, want none", got)
	}
}

func TestInstanceIndexAloneNeedsNoDeclarations(t *testing.T) {
	mod, layout, err := lowerTexelSrc(t, instMaterial(`
    vertex() -> vec4 { return vec4f(float(instanceIndex), 0.0, 0.0, 1.0) }`))
	if err != nil {
		t.Fatal(err)
	}
	if !mod.UsesInstanceIndex || len(layout.InstanceAttributes) != 0 || layout.InstanceStride != 0 {
		t.Errorf("module %+v layout %+v", mod.UsesInstanceIndex, layout.InstanceAttributes)
	}
	if len(layout.Requires.GLExtensions) != 0 {
		t.Errorf("instanceIndex alone asked for %v", layout.Requires.GLExtensions)
	}
}

func TestInstanceIndexInsideControlFlowIsDetected(t *testing.T) {
	mod, _, err := lowerTexelSrc(t, instMaterial(`
    vertex() -> vec4 {
        var x = 0.0
        if (instanceIndex > 3u) {
            x = float(instanceIndex)
        }
        return vec4f(x, 0.0, 0.0, 1.0)
    }`))
	if err != nil {
		t.Fatal(err)
	}
	if !mod.UsesInstanceIndex {
		t.Error("instanceIndex inside an if was not detected")
	}
}

func TestInstanceAttributeInheritedThroughExtends(t *testing.T) {
	program, err := parse.Program([]byte(`material Base {
    instance rect : vec4
    vertex() -> vec4 { return mvp * vec4f(rect.xy, 0.0, 1.0) }
    surface(geo) -> color { return rgb(1.0, 0.0, 0.0) }
}
material Child extends Base {
    instance tint : float
    surface(geo) -> color { return rgb(0.0, 1.0, 0.0) }
}`))
	if err != nil {
		t.Fatal(err)
	}
	_, layout, err := LowerProgram(program, 1)
	if err != nil {
		t.Fatal(err)
	}
	// The parent's attributes come first, so the record layout starts with them.
	if len(layout.InstanceAttributes) != 2 || layout.InstanceAttributes[0].Name != "rect" ||
		layout.InstanceAttributes[1].Name != "tint" || layout.InstanceAttributes[1].Offset != 16 || layout.InstanceStride != 20 {
		t.Fatalf("instance attributes = %+v stride %d", layout.InstanceAttributes, layout.InstanceStride)
	}
}

func TestInstanceDiagnostics(t *testing.T) {
	cases := []struct {
		name, src string
		code      string
		contains  []string
	}{
		{"no vertex", instMaterial("instance a : vec4"), CodeUnsupportedFeat,
			[]string{`instance "a"`, "requires a vertex() stage"}},
		{"bad type", instMaterial("instance a : mat4\nvertex() -> vec4 { return vec4f(1.0, 1.0, 1.0, 1.0) }"), CodeUnsupportedType,
			[]string{`"a"`, `"mat4"`, "float, vec2, vec3, vec4, int, or uint"}},
		{"color type", instMaterial("instance a : color\nvertex() -> vec4 { return vec4f(1.0, 1.0, 1.0, 1.0) }"), CodeUnsupportedType,
			[]string{`"color"`}},
		{"bool type", instMaterial("instance a : bool\nvertex() -> vec4 { return vec4f(1.0, 1.0, 1.0, 1.0) }"), CodeUnsupportedType, nil},
		{"duplicate", instMaterial("instance a : vec4\ninstance a : vec2\nvertex() -> vec4 { return a }"), CodeDuplicateLocal,
			[]string{`duplicate instance attribute "a"`}},
		{"reserved", instMaterial("instance position : vec4\nvertex() -> vec4 { return position }"), CodeReservedName,
			[]string{`"position"`}},
		{"reserved index", instMaterial("instance instanceIndex : uint\nvertex() -> vec4 { return mvp * vec4f(1.0, 1.0, 1.0, 1.0) }"), CodeReservedName,
			[]string{`"instanceIndex"`}},
		{"clashes with param", instMaterial("param a : float\ninstance a : vec4\nvertex() -> vec4 { return mvp * vec4f(1.0, 1.0, 1.0, 1.0) }"), CodeInterfaceCollision,
			[]string{`"a"`}},
		{"clashes with varying", instMaterial("varying a : vec4\ninstance a : vec4\nvertex() -> vec4 { return mvp * vec4f(1.0, 1.0, 1.0, 1.0) }"), CodeInterfaceCollision,
			[]string{`"a"`}},
		{"clashes with local", instMaterial("instance a : vec4\nvertex() -> vec4 {\n let a = vec4f(1.0, 1.0, 1.0, 1.0)\n return a }"), CodeDuplicateLocal,
			[]string{`"a"`, "conflicts with attribute"}},
		{"read in surface", `material M {
    instance a : vec4
    varying v : vec4
    vertex() -> vec4 { v = a
        return mvp * a }
    surface(geo) -> color { return rgb(a.x, 0.0, 0.0) }
}`, CodeUnknownName, []string{`instance attribute "a" is only readable in vertex()`, "varying"}},
		{"instanceIndex in surface", `material M {
    surface(geo) -> color { return rgb(float(instanceIndex), 0.0, 0.0) }
}`, CodeUnknownName, []string{"instanceIndex is only available in a vertex() stage"}},
		{"points kind", `material M kind points {
    instance a : vec4
    surface(geo) -> color { return rgb(1.0, 1.0, 1.0) }
}`, CodeUnsupportedFeat, []string{"points materials cannot declare instance attributes"}},
		{"post kind", `material M kind post {
    instance a : vec4
    surface(geo) -> color { return rgb(1.0, 1.0, 1.0) }
}`, CodeUnsupportedFeat, []string{"post materials"}},
		{"feedback kind", `material M kind feedback {
    state s
    instance a : vec4
    feedback(cell) -> vec4 { return state(0i, 0i) }
}`, CodeUnsupportedFeat, []string{"feedback materials"}},
		{"screen kind", `material M kind screen {
    instance a : vec4
    surface(screen) -> color { return rgb(1.0, 1.0, 1.0, 1.0) }
}`, CodeUnsupportedFeat, []string{"screen materials"}},
		{"surface kind", `material M {
    instance a : vec4
    surface(geo) -> Surface { return pbr(vec3f(1.0, 1.0, 1.0), 0.5, 0.0) }
}`, CodeUnsupportedFeat, []string{"surface materials"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := lowerTexelSrc(t, tc.src)
			if err == nil {
				t.Fatal("lowered without a diagnostic")
			}
			if tc.code == "" {
				return // any error: a local that shadows an interface name
			}
			wantDiagnostic(t, err, tc.code, tc.contains...)
		})
	}
}

func TestBitOperatorTyping(t *testing.T) {
	ok := []string{
		"vertexIndex & 1u", "vertexIndex | 2u", "vertexIndex ^ 3u", "vertexIndex >> 1u", "vertexIndex << 1u",
		"(vertexIndex >> 1u) & 1u",
	}
	for _, e := range ok {
		src := instMaterial("vertex() -> vec4 { return vec4f(float(" + e + "), 0.0, 0.0, 1.0) }")
		if _, _, err := lowerTexelSrc(t, src); err != nil {
			t.Errorf("%s: %v", e, err)
		}
	}
	intOK := []string{"n & 3i", "n | 1i", "n ^ 1i", "n >> 1u", "n << 2u"}
	for _, e := range intOK {
		src := instMaterial("vertex() -> vec4 { let n = 5i\n return vec4f(float(" + e + "), 0.0, 0.0, 1.0) }")
		if _, _, err := lowerTexelSrc(t, src); err != nil {
			t.Errorf("%s: %v", e, err)
		}
	}
	bad := []struct{ expr, contains string }{
		{"vertexIndex & 1i", "two ints or two uints"},
		{"vertexIndex | 1.0", "two ints or two uints"},
		{"1.0 & 1.0", "two ints or two uints"},
		{"vertexIndex >> 1i", "uint count on the right"},
		{"vertexIndex << 1.0", "uint count on the right"},
		{"2.0 >> 1u", "needs an int or uint value on the left"},
		{"true & true", "two ints or two uints"},
	}
	for _, tc := range bad {
		src := instMaterial("vertex() -> vec4 { return vec4f(float(" + tc.expr + "), 0.0, 0.0, 1.0) }")
		_, _, err := lowerTexelSrc(t, src)
		if err == nil {
			t.Errorf("%s: lowered", tc.expr)
			continue
		}
		wantDiagnostic(t, err, CodeTypeMismatch, tc.contains)
	}
}

func TestBitOperatorResultTypes(t *testing.T) {
	// An int shift stays int; mixing it with a uint is a type error.
	if _, _, err := lowerTexelSrc(t, instMaterial("vertex() -> vec4 { let n = 5i\n return vec4f(float((n >> 1u) & vertexIndex), 0.0, 0.0, 1.0) }")); err == nil || !strings.Contains(err.Error(), "two ints or two uints") {
		t.Fatalf("err = %v", err)
	}
	mod, _, err := lowerTexelSrc(t, instMaterial("vertex() -> vec4 { let n = 5i\n let m = (n >> 1u) & 3i\n return vec4f(float(m), 0.0, 0.0, 1.0) }"))
	if err != nil {
		t.Fatal(err)
	}
	if !ir.UsesBitOperators(mod) {
		t.Error("UsesBitOperators = false")
	}
}

func TestBitOperatorsRejectedInDefaults(t *testing.T) {
	_, _, err := lowerTexelSrc(t, `material M {
    param n : int = 1i & 3i
    surface(geo) -> color { return rgb(1.0, 1.0, 1.0) }
}`)
	if err == nil {
		t.Fatal("a bit operator in a default value lowered")
	}
}

// A literal shift count of 32 or more is an error; 31 and a non-literal count
// stay legal.
func TestShiftCountOutOfRange(t *testing.T) {
	for _, expr := range []string{"1u << 32u", "vertexIndex >> 40u", "1i << 32u"} {
		_, _, err := lowerTexelSrc(t, instMaterial("vertex() -> vec4 { return vec4f(float("+expr+"), 0.0, 0.0, 1.0) }"))
		if err == nil {
			t.Errorf("%s: lowered", expr)
			continue
		}
		wantDiagnostic(t, err, CodeTypeMismatch, "out of range")
	}
	for _, expr := range []string{"1u << 31u", "vertexIndex >> 0u", "1u << (vertexIndex & 31u)"} {
		if _, _, err := lowerTexelSrc(t, instMaterial("vertex() -> vec4 { return vec4f(float("+expr+"), 0.0, 0.0, 1.0) }")); err != nil {
			t.Errorf("%s: %v", expr, err)
		}
	}
}
