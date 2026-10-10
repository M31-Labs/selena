package internal

import (
	"fmt"
	"strings"

	"m31labs.dev/selena/ir"
)

// EmitSpecularHelpers emits real, separately scoped functions, not source
// rewriting. Early returns stay inside their authored stage. Surface discard
// still terminates the fragment before either attachment is written.
func EmitSpecularHelpers(b *strings.Builder, m ir.Module, r Resolver, surfaceHeader, specularHeader string, wgsl bool) {
	b.WriteString(surfaceHeader + " {\n")
	EmitStmtList(b, m.Fragment.Body, r, "  ", wgsl)
	fmt.Fprintf(b, "  return %s;\n}\n\n", ir.Print(m.Fragment.Output, r))
	if m.Specular != nil {
		b.WriteString(specularHeader + " {\n")
		EmitStmtList(b, m.Specular.Body, r, "  ", wgsl)
		fmt.Fprintf(b, "  return %s;\n}\n\n", ir.Print(m.Specular.Output, r))
	}
}
