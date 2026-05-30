package glsl

import (
	"fmt"
	"strings"

	"m31labs.dev/selena/ir"
)

// Emit renders m as GLSL ES 1.00 vertex and fragment sources for the WebGL
// backend (browser + GoSX desktop via Chromium). Unlike WGSL, GLSL ships the
// two stages as separate sources and addresses everything through bare globals
// (attribute/uniform/varying) — the divergence this slice exists to surface.
func Emit(m ir.Module) (vertex, fragment string, err error) {
	return emitVertex(m), emitFragment(m), nil
}

func emitVertex(m ir.Module) string {
	var b strings.Builder
	for _, a := range m.Attributes {
		fmt.Fprintf(&b, "attribute %s %s;\n", typeName(a.Type), a.Name)
	}
	for _, u := range m.Uniforms {
		fmt.Fprintf(&b, "uniform %s %s;\n", typeName(u.Type), u.Name)
	}
	for _, v := range m.Varyings {
		fmt.Fprintf(&b, "varying %s %s;\n", typeName(v.Type), v.Name)
	}
	vary := nameSet(m.Varyings)
	b.WriteString("\nvoid main() {\n")
	for _, s := range m.Vertex.Body {
		if vary[s.Target] {
			fmt.Fprintf(&b, "  %s = %s;\n", s.Target, ir.Print(s.Value, bare{}))
		} else {
			fmt.Fprintf(&b, "  %s %s = %s;\n", typeName(s.Type), s.Target, ir.Print(s.Value, bare{}))
		}
	}
	fmt.Fprintf(&b, "  gl_Position = %s;\n}\n", ir.Print(m.Vertex.Output, bare{}))
	return b.String()
}

func emitFragment(m ir.Module) string {
	var b strings.Builder
	b.WriteString("precision mediump float;\n")
	for _, u := range m.Uniforms {
		fmt.Fprintf(&b, "uniform %s %s;\n", typeName(u.Type), u.Name)
	}
	for _, v := range m.Varyings {
		fmt.Fprintf(&b, "varying %s %s;\n", typeName(v.Type), v.Name)
	}
	b.WriteString("\nvoid main() {\n")
	for _, s := range m.Fragment.Body {
		fmt.Fprintf(&b, "  %s %s = %s;\n", typeName(s.Type), s.Target, ir.Print(s.Value, bare{}))
	}
	fmt.Fprintf(&b, "  gl_FragColor = %s;\n}\n", ir.Print(m.Fragment.Output, bare{}))
	return b.String()
}

// bare implements ir.Dialect for GLSL, where every reference is a bare global
// and types use their plain GLSL spelling.
type bare struct{}

func (bare) TypeName(t ir.Type) string { return typeName(t) }
func (bare) Ref(name string) string    { return name }

func typeName(t ir.Type) string {
	switch t {
	case ir.Float:
		return "float"
	case ir.Vec2:
		return "vec2"
	case ir.Vec3:
		return "vec3"
	case ir.Vec4:
		return "vec4"
	case ir.Mat3:
		return "mat3"
	case ir.Mat4:
		return "mat4"
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
