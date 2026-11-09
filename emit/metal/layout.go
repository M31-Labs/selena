package metal

import (
	"fmt"
	"strings"

	"m31labs.dev/selena/bindings"
	"m31labs.dev/selena/emit/internal"
	"m31labs.dev/selena/ir"
)

// padPrefix starts the name of every explicit padding member the emitter
// writes. A uniform may not use it.
const padPrefix = "_selenaPad"

// mslLayout is the size and alignment of a type as a Metal struct member.
type mslLayout struct{ size, align int }

// mslMember returns the Metal layout of t as a struct member. A vec3 is
// float3, which has size 16 and alignment 16; packed is the 12 byte form.
func mslMember(t ir.Type, packed bool) (typ string, l mslLayout, err error) {
	switch t {
	case ir.Float, ir.Int, ir.Uint:
		return typeName(t), mslLayout{4, 4}, nil
	case ir.Vec2:
		return typeName(t), mslLayout{8, 8}, nil
	case ir.Vec3:
		if packed {
			return "packed_float3", mslLayout{12, 4}, nil
		}
		return typeName(t), mslLayout{16, 16}, nil
	case ir.Vec4:
		return typeName(t), mslLayout{16, 16}, nil
	case ir.Mat3:
		return typeName(t), mslLayout{48, 16}, nil
	case ir.Mat4:
		return typeName(t), mslLayout{64, 16}, nil
	}
	return "", mslLayout{}, fmt.Errorf("uniform type %q has no Metal buffer layout", t)
}

// wideElement returns the 16 byte Metal storage type and the accessor that
// reads the value back, for an array element type whose own Metal stride is
// smaller than the descriptor's 16 bytes. ok is false for other types.
func wideElement(t ir.Type) (typ, accessor string, ok bool) {
	switch t {
	case ir.Float:
		return "float4", ".x", true
	case ir.Int:
		return "int4", ".x", true
	case ir.Uint:
		return "uint4", ".x", true
	case ir.Vec2:
		return "float4", ".xy", true
	}
	return "", "", false
}

// wideArrayUniforms maps each widened uniform array to its read accessor.
func wideArrayUniforms(m ir.Module) map[string]string {
	var set map[string]string
	for _, au := range m.ArrayUniforms {
		if _, acc, ok := wideElement(au.ElemType); ok && au.Count > 1 {
			if set == nil {
				set = map[string]string{}
			}
			set[au.Name] = acc
		}
	}
	return set
}

// uniformNamedTypes lists the module's uniforms in the order the lowering
// gives bindings.ComputeUniformBlock: scalars, then arrays.
func uniformNamedTypes(m ir.Module) []bindings.NamedType {
	out := make([]bindings.NamedType, 0, len(m.Uniforms)+len(m.ArrayUniforms))
	for _, u := range m.Uniforms {
		out = append(out, bindings.NamedType{Name: u.Name, Type: u.Type})
	}
	for _, au := range m.ArrayUniforms {
		out = append(out, bindings.NamedType{Name: au.Name, Type: au.ElemType, Count: au.Count})
	}
	return out
}

// packedVec3Uniforms names the non-array vec3 uniforms. They are declared
// packed_float3, so a read must convert back to float3.
func packedVec3Uniforms(m ir.Module) map[string]bool {
	var set map[string]bool
	for _, u := range m.Uniforms {
		if u.Type == ir.Vec3 {
			if set == nil {
				set = map[string]bool{}
			}
			set[u.Name] = true
		}
	}
	return set
}

// writeUniformStruct writes `struct <name> { ... };` so that every member
// offset and the total size equal the binding descriptor's std140 layout,
// which hosts use to fill the buffer. A plain Metal float3 has size 16, so a
// vec3 is declared packed_float3, and explicit char padding places a member
// the descriptor puts after a gap. It returns an error rather than write a
// struct that disagrees with the descriptor.
func writeUniformStruct(b *strings.Builder, name string, m ir.Module) error {
	named := uniformNamedTypes(m)
	block := bindings.ComputeUniformBlock(named)
	types := make(map[string]ir.Type, len(named))
	for _, n := range named {
		if strings.HasPrefix(n.Name, padPrefix) {
			return fmt.Errorf("uniform %q: the %s prefix is reserved for Metal layout padding", n.Name, padPrefix)
		}
		types[n.Name] = n.Type
	}

	var body strings.Builder
	cursor, pads := 0, 0
	pad := func(n int) {
		fmt.Fprintf(&body, "  char %s%d[%d];\n", padPrefix, pads, n)
		pads++
	}
	for _, f := range block.Fields {
		t := types[f.Name]
		typ, l, err := mslMember(t, f.Count <= 1)
		if wide, _, ok := wideElement(t); ok && f.Count > 1 {
			typ, l, err = wide, mslLayout{16, 16}, nil
		}
		if err != nil {
			return fmt.Errorf("uniform %q: %w", f.Name, err)
		}
		if f.Count > 1 && l.size != f.Stride {
			return fmt.Errorf("uniform %q: array of %s has Metal stride %d but the descriptor stride is %d; its elements cannot be widened to 16 bytes", f.Name, t, l.size, f.Stride)
		}
		if f.Offset%l.align != 0 {
			return fmt.Errorf("uniform %q: descriptor offset %d is not a multiple of the Metal alignment %d", f.Name, f.Offset, l.align)
		}
		if cursor > f.Offset {
			return fmt.Errorf("uniform %q: Metal layout reaches offset %d, past the descriptor offset %d", f.Name, cursor, f.Offset)
		}
		if f.Offset > cursor {
			pad(f.Offset - cursor)
		}
		if f.Count > 1 {
			fmt.Fprintf(&body, "  %s %s[%d];\n", typ, f.Name, f.Count)
			cursor = f.Offset + f.Count*l.size
		} else {
			fmt.Fprintf(&body, "  %s %s;\n", typ, f.Name)
			cursor = f.Offset + l.size
		}
	}
	if cursor > block.Size {
		return fmt.Errorf("uniform block: Metal layout is %d bytes, the descriptor size is %d", cursor, block.Size)
	}
	if block.Size > cursor {
		pad(block.Size - cursor)
	}
	fmt.Fprintf(b, "struct %s {\n%s};\n\n", name, body.String())
	return nil
}

// qualified builds the struct-qualified resolver for one stage and marks the
// packed vec3 uniforms.
func qualified(m ir.Module, fragment bool) internal.Resolver {
	r := internal.NewQualified(prismDialect, m, fragment)
	r.PackedVec3 = packedVec3Uniforms(m)
	r.WideArrays = wideArrayUniforms(m)
	return r
}
