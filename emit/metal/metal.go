package metal

import (
	"fmt"
	"strings"

	"m31labs.dev/selena/ir"
)

// Emit renders m as a single Metal Shading Language source with vertexMain and
// fragmentMain, for the iOS SceneKit backend consumed by gosx-native. Structure
// mirrors WGSL (struct IO, a constant uniform buffer) but with Metal spelling
// and attribute/stage_in/buffer qualifiers.
//
// NOTE: this emits standalone MSL functions. The SceneKit binding strategy
// (SCNProgram semantic uniforms vs. a packed argument buffer) is the binding-
// model design decision the slice is meant to expose; for now uniforms are a
// single constant buffer at [[buffer(0)]].
func Emit(m ir.Module) (string, error) {
	var b strings.Builder
	b.WriteString("#include <metal_stdlib>\nusing namespace metal;\n\n")

	if len(m.Uniforms) > 0 {
		b.WriteString("struct Uniforms {\n")
		for _, u := range m.Uniforms {
			fmt.Fprintf(&b, "  %s %s;\n", typeName(u.Type), u.Name)
		}
		b.WriteString("};\n\n")
	}

	b.WriteString("struct VertexIn {\n")
	for i, a := range m.Attributes {
		fmt.Fprintf(&b, "  %s %s [[attribute(%d)]];\n", typeName(a.Type), a.Name, i)
	}
	b.WriteString("};\n\n")

	b.WriteString("struct VertexOut {\n  float4 position [[position]];\n")
	for _, v := range m.Varyings {
		fmt.Fprintf(&b, "  %s %s;\n", typeName(v.Type), v.Name)
	}
	b.WriteString("};\n\n")

	vs := newScope(m, false)
	b.WriteString("vertex VertexOut vertexMain(VertexIn in [[stage_in]], constant Uniforms& u [[buffer(0)]]) {\n  VertexOut out;\n")
	for _, s := range m.Vertex.Body {
		if vs.varyings[s.Target] {
			fmt.Fprintf(&b, "  out.%s = %s;\n", s.Target, ir.Print(s.Value, vs))
		} else {
			fmt.Fprintf(&b, "  %s %s = %s;\n", typeName(s.Type), s.Target, ir.Print(s.Value, vs))
		}
	}
	fmt.Fprintf(&b, "  out.position = %s;\n  return out;\n}\n\n", ir.Print(m.Vertex.Output, vs))

	fs := newScope(m, true)
	fragSig := "fragment float4 fragmentMain(VertexOut in [[stage_in]], constant Uniforms& u [[buffer(0)]]"
	for i, t := range m.Textures {
		fragSig += fmt.Sprintf(", texture2d<float> %s [[texture(%d)]], sampler %sSampler [[sampler(%d)]]", t.Name, i, t.Name, i)
	}
	b.WriteString(fragSig + ") {\n")
	for _, s := range m.Fragment.Body {
		fmt.Fprintf(&b, "  %s %s = %s;\n", typeName(s.Type), s.Target, ir.Print(s.Value, fs))
	}
	fmt.Fprintf(&b, "  return %s;\n}\n", ir.Print(m.Fragment.Output, fs))

	return b.String(), nil
}

// scope resolves references to Metal's struct-qualified form (u.x for the
// uniform buffer, in.x for stage inputs). Same shape as the WGSL scope.
type scope struct {
	uniforms   map[string]bool
	attributes map[string]bool
	varyings   map[string]bool
	fragment   bool
}

func newScope(m ir.Module, fragment bool) scope {
	return scope{
		uniforms:   nameSet(m.Uniforms),
		attributes: nameSet(m.Attributes),
		varyings:   nameSet(m.Varyings),
		fragment:   fragment,
	}
}

func (s scope) TypeName(t ir.Type) string { return typeName(t) }

func (s scope) Call(name string, args []string) string {
	return name + "(" + strings.Join(args, ", ") + ")"
}

func (s scope) Ref(name string) string {
	switch {
	case s.uniforms[name]:
		return "u." + name
	case !s.fragment && s.attributes[name]:
		return "in." + name
	case s.fragment && s.varyings[name]:
		return "in." + name
	default:
		return name
	}
}

// Sample uses Metal's texture.sample(sampler, uv); the sampler is a separate
// function argument bound at [[sampler(i)]].
func (s scope) Sample(tex, uv string) string {
	return tex + ".sample(" + tex + "Sampler, " + uv + ")"
}

func typeName(t ir.Type) string {
	switch t {
	case ir.Float:
		return "float"
	case ir.Vec2:
		return "float2"
	case ir.Vec3:
		return "float3"
	case ir.Vec4:
		return "float4"
	case ir.Mat3:
		return "float3x3"
	case ir.Mat4:
		return "float4x4"
	default:
		return "float"
	}
}

func nameSet(bs []ir.Binding) map[string]bool {
	m := make(map[string]bool, len(bs))
	for _, b := range bs {
		m[b.Name] = true
	}
	return m
}
