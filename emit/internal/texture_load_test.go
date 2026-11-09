package internal

import (
	"testing"

	"m31labs.dev/prism/dialect"
	"m31labs.dev/selena/ir"
)

// TestTexelDialectSpellings pins how each backend spells load() and size().
func TestTexelDialectSpellings(t *testing.T) {
	cases := []struct {
		name       string
		d          Resolver
		load, size string
	}{
		{"wgsl", Resolver{Dialect: dialect.WGSL{}},
			"textureLoad(t, vec2<i32>(ix, iy), 0)", "vec2<f32>(textureDimensions(t))"},
		{"gles", Resolver{Dialect: dialect.GLES{}},
			"texelFetch(t, ivec2(ix, iy), 0)", "vec2(textureSize(t, 0))"},
		{"glsl", Resolver{Dialect: dialect.GLSL{}},
			"texture2D(t, (vec2(ix, iy) + 0.5) / _texSize_t)", "_texSize_t"},
		{"metal", Resolver{Dialect: dialect.Metal{}},
			"t.read(uint2(ix, iy), 0)", "float2(t.get_width(), t.get_height())"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ir.Print(ir.Load{Texture: "t", X: ir.Ref{Name: "ix"}, Y: ir.Ref{Name: "iy"}}, tc.d); got != tc.load {
				t.Errorf("load = %q, want %q", got, tc.load)
			}
			if got := ir.Print(ir.TextureSize{Texture: "t"}, tc.d); got != tc.size {
				t.Errorf("size = %q, want %q", got, tc.size)
			}
		})
	}
}
