package ir

// Instancing helpers: detecting the instanceIndex builtin and the bit
// operators, which GLSL ES 1.00 cannot express.

// StageUsesInstanceIndexBuiltin reports whether stage's body or output
// references the instanceIndex builtin, at any nesting depth.
func StageUsesInstanceIndexBuiltin(stage Stage) bool {
	return stageMatches(stage, func(e Expr) bool {
		r, ok := e.(Ref)
		return ok && r.Name == "instanceIndex"
	})
}

// BitOperators are the infix operators that exist only for int and uint.
var BitOperators = map[string]bool{"&": true, "|": true, "^": true, "<<": true, ">>": true}

// UsesBitOperators reports whether any stage of m uses a bit operator.
func UsesBitOperators(m Module) bool {
	has := func(e Expr) bool {
		b, ok := e.(Binary)
		return ok && BitOperators[b.Op]
	}
	if stageMatches(m.Vertex, has) || stageMatches(m.Fragment, has) {
		return true
	}
	return m.Specular != nil && stageMatches(*m.Specular, has)
}

// GLSLInstancingUnsupported lists why m cannot be emitted for GLSL ES 1.00
// (WebGL 1), or returns nil when it can. GLSL ES 1.00 has no instance id, no
// integer vertex attributes and no bit operators.
func GLSLInstancingUnsupported(m Module) []string {
	var why []string
	if m.UsesInstanceIndex {
		why = append(why, "instanceIndex needs gl_InstanceID, which GLSL ES 1.00 does not have; pass an index as an instance attribute instead")
	}
	for _, a := range m.InstanceAttributes {
		if a.Type == Int || a.Type == Uint {
			why = append(why, "instance attribute "+a.Name+" has type "+string(a.Type)+"; GLSL ES 1.00 has no integer vertex attributes, so use float and convert in vertex()")
		}
	}
	if UsesBitOperators(m) {
		why = append(why, "bit operators (& | ^ << >>) do not exist in GLSL ES 1.00")
	}
	return why
}

// NeedsTypedUintLiterals reports whether m uses a feature that makes a bare
// uint literal wrong on GLSL ES 3.00: bit operators, instance attributes or
// instanceIndex. Materials that use none of them keep the older literal
// spelling so their emitted source does not change.
func NeedsTypedUintLiterals(m Module) bool {
	return len(m.InstanceAttributes) > 0 || m.UsesInstanceIndex || UsesBitOperators(m)
}

// InstanceLocation returns the shader location of instance attribute i.
func (m Module) InstanceLocation(i int) int {
	if m.InstanceBase > 0 {
		return m.InstanceBase + i
	}
	return len(m.Attributes) + i
}
