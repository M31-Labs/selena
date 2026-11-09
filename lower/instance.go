package lower

import (
	"m31labs.dev/selena/bindings"
	"m31labs.dev/selena/hir"
	"m31labs.dev/selena/ir"
)

// Instancing: `instance name : type` declarations and the bit operators.

// instanceTypes are the types an instance attribute may have.
var instanceTypes = map[hir.Type]ir.Type{
	hir.Float: ir.Float,
	hir.Vec2:  ir.Vec2,
	hir.Vec3:  ir.Vec3,
	hir.Vec4:  ir.Vec4,
	hir.Int:   ir.Int,
	hir.Uint:  ir.Uint,
}

// checkInstances rejects `instance` declarations on materials that cannot use
// them: any kind other than mesh, and a mesh without an authored vertex().
func checkInstances(m hir.Material) error {
	if len(m.Instances) == 0 {
		return nil
	}
	first := m.Instances[0]
	switch m.Kind {
	case "", hir.KindMesh:
		if m.Vertex == nil {
			return diagnostic(CodeUnsupportedFeat, first.Span,
				"instance %q requires a vertex() stage to read it; instancing applies to mesh materials that author vertex()", first.Name)
		}
		return nil
	case hir.KindSurface:
		return diagnostic(CodeUnsupportedFeat, first.Span,
			"instance %q: surface materials use the host's vertex stage; instancing needs a mesh material with vertex()", first.Name)
	default:
		return diagnostic(CodeUnsupportedFeat, first.Span,
			"instance %q: %s materials cannot declare instance attributes; instancing needs a mesh material with vertex()", first.Name, m.Kind)
	}
}

// planInstances validates the instance declarations and returns them as IR
// bindings in declaration order.
func planInstances(m hir.Material) ([]ir.Binding, error) {
	var out []ir.Binding
	seen := map[string]bool{}
	for _, a := range m.Instances {
		if err := validateAuthorName("instance attribute", a.Name, a.Span); err != nil {
			return nil, err
		}
		if seen[a.Name] {
			return nil, diagnostic(CodeDuplicateLocal, a.Span, "duplicate instance attribute %q", a.Name)
		}
		seen[a.Name] = true
		t, ok := instanceTypes[a.Type]
		if !ok {
			return nil, diagnostic(CodeUnsupportedType, a.Span,
				"instance attribute %q: unsupported type %q (use float, vec2, vec3, vec4, int, or uint)", a.Name, a.Type)
		}
		out = append(out, ir.Binding{Name: a.Name, Type: t})
	}
	return out, nil
}

// instanceLayout computes the descriptor entries and record stride for attrs,
// whose first location follows the vertex attributes.
func instanceLayout(attrs []ir.Binding, firstLocation int) ([]bindings.InstanceAttribute, int) {
	if len(attrs) == 0 {
		return nil, 0
	}
	in := make([]bindings.InstanceAttribute, len(attrs))
	for i, a := range attrs {
		in[i] = bindings.InstanceAttribute{Name: a.Name, Type: string(a.Type)}
	}
	return bindings.PackInstanceRecord(in, firstLocation)
}

// bitOperatorType types & | ^ << >>. The bitwise operators take two operands of
// the same integer type. A shift takes an integer on the left and a uint count
// on the right (WGSL requires u32 there); the result has the left type.
func (t *typer) bitOperatorType(b hir.Binary) (ir.Type, error) {
	lt, err := t.typeOf(b.L)
	if err != nil {
		return "", err
	}
	rt, err := t.typeOf(b.R)
	if err != nil {
		return "", err
	}
	isInt := func(x ir.Type) bool { return x == ir.Int || x == ir.Uint }
	if b.Op == "<<" || b.Op == ">>" {
		if !isInt(lt) || rt != ir.Uint {
			return "", diagnostic(CodeTypeMismatch, b.Span,
				"shift %s needs an int or uint value on the left and a uint count on the right (write 1u), got %s and %s", b.Op, lt, rt)
		}
		return lt, nil
	}
	if !isInt(lt) || lt != rt {
		return "", diagnostic(CodeTypeMismatch, b.Span,
			"operator %s needs two ints or two uints (write 1u for a uint literal), got %s and %s", b.Op, lt, rt)
	}
	return lt, nil
}
