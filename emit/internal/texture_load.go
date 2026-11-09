package internal

import (
	"m31labs.dev/prism/dialect"
	"m31labs.dev/selena/ir"
)

// Resolver implements ir.TexelDialect, so ir.Load and ir.TextureSize print on
// every backend. Each backend spells the read directly, as SampleLevel does.

var _ ir.TexelDialect = Resolver{}

// Load renders a texel read at integer coordinates x, y (mip level 0).
//
// WGSL:         textureLoad(t, vec2<i32>(x, y), 0)
// GLSL ES 3.00: texelFetch(t, ivec2(x, y), 0)
// GLSL ES 1.00: texture2D(t, (vec2(x, y) + 0.5) / _texSize_t), a nearest read
//
//	at the texel centre; the host sets _texSize_t and binds a NEAREST
//	sampler (ir.TexSizeUniform, Requires.GLTexSizeUniforms)
//
// Metal:        t.read(uint2(x, y), 0)
func (r Resolver) Load(tex, x, y string) string {
	tex = r.tex(tex)
	switch r.Dialect.(type) {
	case dialect.WGSL:
		return "textureLoad(" + tex + ", vec2<i32>(" + x + ", " + y + "), 0)"
	case dialect.Metal:
		return tex + ".read(uint2(" + x + ", " + y + "), 0)"
	case dialect.GLES:
		return "texelFetch(" + tex + ", ivec2(" + x + ", " + y + "), 0)"
	default:
		size := ir.TexSizeUniform(tex)
		return "texture2D(" + tex + ", (vec2(" + x + ", " + y + ") + 0.5) / " + size + ")"
	}
}

// TextureSize renders the base-level size of tex in texels as a vec2.
//
// WGSL:         vec2<f32>(textureDimensions(t))
// GLSL ES 3.00: vec2(textureSize(t, 0))
// GLSL ES 1.00: _texSize_t, a host-set uniform
// Metal:        float2(t.get_width(), t.get_height())
func (r Resolver) TextureSize(tex string) string {
	tex = r.tex(tex)
	switch r.Dialect.(type) {
	case dialect.WGSL:
		return "vec2<f32>(textureDimensions(" + tex + "))"
	case dialect.Metal:
		return "float2(" + tex + ".get_width(), " + tex + ".get_height())"
	case dialect.GLES:
		return "vec2(textureSize(" + tex + ", 0))"
	default:
		return ir.TexSizeUniform(tex)
	}
}
