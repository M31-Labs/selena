package ir

import (
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

type testDialect struct{}

func (testDialect) TypeName(t Type) string { return string(t) }
func (testDialect) Ref(name string) string { return name }
func (testDialect) Call(name string, args []string) string {
	return "builtin::" + name + "(" + strings.Join(args, " | ") + ")"
}
func (testDialect) Sample(tex, uv string) string {
	return "sample(" + tex + ", " + uv + ")"
}
func (testDialect) SceneSample(name, uv string) string {
	return "sceneSample(" + name + ", " + uv + ")"
}
