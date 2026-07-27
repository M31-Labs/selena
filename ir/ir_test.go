package ir

import (
	"strconv"
	"strings"
	"testing"
)

func TestPrintUsesDialectCallSpelling(t *testing.T) {
	got := Print(Call{Func: "mix", Args: []Expr{
		Ref{Name: "a"},
		Ref{Name: "b"},
		Lit{Value: 0.5},
	}}, testDialect{})
	want := "builtin::mix(a | b | 0.5)"
	if got != want {
		t.Fatalf("Print(Call) = %q, want %q", got, want)
	}
}

func TestPrintSampleLevel(t *testing.T) {
	got := Print(SampleLevel{
		Texture: "albedo",
		UV:      Ref{Name: "uv"},
		LOD:     Lit{Value: 0.0},
	}, testDialect{})
	want := "sampleLevel(albedo, uv, 0.0)"
	if got != want {
		t.Fatalf("Print(SampleLevel) = %q, want %q", got, want)
	}
}

// TestPrintCellUVSwizzleParenthesizes is the regression test for a bug where
// Print's Swizzle case appended ".field" straight to Dialect.CellUV()'s
// output with no parentheses. WGSL's and Metal's real CellUVFn render CellUV
// as an unparenthesized division expression (see emit/wgsl and emit/metal),
// so cell.uv.x used to print as "a / b.x" — swizzling the denominator —
// instead of "(a / b).x". compoundCellUVDialect reproduces that shape
// directly, without a full compile.
func TestPrintCellUVSwizzleParenthesizes(t *testing.T) {
	got := Print(Swizzle{E: CellUV{}, Field: "x"}, compoundCellUVDialect{})
	want := "(a / b).x"
	if got != want {
		t.Fatalf("Print(Swizzle{E: CellUV{}}) = %q, want %q", got, want)
	}
}

// TestPrintCellUVBareUnaffected guards against an over-broad fix: a bare
// (non-swizzled) CellUV must print exactly what the dialect returns, with no
// added parens. Every existing feedback material binds `let uv = cell.uv`
// this way, and testdata/conformance goldens pin that output byte-for-byte —
// parenthesizing CellUVFn's return value at the source (rather than only in
// Print's Swizzle case) was the first fix attempted here and it moved three
// existing goldens; this test is what a source-level fix would have failed.
func TestPrintCellUVBareUnaffected(t *testing.T) {
	got := Print(CellUV{}, compoundCellUVDialect{})
	want := "a / b"
	if got != want {
		t.Fatalf("Print(CellUV{}) = %q, want %q (no parens added when not swizzled)", got, want)
	}
}

// compoundCellUVDialect wraps testDialect but overrides CellUV to return an
// unparenthesized division expression, matching the real WGSL/Metal dialects'
// CellUVFn shape (every other testDialect method already renders an atomic
// form, so it is not representative of the bug on its own).
type compoundCellUVDialect struct{ testDialect }

func (compoundCellUVDialect) CellUV() string { return "a / b" }

type testDialect struct{}

func (testDialect) TypeName(t Type) string { return string(t) }
func (testDialect) Ref(name string) string { return name }
func (testDialect) Call(name string, args []string) string {
	return "builtin::" + name + "(" + strings.Join(args, " | ") + ")"
}
func (testDialect) CallTyped(name string, args []string, _ []Type) string {
	return "builtin::" + name + "(" + strings.Join(args, " | ") + ")"
}
func (testDialect) Sample(tex, uv string) string {
	return "sample(" + tex + ", " + uv + ")"
}
func (testDialect) SampleLevel(tex, uv, lod string) string {
	return "sampleLevel(" + tex + ", " + uv + ", " + lod + ")"
}
func (testDialect) SampleCube(tex, dir string) string {
	return "sampleCube(" + tex + ", " + dir + ")"
}
func (testDialect) SceneSample(name, uv string) string {
	return "sceneSample(" + name + ", " + uv + ")"
}
func (testDialect) StateSample(dx, dy int64) string {
	return "stateSample(" + strconv.FormatInt(dx, 10) + ", " + strconv.FormatInt(dy, 10) + ")"
}
func (testDialect) StateSampleUV(uv string) string {
	return "stateSampleUV(" + uv + ")"
}
func (testDialect) Ternary(cond, then, alt string) string {
	return "(" + cond + " ? " + then + " : " + alt + ")"
}
func (testDialect) IntLit(v int64) string   { return strconv.FormatInt(v, 10) }
func (testDialect) UintLit(v uint64) string { return strconv.FormatUint(v, 10) }
func (testDialect) CellUV() string          { return "cellUV" }
func (testDialect) Discard() string         { return "discard;" }
