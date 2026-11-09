package bindings

// Screen materials (`kind screen`): the descriptor fields that are specific to
// them. Selena declares the pipeline state; the host applies it.

// SurfaceKindScreen is a fullscreen pass that draws a color with alpha into a
// host-chosen rectangle, without reading the scene color.
const SurfaceKindScreen SurfaceKind = "screen"

// ScreenSizeUniform is the uniform-block field a screen material gets when its
// body calls screenSize(). The host writes the size in pixels of the target
// rectangle it draws into, as a vec2. The field is tagged class "context" and
// listed in Layout.Context, like any other value the host supplies per draw.
const ScreenSizeUniform = "screenSize"

// Blend modes a screen material may declare in its render block.
const (
	BlendOpaque        = "opaque"        // replace the destination; alpha is ignored
	BlendAlpha         = "alpha"         // src*a + dst*(1-a), straight (non-premultiplied) alpha
	BlendPremultiplied = "premultiplied" // src + dst*(1-a), the shader returns rgb already multiplied by a
	BlendAdditive      = "additive"      // src*a + dst
)

// BlendModes lists the blend words in documentation order.
var BlendModes = []string{BlendOpaque, BlendAlpha, BlendPremultiplied, BlendAdditive}

// Depth words a screen material may declare in its render block.
const (
	DepthOff   = "off"   // no depth test, no depth write
	DepthTest  = "test"  // depth test, no depth write
	DepthWrite = "write" // depth test and depth write
)

// DepthModes lists the depth words in documentation order.
var DepthModes = []string{DepthOff, DepthTest, DepthWrite}

// PipelineState is the fixed-function state a screen material declares. The
// host applies it when it builds the pipeline. All three fields are always
// present when Layout.Pipeline is.
//
// The blend factors, with s the shader result and d the destination:
//
//	opaque         result = s.rgb               (blending off)
//	alpha          result = s.rgb*s.a + d*(1-s.a)
//	premultiplied  result = s.rgb + d*(1-s.a)
//	additive       result = s.rgb*s.a + d
//
// The alpha channel uses source factor one and destination factor
// one-minus-source-alpha for alpha and premultiplied, one and one for additive,
// and is written as s.a for opaque. A host that draws into an opaque swapchain
// never reads it.
type PipelineState struct {
	// Blend is one of BlendModes.
	Blend string `json:"blend"`
	// DepthTest is true when the pass tests against the depth buffer
	// (depth word "test" or "write").
	DepthTest bool `json:"depthTest"`
	// DepthWrite is true when the pass also writes depth (depth word "write").
	DepthWrite bool `json:"depthWrite"`
}

// PipelineFor returns the PipelineState for a blend word and a depth word. The
// words must come from BlendModes and DepthModes.
func PipelineFor(blend, depth string) PipelineState {
	return PipelineState{
		Blend:      blend,
		DepthTest:  depth == DepthTest || depth == DepthWrite,
		DepthWrite: depth == DepthWrite,
	}
}
