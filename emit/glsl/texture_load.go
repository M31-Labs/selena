package glsl

import (
	"fmt"
	"strings"

	"m31labs.dev/selena/ir"
)

// writeTexSizeUniforms declares the host-set size uniform of every texture that
// stage reads with load() or queries with size(). GLSL ES 1.00 has neither
// texelFetch nor textureSize, so load() becomes a nearest texture2D read at the
// texel centre divided by this size (see internal.Resolver.Load), and size(t)
// returns it. The uniform is highp in both stages: a uniform shared by the
// vertex and fragment stage must have the same precision in both, or the
// program fails to link. The host sets it to the texture size in texels and
// binds a NEAREST sampler (Layout.Requires.GLTexSizeUniforms).
func writeTexSizeUniforms(b *strings.Builder, m ir.Module, fragment bool) {
	stage := m.Vertex
	if fragment {
		stage = m.Fragment
	}
	names := ir.StageTexSizeTextures(m, stage)
	if fragment && m.Specular != nil {
		// The optional specular companion reads the same textures; declare the
		// union so the variant that emits it links.
		seen := map[string]bool{}
		for _, n := range names {
			seen[n] = true
		}
		for _, n := range ir.StageTexSizeTextures(m, *m.Specular) {
			if !seen[n] {
				names = append(names, n)
			}
		}
	}
	for _, name := range names {
		fmt.Fprintf(b, "uniform highp vec2 %s;\n", ir.TexSizeUniform(name))
	}
}
