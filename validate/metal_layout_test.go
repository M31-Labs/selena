package validate

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var (
	metalStructRE = regexp.MustCompile(`(?s)struct (Uniforms|UserUniforms) \{\n(.*?)\n\};`)
	metalMemberRE = regexp.MustCompile(`^\s+(\w+) (\w+)(?:\[(\d+)\])?;$`)
)

// uniformStructName returns the name of the struct that holds the material's
// uniform block in its Metal source.
func uniformStructName(t *testing.T, m metalMaterial) string {
	t.Helper()
	sm := metalStructRE.FindStringSubmatch(m.source)
	if sm == nil {
		t.Fatalf("%s: no uniform struct in the Metal source", m.name)
	}
	return sm[1]
}

// mslSizeAlign is the Metal Shading Language size and alignment of a member
// type, as the Metal Shading Language Specification gives them.
var mslSizeAlign = map[string][2]int{
	"float": {4, 4}, "int": {4, 4}, "uint": {4, 4}, "int4": {16, 16}, "uint4": {16, 16},
	"float2": {8, 8}, "float3": {16, 16}, "packed_float3": {12, 4}, "float4": {16, 16},
	"float3x3": {48, 16}, "float4x4": {64, 16},
	"char": {1, 1},
}

// TestMetalUniformLayoutMatchesDescriptor computes the member offsets and the
// size of the emitted Metal uniform struct in Go and compares them with the
// binding descriptor. It runs on every host; the Mac test proves the same
// facts with the real compiler.
func TestMetalUniformLayoutMatchesDescriptor(t *testing.T) {
	checked := 0
	kinds := map[string]bool{}
	for _, m := range metalCorpus(t) {
		block := m.result.Layout.UniformBlock
		if len(block.Fields) == 0 {
			continue
		}
		checked++
		kinds[string(m.result.Layout.Kind)] = true
		t.Run(m.name, func(t *testing.T) {
			sm := metalStructRE.FindStringSubmatch(m.source)
			if sm == nil {
				t.Fatal("no uniform struct in the Metal source")
			}
			offsets := map[string]int{}
			cursor, maxAlign := 0, 1
			for _, line := range strings.Split(sm[2], "\n") {
				mm := metalMemberRE.FindStringSubmatch(line)
				if mm == nil {
					t.Fatalf("cannot parse struct member %q", line)
				}
				typ, name := mm[1], mm[2]
				sa, ok := mslSizeAlign[typ]
				if !ok {
					t.Fatalf("no size for Metal type %q", typ)
				}
				count := 1
				if mm[3] != "" {
					count, _ = strconv.Atoi(mm[3])
				}
				cursor = (cursor + sa[1] - 1) / sa[1] * sa[1]
				offsets[name] = cursor
				cursor += sa[0] * count
				if sa[1] > maxAlign {
					maxAlign = sa[1]
				}
			}
			size := (cursor + maxAlign - 1) / maxAlign * maxAlign
			for _, f := range block.Fields {
				got, ok := offsets[f.Name]
				if !ok {
					t.Errorf("%s: not declared in the Metal struct", f.Name)
				} else if got != f.Offset {
					t.Errorf("%s: Metal offset %d, descriptor offset %d", f.Name, got, f.Offset)
				}
			}
			if size != block.Size {
				t.Errorf("Metal struct size %d, descriptor size %d", size, block.Size)
			}
		})
	}
	if checked == 0 {
		t.Fatal("no material with uniforms was checked")
	}
	// The Metal target has no surface hook, so surface materials are not here.
	for _, kind := range []string{"mesh", "points", "post", "feedback", "screen"} {
		if !kinds[kind] {
			t.Errorf("no %s material with uniforms in the corpus", kind)
		}
	}
	t.Logf("checked %d materials, kinds %v", checked, kinds)
}
