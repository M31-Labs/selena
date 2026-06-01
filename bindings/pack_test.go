package bindings

import (
	"encoding/binary"
	"math"
	"strings"
	"testing"

	"m31labs.dev/selena/ir"
)

func TestPackUniformsStd140(t *testing.T) {
	layout := Layout{
		Material: "Packed",
		UniformBlock: ComputeUniformBlock([]NamedType{
			{Name: "mvp", Type: ir.Mat4},
			{Name: "normalMatrix", Type: ir.Mat3},
			{Name: "baseColor", Type: ir.Vec3},
			{Name: "exposure", Type: ir.Float},
		}),
	}

	buf, err := PackUniforms(layout, map[string]any{
		"mvp":          seq(16),
		"normalMatrix": [9]float32{1, 2, 3, 4, 5, 6, 7, 8, 9},
		"baseColor":    []float64{0.25, 0.5, 0.75},
		"exposure":     2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(buf) != layout.UniformBlock.Size {
		t.Fatalf("buffer length = %d, want %d", len(buf), layout.UniformBlock.Size)
	}

	if got := f32(buf, 0); got != 1 {
		t.Fatalf("mvp[0] = %v, want 1", got)
	}
	if got := f32(buf, 60); got != 16 {
		t.Fatalf("mvp[15] = %v, want 16", got)
	}

	// mat3 uses three 16-byte std140 columns, leaving a 4-byte pad per column.
	if got := f32(buf, 64); got != 1 {
		t.Fatalf("normalMatrix col0 row0 = %v, want 1", got)
	}
	if got := f32(buf, 64+8); got != 3 {
		t.Fatalf("normalMatrix col0 row2 = %v, want 3", got)
	}
	if got := f32(buf, 64+12); got != 0 {
		t.Fatalf("normalMatrix col0 padding = %v, want 0", got)
	}
	if got := f32(buf, 64+16); got != 4 {
		t.Fatalf("normalMatrix col1 row0 = %v, want 4", got)
	}
	if got := f32(buf, 64+40); got != 9 {
		t.Fatalf("normalMatrix col2 row2 = %v, want 9", got)
	}

	// The scalar packs into the vec3 tail, matching ComputeUniformBlock.
	if got := f32(buf, 112); got != 0.25 {
		t.Fatalf("baseColor.r = %v, want 0.25", got)
	}
	if got := f32(buf, 124); got != 2 {
		t.Fatalf("exposure = %v, want 2", got)
	}
}

func TestPackUniformsRejectsBadInputs(t *testing.T) {
	block := ComputeUniformBlock([]NamedType{
		{Name: "baseColor", Type: ir.Vec3},
		{Name: "roughness", Type: ir.Float},
	})

	cases := []struct {
		name   string
		values map[string]any
		want   string
	}{
		{
			name:   "missing",
			values: map[string]any{"baseColor": []float32{1, 1, 1}},
			want:   `missing uniform "roughness"`,
		},
		{
			name: "wrong component count",
			values: map[string]any{
				"baseColor": []float32{1, 1},
				"roughness": 0.5,
			},
			want: `vec3 needs 3 components, got 2`,
		},
		{
			name: "unsupported value type",
			values: map[string]any{
				"baseColor": "red",
				"roughness": 0.5,
			},
			want: `unsupported value type string`,
		},
		{
			name: "non finite",
			values: map[string]any{
				"baseColor": []float32{1, 1, 1},
				"roughness": math.Inf(1),
			},
			want: `non-finite value`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := PackUniformBlock(block, tc.values)
			if err == nil {
				t.Fatal("PackUniformBlock succeeded, want error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want substring %q", err, tc.want)
			}
		})
	}
}

func seq(n int) []float32 {
	out := make([]float32, n)
	for i := range out {
		out[i] = float32(i + 1)
	}
	return out
}

func f32(buf []byte, offset int) float32 {
	return math.Float32frombits(binary.LittleEndian.Uint32(buf[offset : offset+4]))
}
