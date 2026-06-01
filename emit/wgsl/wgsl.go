package wgsl

import (
	"fmt"
	"strings"

	"m31labs.dev/selena/ir"
)

// Emit renders m as a single WGSL source string exposing vertexMain and
// fragmentMain, for the WebGPU backend (browser + GoSX desktop via Chromium).
func Emit(m ir.Module) (string, error) {
	var b strings.Builder

	if len(m.Uniforms) > 0 {
		b.WriteString("struct Uniforms {\n")
		for _, u := range m.Uniforms {
			fmt.Fprintf(&b, "  %s : %s,\n", u.Name, typeName(u.Type))
		}
		b.WriteString("};\n@group(0) @binding(0) var<uniform> u : Uniforms;\n\n")
	}

	// Textures: WGSL binds the texture and its sampler separately. Convention:
	// uniform block is binding 0, then texture i takes 1+2i and its sampler 2+2i.
	for i, t := range m.Textures {
		fmt.Fprintf(&b, "@group(0) @binding(%d) var %s : texture_2d<f32>;\n", 1+2*i, t.Name)
		fmt.Fprintf(&b, "@group(0) @binding(%d) var %sSampler : sampler;\n", 2+2*i, t.Name)
	}
	if len(m.Textures) > 0 {
		b.WriteString("\n")
	}

	b.WriteString("struct VertexInput {\n")
	for i, a := range m.Attributes {
		fmt.Fprintf(&b, "  @location(%d) %s : %s,\n", i, a.Name, typeName(a.Type))
	}
	b.WriteString("};\n\n")

	b.WriteString("struct VertexOutput {\n  @builtin(position) position : vec4<f32>,\n")
	for i, v := range m.Varyings {
		fmt.Fprintf(&b, "  @location(%d) %s : %s,\n", i, v.Name, typeName(v.Type))
	}
	b.WriteString("};\n\n")

	// Vertex stage.
	vs := newScope(m, false)
	b.WriteString("@vertex\nfn vertexMain(in : VertexInput) -> VertexOutput {\n  var out : VertexOutput;\n")
	for _, s := range m.Vertex.Body {
		if vs.varyings[s.Target] {
			fmt.Fprintf(&b, "  out.%s = %s;\n", s.Target, ir.Print(s.Value, vs))
		} else {
			fmt.Fprintf(&b, "  let %s = %s;\n", s.Target, ir.Print(s.Value, vs))
		}
	}
	fmt.Fprintf(&b, "  out.position = %s;\n  return out;\n}\n\n", ir.Print(m.Vertex.Output, vs))

	// Fragment stage.
	fs := newScope(m, true)
	b.WriteString("@fragment\nfn fragmentMain(in : VertexOutput) -> @location(0) vec4<f32> {\n")
	for _, s := range m.Fragment.Body {
		fmt.Fprintf(&b, "  let %s = %s;\n", s.Target, ir.Print(s.Value, fs))
	}
	fmt.Fprintf(&b, "  return %s;\n}\n", ir.Print(m.Fragment.Output, fs))

	return b.String(), nil
}

// scope implements ir.Dialect for one WGSL stage: it resolves references to the
// struct-qualified form WGSL needs (u.x for uniforms, in.x for stage inputs).
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
		return name // stage-local
	}
}

func (s scope) Sample(tex, uv string) string {
	return "textureSample(" + tex + ", " + tex + "Sampler, " + uv + ")"
}

func typeName(t ir.Type) string {
	switch t {
	case ir.Float:
		return "f32"
	case ir.Vec2:
		return "vec2<f32>"
	case ir.Vec3:
		return "vec3<f32>"
	case ir.Vec4:
		return "vec4<f32>"
	case ir.Mat3:
		return "mat3x3<f32>"
	case ir.Mat4:
		return "mat4x4<f32>"
	default:
		return "f32"
	}
}

func nameSet(bs []ir.Binding) map[string]bool {
	m := make(map[string]bool, len(bs))
	for _, b := range bs {
		m[b.Name] = true
	}
	return m
}
