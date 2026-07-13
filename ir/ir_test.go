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
