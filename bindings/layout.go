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

// Layout is the complete host descriptor for one material.
type Layout struct {
	Material     string       `json:"material"`
	UniformBlock UniformBlock `json:"uniformBlock"`
	Attributes   []Attribute  `json:"attributes"`
	Textures     []Texture    `json:"textures"`
	WGSL         WGSLBinding  `json:"wgsl"`
	Metal        MetalBinding `json:"metal"`
}

// UniformBlock is the packed std140 uniform block.
type UniformBlock struct {
	Size   int     `json:"size"` // total size in bytes, rounded to 16
	Fields []Field `json:"fields"`
}

// Field is one uniform at a computed byte offset.
type Field struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Offset int    `json:"offset"`
	Size   int    `json:"size"`
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
// use a single combined sampler2D uniform set from a texture unit.
type Texture struct {
	Name  string          `json:"name"`
	WGSL  WGSLTexBinding  `json:"wgsl"`
	GL    GLTexBinding    `json:"gl"`
	Metal MetalTexBinding `json:"metal"`
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

// ComputeTextures assigns per-backend binding coordinates to the given textures
// in order, matching the conventions the emitters use: WGSL group 0 with the
// uniform block at binding 0, then texture i at 1+2i and its sampler at 2+2i;
// GL texture unit i; Metal texture/sampler index i.
func ComputeTextures(names []string) []Texture {
	out := make([]Texture, len(names))
	for i, n := range names {
		out[i] = Texture{
			Name:  n,
			WGSL:  WGSLTexBinding{Group: 0, TextureBinding: 1 + 2*i, SamplerBinding: 2 + 2*i},
			GL:    GLTexBinding{Uniform: n, Unit: i},
			Metal: MetalTexBinding{Texture: i, Sampler: i},
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
type NamedType struct {
	Name string
	Type ir.Type
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

// ComputeUniformBlock packs fields in declaration order under std140 rules,
// assigning each a byte offset and recording the total (16-aligned) size.
func ComputeUniformBlock(fields []NamedType) UniformBlock {
	offset := 0
	out := make([]Field, 0, len(fields))
	for _, f := range fields {
		a, sz := std140(f.Type)
		offset = roundUp(offset, a)
		out = append(out, Field{Name: f.Name, Type: string(f.Type), Offset: offset, Size: sz})
		offset += sz
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
