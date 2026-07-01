// Package bindings computes and represents the host-facing binding layout for a
// material: the std140 uniform-block byte map, vertex attributes, textures, and
// per-backend binding coordinates. It is the artifact that lets a host wire
// uniforms with zero hand-packing — the generated form of the byte layout that
// would otherwise be hand-written (and mis-aligned) per material.
package bindings

import (
	"encoding/json"

	"m31labs.dev/selena/ir"
)

const (
	// DescriptorSchemaVersion identifies the JSON binding descriptor contract.
	DescriptorSchemaVersion = "selena.descriptor.v1"
	// LanguageVersion identifies the authored .sel language contract.
	LanguageVersion = "selena.lang.v1"
)

// SurfaceKind identifies which pipeline the compiled material targets.
type SurfaceKind string

const (
	SurfaceKindMesh     SurfaceKind = "mesh"
	SurfaceKindPoints   SurfaceKind = "points"
	SurfaceKindPost     SurfaceKind = "post"
	SurfaceKindFeedback SurfaceKind = "feedback"
)

// EntryPoints names the shader entry functions in the emitted source.
// For WGSL the vertex+fragment entries are in the same source file; for GLSL/GLES
// the same names apply in the respective split files.
//
// VertexStorage is non-empty only for points WGSL modules, where a second vertex
// entry point ("vertexStorageMain") reads particle data from a storage buffer
// instead of vertex attributes. GLSL/GLES emit only the attribute variant.
type EntryPoints struct {
	Vertex        string `json:"vertex"`
	Fragment      string `json:"fragment"`
	VertexStorage string `json:"vertexStorage,omitempty"`
	// Compute is non-empty only for feedback-kind modules, where the WGSL and
	// Metal artifacts are a @compute / kernel entry (no vertex/fragment). The
	// GLSL/GLES feedback artifacts still use Vertex+Fragment (fullscreen pass).
	Compute string `json:"compute,omitempty"`
}

// Layout is the complete host descriptor for one material.
type Layout struct {
	SchemaVersion   string       `json:"schemaVersion"`
	LanguageVersion string       `json:"languageVersion"`
	Material        string       `json:"material"`
	Kind            SurfaceKind  `json:"kind"`
	EntryPoints     EntryPoints  `json:"entryPoints"`
	UniformBlock    UniformBlock `json:"uniformBlock"`
	Attributes      []Attribute  `json:"attributes"`
	Textures        []Texture    `json:"textures"`
	WGSL            WGSLBinding  `json:"wgsl"`
	Metal           MetalBinding `json:"metal"`
	// States carries the feedback-kind statefield bindings (empty otherwise).
	// Each entry tells the host to allocate a ping-pong pair: two storage
	// buffers (WebGPU) or two float FBOs sampled through one sampler2D (WebGL).
	States []StateField `json:"states,omitempty"`
	// Grid describes the kind-injected grid uniforms a feedback dispatch needs:
	// the WGSL/Metal { gridWidth, gridLen } uniform block, and the GL texelSize
	// (=1/resolution) vec2 uniform. Nil for non-feedback materials.
	Grid *GridBinding `json:"grid,omitempty"`
	// Context lists the names of the declared engine-injected uniforms (the
	// material's `context { ... }` block), in declaration order — a convenience
	// so a host can enumerate required per-frame values without scanning every
	// UniformBlock.Field for Class == "context". Empty for materials with no
	// context block.
	Context []string `json:"context,omitempty"`
}

// StateField is one feedback-kind vec4-per-cell statefield with its per-backend
// binding coordinates. WebGPU uses two storage buffers (in: read, out:
// read_write) that the host ping-pongs; WebGL/GLES sample the previous-state
// float texture through one combined sampler2D and render the next state into
// the bound float FBO (the host swaps which texture/FBO is in/out each step).
type StateField struct {
	Name  string            `json:"name"`
	WGSL  WGSLStateBinding  `json:"wgsl"`
	GL    GLStateBinding    `json:"gl"`
	Metal MetalStateBinding `json:"metal"`
}

// WGSLStateBinding gives the bind-group slots for the read (in) and read_write
// (out) storage buffers of one statefield.
type WGSLStateBinding struct {
	Group      int `json:"group"`
	InBinding  int `json:"inBinding"`
	OutBinding int `json:"outBinding"`
}

// GLStateBinding gives the combined previous-state sampler2D uniform name and
// its texture unit. The next state is written to the bound float FBO (the
// fragment output), so only the read sampler needs a name.
type GLStateBinding struct {
	Uniform string `json:"uniform"`
	Unit    int    `json:"unit"`
}

// MetalStateBinding gives the [[buffer(i)]] indices for the in/out state buffers.
type MetalStateBinding struct {
	InBuffer  int `json:"inBuffer"`
	OutBuffer int `json:"outBuffer"`
}

// GridBinding locates the kind-injected grid uniforms a feedback dispatch needs.
// WGSL/Metal read a { gridWidth:u32, gridLen:u32 } uniform block (compute index
// math); WebGL/GLES read a vec2 texelSize uniform (=1/resolution, fragment UV
// stepping). GLTexelUniform names that GL uniform.
type GridBinding struct {
	WGSL           WGSLBinding  `json:"wgsl"`
	Metal          MetalBinding `json:"metal"`
	GLTexelUniform string       `json:"glTexelUniform"`
}

// UniformBlock is the packed std140 uniform block.
type UniformBlock struct {
	Size     int            `json:"size"` // total size in bytes, rounded to 16
	Fields   []Field        `json:"fields"`
	Defaults []DefaultValue `json:"defaults,omitempty"`
}

// Field is one uniform at a computed byte offset.
//
// For scalar/vector/matrix fields, Count == 0 and Stride == 0 (omitted in
// JSON). For fixed-size array fields (B3.2), Count > 1 and Stride == 16 (the
// std140 array stride: every element padded to 16 bytes), Size == Count*Stride.
type Field struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Offset int    `json:"offset"`
	Size   int    `json:"size"`
	Count  int    `json:"count,omitempty"`  // number of elements (0 or 1 = scalar)
	Stride int    `json:"stride,omitempty"` // bytes per array element (std140: 16)
	// Class distinguishes provenance: "" (author param, the default/zero
	// value) or "context" (engine-injected, declared in a `context { ... }`
	// block — the host is expected to supply this value every frame). A
	// "context" field never carries a UniformBlock.Defaults entry, so it is
	// never surfaced as a customUniforms default. "auto" (mvp/normalMatrix) is
	// reserved for future use and not currently emitted.
	Class string `json:"class,omitempty"`
}

// DefaultValue is a host-packable default for one uniform field. Values are
// scalar components in shader order; mat3/mat4 use column-major order.
type DefaultValue struct {
	Name   string    `json:"name"`
	Type   string    `json:"type"`
	Values []float32 `json:"values"`
}

// Attribute is a per-vertex input with its assigned location.
type Attribute struct {
	Name     string `json:"name"`
	Location int    `json:"location"`
	Type     string `json:"type"`
}

// Texture is a sampled texture with its per-backend binding coordinates. The
// backends disagree on how textures bind, so the descriptor carries all three:
// WGSL/Metal split the texture and its sampler into separate bindings; GLSL/GLES
// use a single combined sampler2D/samplerCube uniform set from a texture unit.
// Dimension is "2d" for a regular 2D texture or "cube" for a cube-map texture;
// it tells the host which GPU texture view and bind-group-layout entry to use.
type Texture struct {
	Name      string          `json:"name"`
	Dimension string          `json:"dimension"` // "2d" or "cube"
	WGSL      WGSLTexBinding  `json:"wgsl"`
	GL        GLTexBinding    `json:"gl"`
	Metal     MetalTexBinding `json:"metal"`
}

// WGSLTexBinding gives the bind-group slots for the texture and its sampler.
type WGSLTexBinding struct {
	Group          int `json:"group"`
	TextureBinding int `json:"textureBinding"`
	SamplerBinding int `json:"samplerBinding"`
}

// GLTexBinding gives the sampler2D uniform name and the texture unit.
type GLTexBinding struct {
	Uniform string `json:"uniform"`
	Unit    int    `json:"unit"`
}

// MetalTexBinding gives the [[texture(i)]] / [[sampler(i)]] indices.
type MetalTexBinding struct {
	Texture int `json:"texture"`
	Sampler int `json:"sampler"`
}

// ComputeTextures assigns per-backend binding coordinates to the given 2D
// textures in order, matching the conventions the emitters use: WGSL group 0
// with the uniform block at binding 0, then texture i at 1+2i and its sampler
// at 2+2i; GL texture unit i; Metal texture/sampler index i. All textures are
// marked Dimension "2d". Use ComputeTexturesMixed when cube-map textures are
// present.
func ComputeTextures(names []string) []Texture {
	out := make([]Texture, len(names))
	for i, n := range names {
		out[i] = Texture{
			Name:      n,
			Dimension: "2d",
			WGSL:      WGSLTexBinding{Group: 0, TextureBinding: 1 + 2*i, SamplerBinding: 2 + 2*i},
			GL:        GLTexBinding{Uniform: n, Unit: i},
			Metal:     MetalTexBinding{Texture: i, Sampler: i},
		}
	}
	return out
}

// ComputeTexturesMixed assigns per-backend binding coordinates to the given
// textures in order. cubeMap maps texture names to true for cube-map textures
// (Dimension "cube") and false/absent for regular 2D textures (Dimension "2d").
// Binding arithmetic is identical to ComputeTextures; only Dimension differs.
// The GoSX host uses Dimension to select the correct texture view dimension
// (GPUTextureViewDimension "cube" vs "2d") and bind-group-layout entry type.
func ComputeTexturesMixed(names []string, cubeMap map[string]bool) []Texture {
	out := make([]Texture, len(names))
	for i, n := range names {
		dim := "2d"
		if cubeMap != nil && cubeMap[n] {
			dim = "cube"
		}
		out[i] = Texture{
			Name:      n,
			Dimension: dim,
			WGSL:      WGSLTexBinding{Group: 0, TextureBinding: 1 + 2*i, SamplerBinding: 2 + 2*i},
			GL:        GLTexBinding{Uniform: n, Unit: i},
			Metal:     MetalTexBinding{Texture: i, Sampler: i},
		}
	}
	return out
}

// WGSLBinding is where the uniform block lives in the WGSL bind-group space.
type WGSLBinding struct {
	Group   int `json:"group"`
	Binding int `json:"binding"`
}

// MetalBinding is the Metal buffer index for the uniform block.
type MetalBinding struct {
	Buffer int `json:"buffer"`
}

// NamedType is an ordered (name, type) uniform-block member input.
//
// When Count > 1 the member is a fixed-size array: ComputeUniformBlock packs
// it at std140 array stride (16 bytes per element, alignment 16). Count == 0
// is treated as a scalar (same as Count == 1).
type NamedType struct {
	Name  string
	Type  ir.Type
	Count int // 0 or 1 = scalar; >1 = fixed-size array with std140 stride 16
}

// std140 returns the (alignment, size) of t in the std140 uniform layout. The
// painful cases this centralizes: vec3 occupies 12 bytes but aligns to 16, and
// mat3 is three 16-byte-aligned columns = 48 bytes (not 36).
func std140(t ir.Type) (align, size int) {
	switch t {
	case ir.Float:
		return 4, 4
	case ir.Vec2:
		return 8, 8
	case ir.Vec3:
		return 16, 12
	case ir.Vec4:
		return 16, 16
	case ir.Mat3:
		return 16, 48
	case ir.Mat4:
		return 16, 64
	default:
		return 4, 4
	}
}

func roundUp(n, a int) int {
	if a <= 0 {
		return n
	}
	return (n + a - 1) / a * a
}

// ComputeUniformBlock packs fields under std140 rules, assigning each a byte
// offset and recording the total (16-aligned) size.
//
// Fields are packed scalars/vectors/matrices first (in declaration order),
// then fixed-size arrays (in declaration order) — NOT raw declaration order.
// This mirrors every backend emitter (WGSL/GLSL/GLES/Metal), which all render
// a material's uniform-block struct as its scalar members followed by its
// array members (see lower.toBindings/toArrayBindings, which split a
// declaration-ordered field list into exactly those two groups for the
// emitters to concatenate). If ComputeUniformBlock instead offset fields in
// raw declaration order, a material declaring a scalar after an array would
// get a descriptor whose offsets disagree with the emitted struct's actual
// member order — a host packing the uniform buffer per the descriptor would
// write scalar fields to the wrong bytes.
//
// When f.Count > 1 the field is a fixed-size array: every element is padded
// to 16 bytes (std140 array stride), and the array itself aligns to 16 bytes.
// This is correct for all element types (float, vec2, vec3, vec4).
func ComputeUniformBlock(fields []NamedType) UniformBlock {
	ordered := make([]NamedType, 0, len(fields))
	for _, f := range fields {
		if f.Count <= 1 {
			ordered = append(ordered, f)
		}
	}
	for _, f := range fields {
		if f.Count > 1 {
			ordered = append(ordered, f)
		}
	}

	offset := 0
	out := make([]Field, 0, len(ordered))
	for _, f := range ordered {
		if f.Count > 1 {
			// std140 array: align to 16, stride 16 per element.
			const arrayStride = 16
			offset = roundUp(offset, 16)
			size := f.Count * arrayStride
			out = append(out, Field{
				Name:   f.Name,
				Type:   string(f.Type),
				Offset: offset,
				Size:   size,
				Count:  f.Count,
				Stride: arrayStride,
			})
			offset += size
		} else {
			a, sz := std140(f.Type)
			offset = roundUp(offset, a)
			out = append(out, Field{Name: f.Name, Type: string(f.Type), Offset: offset, Size: sz})
			offset += sz
		}
	}
	return UniformBlock{Size: roundUp(offset, 16), Fields: out}
}

// JSON renders the descriptor as indented JSON for embedding/host consumption.
func (l Layout) JSON() (string, error) {
	b, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// MarshalJSON fills version metadata for hand-built Layout values too.
func (l Layout) MarshalJSON() ([]byte, error) {
	type layout Layout
	out := layout(l)
	if out.SchemaVersion == "" {
		out.SchemaVersion = DescriptorSchemaVersion
	}
	if out.LanguageVersion == "" {
		out.LanguageVersion = LanguageVersion
	}
	return json.Marshal(out)
}
