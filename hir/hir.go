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

// Material is one authored material.
type Material struct {
	Name    string
	Params  []Param
	Surface Func  // surface(geo) -> color
	Vertex  *Func // optional geometry hook; nil = use the default transform
}

// Param is a material input. Defaults are deferred to a later milestone.
type Param struct {
	Name string
	Type Type
}

// Func is a surface or vertex body: a binding name for the geometry record,
// local bindings, and (for surface) a result expression.
type Func struct {
	Geo    string // geometry record binding name (e.g. "geo")
	Body   []Let
	Result Expr // surface: the color expression; vertex hooks leave this nil
}

// Let binds a local to an expression: `let Name = Value`.
type Let struct {
	Name  string
	Value Expr
}

// Expr is the high-level expression tree.
type Expr interface{ isExpr() }

// Ref references a param or a local by name.
type Ref struct{ Name string }

// Lit is a scalar float literal.
type Lit struct{ Value float64 }

// Member is field access on a record: geo.worldNormal, light.dir.
type Member struct {
	E     Expr
	Field string
}

// Call is a builtin or stdlib function call: normalize, dot, max, rgb, sample…
type Call struct {
	Func string
	Args []Expr
}

// Binary is an infix operator: + - * /.
type Binary struct {
	Op   string
	L, R Expr
}

func (Ref) isExpr()    {}
func (Lit) isExpr()    {}
func (Member) isExpr() {}
func (Call) isExpr()   {}
func (Binary) isExpr() {}
