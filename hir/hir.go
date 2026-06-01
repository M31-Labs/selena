// Package hir is Selena's high-level, author-facing material model — the level
// a human (or grammar front-end) targets. A Material declares params (its
// inputs, including stdlib record types like Sun) and a surface function (its
// per-fragment response), with an optional vertex hook. Stages, varyings,
// binding numbers, uniform-buffer layout, and per-backend dialects do not exist
// here; package lower resolves all of that into the low-level ir.Module.
package hir

// Type is a high-level value or stdlib record type. Color lowers to vec3; Sun is
// a stdlib record expanded into uniforms during lowering.
type Type string

const (
	Float     Type = "float"
	Vec2      Type = "vec2"
	Vec3      Type = "vec3"
	Vec4      Type = "vec4"
	Mat3      Type = "mat3"
	Mat4      Type = "mat4"
	Color     Type = "color"     // -> vec3 at the LIR
	Sun       Type = "Sun"       // stdlib record: { dir: vec3, ambient: float }
	Texture2D Type = "texture2d" // M2
)

// Position is a 1-based source position with a 0-based byte offset.
type Position struct {
	Offset int
	Line   int
	Column int
}

// Span is a half-open source range. Zero values mean "unknown".
type Span struct {
	Start Position
	End   Position
}

// IsZero reports whether s has source information.
func (s Span) IsZero() bool {
	return s.Start.Line == 0 && s.End.Line == 0 && s.Start.Offset == 0 && s.End.Offset == 0
}

// Program is the whole parsed unit: reusable functions plus materials.
type Program struct {
	Funcs     []FuncDecl
	Materials []Material
}

// FuncDecl is a reusable top-level function: fn name(p: T, …) -> R { … }. It is
// inlined at call sites during lowering (params and locals substituted), so it
// composes without runtime cost and needs no backend function support.
type FuncDecl struct {
	Name    string
	Span    Span
	Params  []Param // typed parameters
	Body    []Let
	Result  Expr
	Returns Type
}

// Material is one authored material.
type Material struct {
	Name    string
	Extends string // parent material name, or "" — resolved during lowering
	Span    Span
	Params  []Param
	Surface Func  // surface(geo) -> color
	Vertex  *Func // optional geometry hook; nil = use the default transform
}

// Param is a material input. Defaults are deferred to a later milestone.
type Param struct {
	Name    string
	Type    Type
	Default Expr
	Span    Span
}

// Func is a surface or vertex body: a binding name for the geometry record,
// local bindings, and (for surface) a result expression.
type Func struct {
	Geo    string // geometry record binding name (e.g. "geo")
	Span   Span
	Body   []Let
	Result Expr // surface: the color expression; vertex hooks leave this nil
}

// Let binds a local to an expression: `let Name = Value`.
type Let struct {
	Name  string
	Value Expr
	Span  Span
}

// Expr is the high-level expression tree.
type Expr interface{ isExpr() }

// Ref references a param or a local by name.
type Ref struct {
	Name string
	Span Span
}

// Lit is a scalar float literal.
type Lit struct {
	Value float64
	Span  Span
}

// Member is field access on a record: geo.worldNormal, light.dir.
type Member struct {
	E     Expr
	Field string
	Span  Span
}

// Call is a builtin or stdlib function call: normalize, dot, max, rgb, sample…
type Call struct {
	Func string
	Args []Expr
	Span Span
}

// Binary is an infix operator: + - * /.
type Binary struct {
	Op   string
	L, R Expr
	Span Span
}

// SuperCall is super.<Method>(args) — calls the parent material's method (only
// surface in v1). Resolved during lowering by inlining the parent's surface.
type SuperCall struct {
	Method string
	Args   []Expr
	Span   Span
}

func (Ref) isExpr()       {}
func (Lit) isExpr()       {}
func (Member) isExpr()    {}
func (Call) isExpr()      {}
func (Binary) isExpr()    {}
func (SuperCall) isExpr() {}

// ExprSpan returns source information for an expression when it came from
// parsed source.
func ExprSpan(e Expr) Span {
	switch x := e.(type) {
	case Ref:
		return x.Span
	case Lit:
		return x.Span
	case Member:
		return x.Span
	case Call:
		return x.Span
	case Binary:
		return x.Span
	case SuperCall:
		return x.Span
	default:
		return Span{}
	}
}
