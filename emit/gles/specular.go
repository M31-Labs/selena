package gles

import (
	"fmt"
	"strings"

	"m31labs.dev/selena/emit/internal"
	"m31labs.dev/selena/ir"
)

func writeMeshFragment(b *strings.Builder, m ir.Module, fs internal.Resolver) {
	if !m.SpecularMRT {
		fs.ReturnFn = glesReturnFn
		b.WriteString("out vec4 fragColor;\n\nvoid main() {\n")
		internal.EmitStmtList(b, m.Fragment.Body, fs, "  ", false)
		fmt.Fprintf(b, "  fragColor = %s;\n}\n", ir.Print(m.Fragment.Output, fs))
		return
	}
	b.WriteString("layout(location = 0) out vec4 fragColor;\nlayout(location = 1) out vec4 specularColor;\n\n")
	if body, color, specular, ok := ir.FusedSpecular(m); ok {
		b.WriteString("void main() {\n")
		internal.EmitStmtList(b, body, fs, "  ", false)
		fmt.Fprintf(b, "  vec4 selenaMRTColor = %s;\n  vec3 selenaMRTSpecular = (%s).rgb;\n  fragColor = selenaMRTColor;\n  specularColor = vec4(selenaMRTSpecular, selenaMRTColor.a);\n}\n", ir.Print(color, fs), ir.Print(specular, fs))
		return
	}
	internal.EmitSpecularHelpers(b, m, fs, "vec4 selenaSurface()", "vec4 selenaSpecular()", false)
	b.WriteString("void main() {\n  vec4 color = selenaSurface();\n")
	if m.Specular == nil {
		b.WriteString("  vec3 specular = vec3(0.0);\n")
	} else {
		b.WriteString("  vec3 specular = selenaSpecular().rgb;\n")
	}
	b.WriteString("  fragColor = color;\n  specularColor = vec4(specular, color.a);\n}\n")
}
