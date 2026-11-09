package parse

import (
	"testing"

	"m31labs.dev/selena/hir"
)

func TestParseInstanceDeclarations(t *testing.T) {
	m, err := Material([]byte(`material NoteQuad {
    instance rect : vec4
    instance region : int
    varying vTone : vec4
    vertex(geo) -> vec4 { return mvp * vec4f(rect.xy, 0.0, 1.0) }
    surface(geo) -> color { return rgb(1.0, 1.0, 1.0) }
}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Instances) != 2 {
		t.Fatalf("instances = %+v", m.Instances)
	}
	if m.Instances[0].Name != "rect" || m.Instances[0].Type != hir.Vec4 ||
		m.Instances[1].Name != "region" || m.Instances[1].Type != hir.Int {
		t.Errorf("instances = %+v", m.Instances)
	}
	if m.Instances[0].Span.Start.Line != 2 {
		t.Errorf("span line = %d, want 2", m.Instances[0].Span.Start.Line)
	}
}

func TestParseInstanceNeedsNameAndType(t *testing.T) {
	_, err := Material([]byte("material M {\n    instance rect\n    surface(geo) -> color { return rgb(1.0, 1.0, 1.0) }\n}"))
	if err == nil {
		t.Fatal("an instance without a type parsed")
	}
}

// bitOp parses `let x = <expr>` in a surface and returns the expression.
func parseExpr(t *testing.T, expr string) hir.Expr {
	t.Helper()
	m, err := Material([]byte("material M {\n    surface(geo) -> color {\n        let x = " + expr + "\n        return rgb(1.0, 1.0, 1.0)\n    }\n}"))
	if err != nil {
		t.Fatalf("%s: %v", expr, err)
	}
	return m.Surface.Body[0].(hir.Let).Value
}

// shape renders an expression tree with every binary node in parentheses.
func shape(e hir.Expr) string {
	switch x := e.(type) {
	case hir.Binary:
		return "(" + shape(x.L) + " " + x.Op + " " + shape(x.R) + ")"
	case hir.Ref:
		return x.Name
	case hir.UintLit:
		return "u"
	case hir.IntLit:
		return "i"
	case hir.Lit:
		return "f"
	}
	return "?"
}

// Bit operators follow Go's precedence: & << >> bind like *, | ^ like +, and
// both groups bind tighter than comparisons.
func TestParseBitOperatorPrecedence(t *testing.T) {
	cases := map[string]string{
		"a & 1u":              "(a & u)",
		"a | b ^ c":           "((a | b) ^ c)",
		"a >> 1u":             "(a >> u)",
		"a << 2u":             "(a << u)",
		"a & 1u == 0u":        "((a & u) == u)",
		"a + b & c":           "(a + (b & c))",
		"a | b & c":           "(a | (b & c))",
		"a * b << c":          "((a * b) << c)",
		"a | b + c":           "((a | b) + c)",
		"a >> 1u & 1u":        "((a >> u) & u)",
		"a < b && c & d == e": "((a < b) && ((c & d) == e))",
		"a || b && c":         "(a || (b && c))",
		"a & b || c | d":      "((a & b) || (c | d))",
		"(a | b) & c":         "((a | b) & c)",
		"a < 1.0":             "(a < f)",
		"a > b":               "(a > b)",
	}
	for src, want := range cases {
		if got := shape(parseExpr(t, src)); got != want {
			t.Errorf("%s parsed as %s, want %s", src, got, want)
		}
	}
}
