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
	Int       Type = "int"
	Uint      Type = "uint"
	Vec2      Type = "vec2"
	Vec3      Type = "vec3"
	Vec4      Type = "vec4"
	Mat3      Type = "mat3"
	Mat4      Type = "mat4"
	Color       Type = "color"       // -> vec3 at the LIR
	Sun         Type = "Sun"         // stdlib record: { dir: vec3, ambient: float }
	Texture2D   Type = "texture2d"   // M2
	TextureCube Type = "textureCube" // cube-map texture; sampled with sampleCube(tex, dir) -> vec4
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

// Kind identifies which surface pipeline a material targets.
type Kind string

const (
	// KindMesh is the default mesh/triangle pipeline (vertex + fragment stages).
	KindMesh Kind = "mesh"
	// KindPoints is a billboard point/particle appearance surface.
	KindPoints Kind = "points"
	// KindPost is a fullscreen post-process pass.
	KindPost Kind = "post"
	// KindFeedback is a per-cell simulation step over a statefield grid. One
	// authored feedback(cell) -> vec4 body lowers to BOTH a WGSL @compute kernel
	// (WebGPU) and a GLSL/GLES fullscreen fragment ping-pong pass (WebGL2).
	KindFeedback Kind = "feedback"
)

// StateField declares a vec4-per-cell state grid (`state <name>`). The host
// allocates a ping-pong pair: two storage buffers on WebGPU, two float FBOs on
// WebGL/GLES. The feedback body reads the previous state via state(dx, dy).
type StateField struct {
	Name string
	Span Span
}

// Varying declares an author-controlled vertex->fragment interpolant
// (`varying <name> : <type>`). It is written by the material's vertex() body and
// read by the surface() fragment as geo.<name>. Only mesh/general materials that
// author a vertex() hook may declare varyings. B4.
type Varying struct {
	Name string
	Type Type
	Span Span
}

// Material is one authored material.
type Material struct {
	Name    string
	Kind    Kind   // defaults to KindMesh when unset
	Extends string // parent material name, or "" — resolved during lowering
	Span    Span
	Params  []Param
	Surface Func  // surface(geo) -> color; for KindFeedback this holds feedback(cell) -> vec4
	Vertex  *Func // optional geometry hook; nil = use the default transform
	// States lists the declared statefields. KindFeedback uses these for the
	// state(dx, dy) gather; mesh/general materials may declare one statefield to
	// read via stateAt(uv) in the vertex/fragment stages (B4).
	States []StateField
	// Varyings lists the author-declared vertex->fragment interpolants written by
	// Vertex and read by Surface as geo.<name>. Empty unless a vertex() is authored. B4.
	Varyings []Varying
	// Context lists the declared engine-injected uniforms (`context { ... }`).
	// Unlike Params, a context field's value is supplied by the host every frame;
	// it lowers into the SAME uniform block as params (appended after them) but
	// is tagged Class:"context" in the compiled bindings.Layout and is excluded
	// from author-facing customUniforms defaults (the "context uniform" design,
	// §3.1/§3.2). At most one `context { ... }` block is authored per material;
	// its fields are flattened here in declaration order.
	Context []ContextField
}

// Param is a material input. Defaults are deferred to a later milestone.
//
// When IsArray is true, Type is the element type (e.g. Vec4) and ArraySize is
// the fixed number of elements. Array params lower to uniform array fields in
// the UBO; indexing them with [i] in the surface body returns the element type.
// Array params do not support default values.
type Param struct {
	Name      string
	Type      Type
	Default   Expr
	Span      Span
	IsArray   bool // true: this is a fixed-size array param (B3.2)
	ArraySize int  // element count when IsArray == true
}

// ContextField is one field declared inside a material's `context { ... }`
// block — an engine-injected uniform. It mirrors Param's shape (name, type,
// optional default, array support) but carries different provenance: the host
// is expected to supply this value every frame, not the author. Default, when
// present, is a fallback used ONLY when the host omits the field (deterministic
// native/test rendering); it is never folded into a customUniforms default the
// way a Param's default is (the context-uniform design §3.1). Both lower to an
// ordinary uniform-block member — the difference is metadata, not codegen.
type ContextField struct {
	Name      string
	Type      Type
	Default   Expr
	Span      Span
	IsArray   bool // true: this is a fixed-size array context field
	ArraySize int  // element count when IsArray == true
}

// Func is a surface or vertex body: a binding name for the geometry record,
// local bindings, and (for surface) a result expression.
//
// Body is now []Stmt, supporting the full imperative statement set.
// FuncDecl.Body remains []Let (pure inlined functions are expression-only).
type Func struct {
	Geo    string // geometry record binding name (e.g. "geo")
	Span   Span
	Body   []Stmt
	Result Expr // surface: the color expression; vertex(): the clip-space position (vec4)
}

// Stmt is a high-level surface statement. Implementations: Let, VarDecl,
// Assign, If, For.
type Stmt interface{ isHIRStmt() }

// Let binds a local immutably to an expression: `let Name = Value`.
type Let struct {
	Name  string
	Value Expr
	Span  Span
}

// VarDecl declares a mutable local: `var Name = Value`.
type VarDecl struct {
	Name  string
	Value Expr
	Span  Span
}

// Assign reassigns a mutable local: `Name = Value`.
type Assign struct {
	Name  string
	Value Expr
	Span  Span
}

// If is an if/else statement.
type If struct {
	Cond Expr
	Then []Stmt
	Else []Stmt // nil if no else branch
	Span Span
}

// For is a bounded for loop: `for (var InitName = InitValue; Cond; PostName = PostValue) { Body }`.
type For struct {
	InitName  string
	InitValue Expr
	Cond      Expr
	PostName  string
	PostValue Expr
	Body      []Stmt
	Span      Span
}

// VarArrayDecl declares a mutable local fixed-size array:
//
//	var name : array<ElemType, Size>
//
// No initializer syntax: WGSL zero-initialises; authors fill elements explicitly
// (writes via IndexAssign / reads via IndexExpr). B2b.
type VarArrayDecl struct {
	Name     string
	ElemType Type
	Size     int
	Span     Span
}

// IndexExpr reads an element from a local array: Arr[Index].
// Arr is typically a Ref to a local array variable; Index must be int or uint.
type IndexExpr struct {
	Arr   Expr
	Index Expr
	Span  Span
}

// IndexAssign writes to an array element: Name[Index] = Value.
// Name must refer to a VarArrayDecl local; Index must be int or uint.
type IndexAssign struct {
	Name  string
	Index Expr
	Value Expr
	Span  Span
}

// Break exits the innermost enclosing for loop. Lowering rejects it outside a loop.
type Break struct {
	Span Span
}

// Discard throws away the current fragment (`discard`). It is a fragment-only
// statement (mesh/post kinds) and may appear inside an if-block. Lowering rejects
// it in the vertex stage and in kinds that do not support it.
type Discard struct {
	Span Span
}

// Return exits the surface/feedback body early with Value as the result,
// nested inside an if/for body (`return <expr>` that is not the last
// statement of the top-level function). The top-level function body keeps
// its single trailing return as Func.Result — Return only carries the early
// exits nested inside control flow. Lowering rejects it in authored vertex()
// bodies (varyings assigned after an early return would be undefined).
type Return struct {
	Value Expr
	Span  Span
}

func (Let) isHIRStmt()          {}
func (VarDecl) isHIRStmt()      {}
func (Assign) isHIRStmt()       {}
func (If) isHIRStmt()           {}
func (For) isHIRStmt()          {}
func (VarArrayDecl) isHIRStmt() {}
func (IndexAssign) isHIRStmt()  {}
func (Discard) isHIRStmt()      {}
func (Break) isHIRStmt()        {}
func (Return) isHIRStmt()       {}
func (IndexExpr) isExpr()       {}

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

// IntLit is a signed integer (i32) literal (written as 0i, 1i, etc.).
type IntLit struct {
	Value int64
	Span  Span
}

// UintLit is an unsigned integer (u32) literal (written as 0u, 1u, etc.).
type UintLit struct {
	Value uint64
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

// Unary is a prefix operator: -x (negation).
type Unary struct {
	Op   string
	E    Expr
	Span Span
}

// SuperCall is super.<Method>(args) — calls the parent material's method (only
// surface in v1). Resolved during lowering by inlining the parent's surface.
type SuperCall struct {
	Method string
	Args   []Expr
	Span   Span
}

// Conditional is a ternary expression: Cond ? Then : Alt. Cond must have
// bool type (produced by comparison or boolean operators); Then and Alt must
// have the same type; the result is that type.
type Conditional struct {
	Cond Expr
	Then Expr
	Alt  Expr
	Span Span
}

func (Ref) isExpr()         {}
func (Lit) isExpr()         {}
func (IntLit) isExpr()      {}
func (UintLit) isExpr()     {}
func (Member) isExpr()      {}
func (Call) isExpr()        {}
func (Binary) isExpr()      {}
func (Unary) isExpr()       {}
func (SuperCall) isExpr()   {}
func (Conditional) isExpr() {}

// ExprSpan returns source information for an expression when it came from
// parsed source.
func ExprSpan(e Expr) Span {
	switch x := e.(type) {
	case Ref:
		return x.Span
	case Lit:
		return x.Span
	case IntLit:
		return x.Span
	case UintLit:
		return x.Span
	case Member:
		return x.Span
	case Call:
		return x.Span
	case Binary:
		return x.Span
	case Unary:
		return x.Span
	case SuperCall:
		return x.Span
	case Conditional:
		return x.Span
	case IndexExpr:
		return x.Span
	default:
		return Span{}
	}
}
