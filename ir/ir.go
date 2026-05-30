package ir

import "strconv"

// Type is a shader value type. The set is intentionally small for the first
// slice; it grows as the language does.
type Type string

const (
	Float Type = "float"
	Vec2  Type = "vec2"
	Vec3  Type = "vec3"
	Vec4  Type = "vec4"
	Mat3  Type = "mat3"
	Mat4  Type = "mat4"
)

// Module is a complete material shader: a declared interface (uniforms,
// per-vertex attributes, vertex->fragment varyings) plus the two stages.
type Module struct {
	Name       string
	Uniforms   []Binding // shared constants (mvp, color, light, ...)
	Attributes []Binding // per-vertex inputs (position, normal, uv, ...)
	Varyings   []Binding // interpolated vertex -> fragment values
	Vertex     Stage
	Fragment   Stage
}

// Binding is a named, typed interface slot.
type Binding struct {
	Name string
	Type Type
}

// Stage is one shader stage: a sequence of assignments followed by the stage's
// output expression. The vertex Output is clip-space position (vec4); the
// fragment Output is the fragment color (vec4).
type Stage struct {
	Body   []Stmt
	Output Expr
}

// Stmt assigns Value to Target. Target is either a declared varying (written in
// the vertex stage) or a stage-local. Type is the value's type, carried so
// backends that need explicit local declarations (GLSL) can emit them without
// inference — a deliberate slice simplification; the front-end will infer it.
type Stmt struct {
	Target string
	Type   Type
	Value  Expr
}

// Expr is the typed expression graph. It is total by construction (no loops,
// no recursion) so every backend can emit it deterministically.
type Expr interface{ isExpr() }

// Ref references a uniform, attribute, varying, or stage-local by name.
type Ref struct{ Name string }

// Lit is a scalar float literal.
type Lit struct{ Value float64 }

// Construct builds a vector/matrix value, e.g. vec4(rgb, 1.0).
type Construct struct {
	Type Type
	Args []Expr
}

// Call is a builtin function call whose name is identical across backends
// (normalize, dot, max, mix, clamp, ...).
type Call struct {
	Func string
	Args []Expr
}

// Binary is an infix operator shared across backends: + - * / (including
// matrix*vector and vector*scalar).
type Binary struct {
	Op   string
	L, R Expr
}

// Swizzle selects components, e.g. position.xyz or color.rgb.
type Swizzle struct {
	E     Expr
	Field string
}

func (Ref) isExpr()       {}
func (Lit) isExpr()       {}
func (Construct) isExpr() {}
func (Call) isExpr()      {}
func (Binary) isExpr()    {}
func (Swizzle) isExpr()   {}

// Dialect spells the backend-specific parts of expression printing: how a type
// name renders (vec4 vs vec4<f32>) and how a reference renders in the current
// stage scope (a uniform as u.x in WGSL vs a bare global in GLSL). Operators,
// builtins, literals, and the overall expression structure are shared — see
// Print. The two backends differing only in Dialect (and stage scaffolding) is
// the property this slice is meant to validate.
type Dialect interface {
	TypeName(Type) string
	Ref(name string) string
}

// Print renders e using d for the backend-specific spellings.
func Print(e Expr, d Dialect) string {
	switch x := e.(type) {
	case Ref:
		return d.Ref(x.Name)
	case Lit:
		return formatFloat(x.Value)
	case Construct:
		return d.TypeName(x.Type) + "(" + printArgs(x.Args, d) + ")"
	case Call:
		return x.Func + "(" + printArgs(x.Args, d) + ")"
	case Binary:
		return "(" + Print(x.L, d) + " " + x.Op + " " + Print(x.R, d) + ")"
	case Swizzle:
		return Print(x.E, d) + "." + x.Field
	default:
		return "/* unknown expr */"
	}
}

func printArgs(args []Expr, d Dialect) string {
	parts := make([]string, len(args))
	for i, a := range args {
		parts[i] = Print(a, d)
	}
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += ", "
		}
		out += p
	}
	return out
}

// formatFloat renders a float literal with a decimal point, as both WGSL and
// GLSL require for float constants (1 -> "1.0").
func formatFloat(v float64) string {
	s := strconv.FormatFloat(v, 'f', -1, 64)
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			return s
		}
	}
	return s + ".0"
}
