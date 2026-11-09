package ir

// Texel loads and texture sizes for data textures (spec G1). These nodes live
// in their own file so the shared ir.go only needs a Print hook and a Format
// field on Texture.

// Texture storage formats a texture2d param may declare with
// `param t : texture2d format <name>`. The empty string means the default,
// FormatRGBA8Unorm.
const (
	FormatRGBA8Unorm  = "rgba8unorm"
	FormatRGBA16Float = "rgba16float"
	FormatRGBA32Float = "rgba32float"
	FormatR16Float    = "r16float"
	FormatR32Float    = "r32float"
)

// TextureFormats lists the storage formats a texture2d param may declare, in
// documentation order.
var TextureFormats = []string{
	FormatRGBA8Unorm, FormatRGBA16Float, FormatRGBA32Float, FormatR16Float, FormatR32Float,
}

// IsTextureFormat reports whether name is a storage format a texture2d param
// may declare.
func IsTextureFormat(name string) bool {
	for _, f := range TextureFormats {
		if f == name {
			return true
		}
	}
	return false
}

// IsFloat32Format reports whether format is a 32-bit float format. WebGPU can
// only filter these when the float32-filterable feature is on, so sample() and
// sampleLevel() on them are rejected and load() is the supported read.
func IsFloat32Format(format string) bool {
	return format == FormatR32Float || format == FormatRGBA32Float
}

// Load reads one texel of a 2D Texture at integer coordinates (X, Y), mip level
// 0, with no filtering and no sampler. X and Y are int expressions. The result
// is a vec4; one- and two-channel formats return (r, 0, 0, 1) as the GPU APIs
// define it. Legal in the fragment stage and in an authored vertex stage.
//
// WGSL:         textureLoad(t, vec2<i32>(x, y), 0)
// GLSL ES 3.00: texelFetch(t, ivec2(x, y), 0)
// GLSL ES 1.00: texture2D(t, (vec2(x, y) + 0.5) / _texSize_t) (no texelFetch)
// Metal:        t.read(uint2(x, y), 0)
//
// Coordinates outside the texture are not defined to agree across targets; the
// author clamps them.
type Load struct {
	Texture string
	X, Y    Expr
}

// TextureSize is the size of a 2D Texture's base level in texels, as a vec2
// (width, height).
//
// WGSL:         vec2<f32>(textureDimensions(t))
// GLSL ES 3.00: vec2(textureSize(t, 0))
// GLSL ES 1.00: the host-set uniform _texSize_t (see TexSizeUniform)
// Metal:        float2(t.get_width(), t.get_height())
type TextureSize struct {
	Texture string
}

func (Load) isExpr()        {}
func (TextureSize) isExpr() {}

func (x Load) children() []Expr      { return []Expr{x.X, x.Y} }
func (TextureSize) children() []Expr { return nil }

// TexelDialect is the optional Dialect extension that renders Load and
// TextureSize. Like DepthDialect, it is separate so that adding the node does
// not break other Dialect implementations.
type TexelDialect interface {
	Load(tex, x, y string) string
	TextureSize(tex string) string
}

func printLoad(x Load, d Dialect) string {
	if td, ok := d.(TexelDialect); ok {
		return td.Load(x.Texture, Print(x.X, d), Print(x.Y, d))
	}
	emitPanic(CodeEmitUnwiredNode, "dialect does not support texel loads")
	return ""
}

func printTextureSize(x TextureSize, d Dialect) string {
	if td, ok := d.(TexelDialect); ok {
		return td.TextureSize(x.Texture)
	}
	emitPanic(CodeEmitUnwiredNode, "dialect does not support texture size queries")
	return ""
}

// TexSizeUniform is the name of the host-set vec2 uniform that carries the
// size of texture name in the GLSL ES 1.00 (WebGL 1) artifact, which has no
// textureSize.
func TexSizeUniform(name string) string { return "_texSize_" + name }

// SampleMode values for a texture, from how the material reads it.
const (
	SampleModeSample = "sample"
	SampleModeLoad   = "load"
	SampleModeBoth   = "both"
)

// TextureAccess reports, for each texture name, whether the material samples
// it (Sample, SampleLevel, SampleCube, SampleCubeLevel, SampleDepth), loads it
// (Load), or both, across every stage. A TextureSize query counts as neither:
// it needs the texture bound but reads no texel.
func TextureAccess(m Module) (sampled, loaded map[string]bool) {
	sampled, loaded = map[string]bool{}, map[string]bool{}
	visit := func(e Expr) bool {
		switch s := e.(type) {
		case Sample:
			sampled[s.Texture] = true
		case SampleLevel:
			sampled[s.Texture] = true
		case SampleCube:
			sampled[s.Texture] = true
		case SampleCubeLevel:
			sampled[s.Texture] = true
		case SampleDepth:
			sampled[s.Texture] = true
		case Load:
			loaded[s.Texture] = true
		}
		return false // keep walking: collect every use
	}
	stageMatches(m.Vertex, visit)
	stageMatches(m.Fragment, visit)
	if m.Specular != nil {
		stageMatches(*m.Specular, visit)
	}
	return sampled, loaded
}

// TexSizeTextures returns the names of the textures whose size the GLSL ES 1.00
// artifact must receive from the host: every texture read by Load or queried by
// TextureSize in any stage, in m.Textures order.
func TexSizeTextures(m Module) []string {
	used := map[string]bool{}
	stageTexelTextures(m.Vertex, used)
	stageTexelTextures(m.Fragment, used)
	if m.Specular != nil {
		stageTexelTextures(*m.Specular, used)
	}
	return orderedTextures(m, used)
}

// stageTexelTextures records in used the textures stage reads with Load or
// queries with TextureSize.
func stageTexelTextures(stage Stage, used map[string]bool) {
	stageMatches(stage, func(e Expr) bool {
		switch s := e.(type) {
		case Load:
			used[s.Texture] = true
		case TextureSize:
			used[s.Texture] = true
		}
		return false
	})
}

// StageTexSizeTextures returns the textures in m whose Load or TextureSize the
// given stage uses, in m.Textures order. The GLSL ES 1.00 emitters declare a
// size uniform in each stage that reads one.
func StageTexSizeTextures(m Module, stage Stage) []string {
	used := map[string]bool{}
	stageTexelTextures(stage, used)
	return orderedTextures(m, used)
}

func orderedTextures(m Module, used map[string]bool) []string {
	var out []string
	for _, t := range m.Textures {
		if used[t.Name] {
			out = append(out, t.Name)
		}
	}
	return out
}
