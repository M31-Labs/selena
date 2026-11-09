package bindings

// Instance attributes: the per-instance inputs of a mesh material's authored
// vertex(). A host streams one fixed-size record per instance; the descriptor
// says where each attribute sits in it.

// InstanceAttribute is one `instance name : type` declaration.
//
// Location continues after the vertex attributes (Layout.Attributes), in
// declaration order. Offset is the byte offset of the attribute inside the
// packed record. The record has no padding between attributes: every legal
// type is a multiple of 4 bytes, so every offset is 4-byte aligned already.
// The host binds the buffer with instance step mode (WebGPU stepMode
// "instance", Metal MTLVertexStepFunctionPerInstance, WebGL vertexAttribDivisor
// 1).
//
// Type is one of float, vec2, vec3, vec4, int, uint. A host binds int and uint
// as integers (WebGPU sint32/uint32, WebGL 2 vertexAttribIPointer), never as
// normalized or converted floats.
type InstanceAttribute struct {
	Name     string `json:"name"`
	Location int    `json:"location"`
	Type     string `json:"type"`
	Offset   int    `json:"offset"`
}

// InstanceAttributeSize returns the packed size in bytes of an instance
// attribute type, and false for a type that cannot be an instance attribute.
func InstanceAttributeSize(typ string) (int, bool) {
	switch typ {
	case "float", "int", "uint":
		return 4, true
	case "vec2":
		return 8, true
	case "vec3":
		return 12, true
	case "vec4":
		return 16, true
	}
	return 0, false
}

// PackInstanceRecord lays out attrs (name and type, in declaration order) as
// one packed record. firstLocation is the location of the first attribute.
// It returns the attributes with Location and Offset set, and the record
// stride in bytes. Types are assumed valid (see InstanceAttributeSize); an
// invalid type contributes no bytes.
func PackInstanceRecord(attrs []InstanceAttribute, firstLocation int) ([]InstanceAttribute, int) {
	out := make([]InstanceAttribute, len(attrs))
	offset := 0
	for i, a := range attrs {
		a.Location = firstLocation + i
		a.Offset = offset
		size, _ := InstanceAttributeSize(a.Type)
		offset += size
		out[i] = a
	}
	return out, offset
}
