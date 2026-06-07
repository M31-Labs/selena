package glsl

import (
	"fmt"
	"strings"

	"m31labs.dev/prism/dialect"
	"m31labs.dev/selena/emit/internal"
	"m31labs.dev/selena/ir"
)

var prismDialect = dialect.GLSL{}

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
	vary := internal.NameSet(m.Varyings)
	res := internal.NewBare(prismDialect)
	b.WriteString("\nvoid main() {\n")
	for _, s := range m.Vertex.Body {
		if vary[s.Target] {
			fmt.Fprintf(&b, "  %s = %s;\n", s.Target, ir.Print(s.Value, res))
		} else {
			fmt.Fprintf(&b, "  %s %s = %s;\n", typeName(s.Type), s.Target, ir.Print(s.Value, res))
		}
	}
	fmt.Fprintf(&b, "  gl_Position = %s;\n}\n", ir.Print(m.Vertex.Output, res))
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
	for _, t := range m.Textures {
		fmt.Fprintf(&b, "uniform sampler2D %s;\n", t.Name)
	}
	res := internal.NewBare(prismDialect)
	b.WriteString("\nvoid main() {\n")
	for _, s := range m.Fragment.Body {
		fmt.Fprintf(&b, "  %s %s = %s;\n", typeName(s.Type), s.Target, ir.Print(s.Value, res))
	}
	fmt.Fprintf(&b, "  gl_FragColor = %s;\n}\n", ir.Print(m.Fragment.Output, res))
	return b.String()
}

// typeName spells an ir.Type in GLSL, delegating to prism/dialect.
func typeName(t ir.Type) string { return prismDialect.TypeName(internal.TypeToGPU(t)) }
