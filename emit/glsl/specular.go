package glsl

import (
	"fmt"
	"strings"

	"m31labs.dev/selena/emit/internal"
	"m31labs.dev/selena/ir"
)

func writeMeshFragment(b *strings.Builder, m ir.Module, fs internal.Resolver) {
	if !m.SpecularMRT {
		fs.ReturnFn = glReturnFn
		b.WriteString("\nvoid main() {\n")
		internal.EmitStmtList(b, m.Fragment.Body, fs, "  ", false)
		fmt.Fprintf(b, "  gl_FragColor = %s;\n}\n", ir.Print(m.Fragment.Output, fs))
		return
	}
	b.WriteString("\n")
	if body, color, specular, ok := ir.FusedSpecular(m); ok {
		b.WriteString("void main() {\n")
		internal.EmitStmtList(b, body, fs, "  ", false)
		fmt.Fprintf(b, "  vec4 selenaMRTColor = %s;\n  vec3 selenaMRTSpecular = (%s).rgb;\n  gl_FragData[0] = selenaMRTColor;\n  gl_FragData[1] = vec4(selenaMRTSpecular, selenaMRTColor.a);\n}\n", ir.Print(color, fs), ir.Print(specular, fs))
		return
	}
	internal.EmitSpecularHelpers(b, m, fs, "vec4 selenaSurface()", "vec4 selenaSpecular()", false)
	b.WriteString("void main() {\n  vec4 color = selenaSurface();\n")
	if m.Specular == nil {
		b.WriteString("  vec3 specular = vec3(0.0);\n")
	} else {
		b.WriteString("  vec3 specular = selenaSpecular().rgb;\n")
	}
	b.WriteString("  gl_FragData[0] = color;\n  gl_FragData[1] = vec4(specular, color.a);\n}\n")
}
