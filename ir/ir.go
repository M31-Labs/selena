package ir

import (
	"strconv"
)

// Type is a shader value type. The set is intentionally small for the first
// slice; it grows as the language does.
type Type string

const (
	Bool  Type = "bool"
	Float Type = "float"
	Int   Type = "int"
	Uint  Type = "uint"
	Vec2  Type = "vec2"
	Vec3  Type = "vec3"
	Vec4  Type = "vec4"
	Mat3  Type = "mat3"
	Mat4  Type = "mat4"
)

// Kind identifies which surface pipeline a Module targets.
type Kind string

const (
	// KindMesh is the default: vertex+fragment stages for triangle meshes.
	KindMesh Kind = "mesh"
	// KindPoints is a billboard quad point/particle appearance surface.
	// The emitter bakes billboard quad expansion and the engine point
	// uniforms; the author-body runs inside the fragment stage.
	KindPoints Kind = "points"
	// KindPost is a fullscreen post-process pass. The emitter generates a
	// fullscreen triangle vertex stage; the author-body is the fragment.
	KindPost Kind = "post"
	// KindFeedback is a per-cell simulation step over a vec4-per-cell statefield.
	// The author body is a pure GATHER (outState[self] from inState[neighbours]),
	// so it lowers to BOTH a WGSL @compute kernel (writing outState) and a
	// GLSL/GLES fullscreen fragment ping-pong pass (writing the stage output).
	KindFeedback Kind = "feedback"
)

// Module is a complete material shader: a declared interface (uniforms,
// per-vertex attributes, vertex->fragment varyings, sampled textures) plus the
// two stages.
type Module struct {
	Name          string
	Kind          Kind           // KindMesh / KindPoints / KindPost; zero == KindMesh
	Uniforms      []Binding      // shared constants (mvp, color, light, ...)
	ArrayUniforms []ArrayBinding // fixed-size uniform arrays (B3.2)
	Attributes    []Binding      // per-vertex inputs (position, normal, uv, ...)
	Varyings      []Binding      // interpolated vertex -> fragment values
	Textures      []Texture      // sampled textures (fragment stage)
	Vertex        Stage
	Fragment      Stage
	// StateField is the name of the vec4-per-cell statefield. For KindFeedback the
	// feedback body is carried in Fragment and state(dx, dy) reads lower to
	// StateSample. For mesh/general modules a statefield enables stateAt(uv) reads
	// (StateSampleUV) in the vertex and fragment stages; empty otherwise. B4.
	StateField string
	// VertexAuthored is true when the material authored its own vertex() stage
	// (B4). The mesh emitters then emit the author-controlled vertex entry (clip
	// position from the body, author varyings, optional vertexIndex builtin)
	// instead of the default mvp*position transform. False for every legacy mesh
	// material, keeping their emitted source byte-identical.
	VertexAuthored bool
	// UsesVertexIndex is true when the authored vertex stage references the
	// vertexIndex builtin (procedural geometry). Each backend then wires its
	// vertex-index source: WGSL @builtin(vertex_index), GLES gl_VertexID, Metal
	// [[vertex_id]], GLSL (WebGL1) a host-supplied a_vertexIndex attribute. B4.
	UsesVertexIndex bool
}

// Texture is a sampled texture used by the fragment stage. Backends bind it
// differently (WGSL/Metal split texture + sampler; GLSL/GLES use a combined
// sampler2D/samplerCube) — the emitters and the binding descriptor encode that.
// When Cube is true the texture is a cube map: WGSL emits texture_cube<f32>,
// GLSL/GLES emit samplerCube, Metal emits texturecube<float>, and
// sampleCube(tex, dir) lowers to ir.SampleCube rather than ir.Sample.
type Texture struct {
	Name string
	Cube bool // true = cube-map texture; false (default) = 2D texture
}

// Binding is a named, typed interface slot.
type Binding struct {
	Name string
	Type Type
}

// ArrayBinding is a named, fixed-size uniform array: a field inside the
// shader's uniform block that holds Count elements of ElemType. In WGSL it
// emits as `name : array<elemType, Count>,`; in GLSL/GLES as
// `uniform elemType name[Count];`; in Metal as `elemType name[Count];`.
// std140 rule: every array element is padded to 16 bytes (stride == 16). B3.2.
type ArrayBinding struct {
	Name     string
	ElemType Type
	Count    int
}

// Stage is one shader stage: a sequence of statements followed by the stage's
// output expression. The vertex Output is clip-space position (vec4); the
// fragment Output is the fragment color (vec4).
type Stage struct {
	Body   []Stmt
	Output Expr
}

// Stmt is one IR statement. For simple let/var assignments (CF == nil) it
// carries Target/Type/Value/Mutable. When CF is non-nil this is a control-flow
// node (AssignCF, IfCF, or ForCF) and the other fields are ignored.
//
// Using a struct union avoids changing the Stage.Body slice type and preserves
// backwards compatibility for hand-built IR samples that only use simple assigns.
type Stmt struct {
	// For let/var declarations (CF == nil):
	Target  string
	Type    Type
	Value   Expr
	Mutable bool // true → var (mutable); false → let (immutable)

	// For control-flow statements (CF != nil):
	CF StmtCF
}

// StmtCF is the control-flow payload of an ir.Stmt.
type StmtCF interface{ isStmtCF() }

// AssignCF reassigns an existing mutable variable: Target = Value.
type AssignCF struct {
	Target string
	Value  Expr
}

// IfCF is an if/else statement.
type IfCF struct {
	Cond Expr
	Then []Stmt
	Else []Stmt // nil if no else branch
}

// ForCF is a bounded for loop:
//
//	for (var InitTarget : InitType = InitValue; Cond; PostTarget = PostValue) { Body }
type ForCF struct {
	InitTarget string
	InitType   Type
	InitValue  Expr
	Cond       Expr
	PostTarget string
	PostValue  Expr
	Body       []Stmt
}

// VarArrayCF declares a mutable local fixed-size array.
//
//	WGSL:        var Target : array<ElemTypeName, Size>;
//	GLSL/GLES:   ElemTypeName Target[Size];
//	Metal:       ElemTypeName Target[Size];
//
// No initializer: WGSL zero-initialises; GLSL/Metal leave it uninitialised
// (authors must write all elements before reading). B2b.
type VarArrayCF struct {
	Target   string
	ElemType Type
	Size     int
}

// IndexAssignCF writes to an array element: Target[Index] = Value.
type IndexAssignCF struct {
	Target string
	Index  Expr
	Value  Expr
}

// DiscardCF discards the current fragment. It carries no payload; each backend
// renders it through Dialect.Discard (WGSL/GLSL/GLES `discard;`, Metal
// `discard_fragment();`).
type DiscardCF struct{}

// Index reads an array element: Arr[Idx].
// Emits identically across all four backends as Arr[Idx].
type Index struct {
	Arr Expr
	Idx Expr
}

func (AssignCF) isStmtCF()      {}
func (IfCF) isStmtCF()          {}
func (ForCF) isStmtCF()         {}
func (VarArrayCF) isStmtCF()    {}
func (IndexAssignCF) isStmtCF() {}
func (DiscardCF) isStmtCF()     {}
func (Index) isExpr()           {}

// Expr is the typed expression graph. It is total by construction (no loops,
// no recursion) so every backend can emit it deterministically.
type Expr interface{ isExpr() }

// Ref references a uniform, attribute, varying, or stage-local by name.
type Ref struct{ Name string }

// Lit is a scalar float literal.
type Lit struct{ Value float64 }

// IntLit is a signed integer (i32) literal.
type IntLit struct{ Value int64 }

// UintLit is an unsigned integer (u32) literal.
type UintLit struct{ Value uint64 }

// Construct builds a vector/matrix value, e.g. vec4(rgb, 1.0).
type Construct struct {
	Type Type
	Args []Expr
}

// Call is a builtin function call. Dialects render the backend spelling through
// Dialect.Call so future builtins do not need to change the generic IR printer.
//
// ArgTypes carries the IR type of each argument when the lowerer can determine
// them at compile time. When non-nil and non-empty, Print dispatches to
// Dialect.CallTyped instead of Dialect.Call, allowing the WGSL emitter to
// auto-splat scalar arguments to the base vector type for functions like max,
// min, clamp, etc. that require all arguments to have the same type in WGSL.
// GLSL/GLES/Metal ignore ArgTypes and receive the same scalar literal.
type Call struct {
	Func     string
	Args     []Expr
	ArgTypes []Type // optional; nil/empty means use CallTyped fallback to Call
}

// Binary is an infix operator shared across backends: + - * / (including
// matrix*vector and vector*scalar).
type Binary struct {
	Op   string
	L, R Expr
}

// Unary is a prefix operator: -x. Spelled identically in every backend.
type Unary struct {
	Op string
	E  Expr
}

// Swizzle selects components, e.g. position.xyz or color.rgb.
type Swizzle struct {
	E     Expr
	Field string
}

// Sample samples Texture at UV. Each backend renders it differently (see
// Dialect.Sample); the named texture must be declared in Module.Textures.
type Sample struct {
	Texture string
	UV      Expr
}

// SampleCube samples a cube-map Texture by a vec3 direction vector. Each
// backend renders it differently (see Dialect.SampleCube); the named texture
// must be declared in Module.Textures with Cube == true.
// WGSL:         textureSample(tex, texSampler, dir)
// GLSL ES 1.00: textureCube(tex, dir)
// GLSL ES 3.00: texture(tex, dir)
// Metal:        tex.sample(texSampler, dir)
type SampleCube struct {
	Texture string
	Dir     Expr
}

// SceneSample samples one of the engine-provided post-pass textures at UV.
// Name is "sceneColor" or "sceneDepth". These are NOT in Module.Textures —
// they are at fixed engine binding slots the post emitter knows. The return
// type of sceneColor is vec4; sceneDepth is float (depth buffer read).
type SceneSample struct {
	Name string // "sceneColor" or "sceneDepth"
	UV   Expr
}

// Conditional is a ternary expression: Cond ? Then : Alt. Cond is bool;
// Then and Alt have the same type; result is that type.
// Backends render this differently: WGSL emits select(Alt, Then, Cond)
// while GLSL, GLES, and Metal emit (Cond ? Then : Alt).
type Conditional struct {
	Cond Expr
	Then Expr
	Alt  Expr
}

// StateSample reads the previous statefield value at the current cell offset by
// the compile-time-constant integers (DX, DY); result type is vec4. DX/DY are
// resolved to constants at lowering so each backend can address the state grid
// directly: WGSL/Metal index the inState buffer (cellIndex + DX + DY*gridWidth,
// clamped); GLSL/GLES sample stateTex at (vUV + vec2(DX, DY)*texelSize).
type StateSample struct {
	DX int64
	DY int64
}

// StateSampleUV reads the statefield at an arbitrary uv (vec2 in [0,1]); result
// type is vec4. Unlike StateSample (the feedback kind's compile-time-constant
// neighbour gather) this addresses the grid by a runtime uv, so it is usable in
// both the vertex stage (vertex texture fetch / displacement) and the fragment
// stage of mesh/general materials. Each backend renders it through
// Dialect.StateSampleUV: WGSL/Metal index the inState storage buffer (uv ->
// linear cell index via the state-grid dimensions); GLSL/GLES sample stateTex at
// uv (textureLod in the vertex stage). B4.
type StateSampleUV struct {
	UV Expr
}

// CellUV is the normalized [0,1]×[0,1] UV of the current feedback cell.
// Each backend renders it differently: GLSL/GLES emit the fragment varying
// `vUV` (already in scope from the fullscreen-quad vertex pass); WGSL/Metal
// derive it from the compute cellIndex and the injected grid dimensions.
// Result type is vec2.
type CellUV struct{}

func (Ref) isExpr()           {}
func (Lit) isExpr()           {}
func (IntLit) isExpr()        {}
func (UintLit) isExpr()       {}
func (Construct) isExpr()     {}
func (Call) isExpr()          {}
func (Binary) isExpr()        {}
func (Unary) isExpr()         {}
func (Swizzle) isExpr()       {}
func (Sample) isExpr()        {}
func (SampleCube) isExpr()    {}
func (SceneSample) isExpr()   {}
func (Conditional) isExpr()   {}
func (StateSample) isExpr()   {}
func (StateSampleUV) isExpr() {}
func (CellUV) isExpr()        {}

// Dialect spells the backend-specific parts of expression printing: how a type
// name renders (vec4 vs vec4<f32>) and how a reference renders in the current
// stage scope (a uniform as u.x in WGSL vs a bare global in GLSL). Operators,
// builtins, literals, and the overall expression structure are shared — see
// Print. The two backends differing only in Dialect (and stage scaffolding) is
// the property this slice is meant to validate.
type Dialect interface {
	TypeName(Type) string
	Ref(name string) string
	Call(name string, args []string) string
	// CallTyped renders a builtin call when per-argument type information is
	// available. WGSL requires all arguments to component-wise builtins (max,
	// min, clamp, etc.) to have the same type; CallTyped wraps scalar float
	// args in vecN<f32>(scalar) when the call has a vector+scalar mix.
	// GLSL/GLES/Metal broadcast scalar args natively so they fall back to Call.
	CallTyped(name string, args []string, argTypes []Type) string
	// Sample renders a 2D texture sample given the texture name and the
	// already-rendered UV expression (WGSL: textureSample(t, tSampler, uv);
	// GLSL ES1: texture2D(t, uv); GLSL ES3: texture(t, uv); Metal: t.sample(tSampler, uv)).
	Sample(tex, uv string) string
	// SampleCube renders a cube-map texture sample given the texture name and the
	// already-rendered direction expression (vec3).
	// WGSL: textureSample(t, tSampler, dir); GLSL ES1: textureCube(t, dir);
	// GLSL ES3/GLES: texture(t, dir); Metal: t.sample(tSampler, dir).
	SampleCube(tex, dir string) string
	// SceneSample renders an engine-provided post-pass scene texture sample.
	// name is "sceneColor" or "sceneDepth"; each backend uses its own fixed
	// binding names for these engine-provided textures.
	SceneSample(name, uv string) string
	// StateSample renders a feedback-kind previous-state read at the constant
	// cell offset (dx, dy). WGSL/Metal index the inState buffer; GLSL/GLES
	// sample stateTex at vUV + vec2(dx, dy)*texelSize.
	StateSample(dx, dy int64) string
	// StateSampleUV renders a by-uv statefield read given the already-rendered uv
	// expression. WGSL/Metal index the inState storage buffer (uv -> cell index);
	// GLSL/GLES sample stateTex at uv. B4.
	StateSampleUV(uv string) string
	// Ternary renders a conditional expression. WGSL emits select(alt, then, cond);
	// GLSL, GLES, and Metal emit (cond ? then : alt).
	Ternary(cond, then, alt string) string
	// IntLit renders a signed integer literal. WGSL uses "0i" suffix;
	// GLSL, GLES, and Metal use plain decimal digits.
	IntLit(v int64) string
	// UintLit renders an unsigned integer literal. WGSL uses "0u" suffix;
	// GLSL, GLES, and Metal use plain decimal digits.
	UintLit(v uint64) string
	// CellUV renders the normalized [0,1]×[0,1] UV of the current feedback cell.
	// GLSL/GLES: the fragment varying vUV; WGSL/Metal: derived from cellIndex and
	// the injected grid dimensions.
	CellUV() string
	// Discard renders the fragment-discard statement spelling, including its
	// trailing semicolon: WGSL/GLSL/GLES `discard;`, Metal `discard_fragment();`.
	Discard() string
}

// Print renders e using d for the backend-specific spellings.
func Print(e Expr, d Dialect) string {
	switch x := e.(type) {
	case Ref:
		return d.Ref(x.Name)
	case Lit:
		return formatFloat(x.Value)
	case IntLit:
		return d.IntLit(x.Value)
	case UintLit:
		return d.UintLit(x.Value)
	case Construct:
		return d.TypeName(x.Type) + "(" + joinArgs(printArgs(x.Args, d)) + ")"
	case Call:
		if len(x.ArgTypes) > 0 {
			return d.CallTyped(x.Func, printArgs(x.Args, d), x.ArgTypes)
		}
		return d.Call(x.Func, printArgs(x.Args, d))
	case Binary:
		return "(" + Print(x.L, d) + " " + x.Op + " " + Print(x.R, d) + ")"
	case Unary:
		return "(" + x.Op + Print(x.E, d) + ")"
	case Swizzle:
		return Print(x.E, d) + "." + x.Field
	case Sample:
		return d.Sample(x.Texture, Print(x.UV, d))
	case SampleCube:
		return d.SampleCube(x.Texture, Print(x.Dir, d))
	case SceneSample:
		return d.SceneSample(x.Name, Print(x.UV, d))
	case Conditional:
		return d.Ternary(Print(x.Cond, d), Print(x.Then, d), Print(x.Alt, d))
	case StateSample:
		return d.StateSample(x.DX, x.DY)
	case StateSampleUV:
		return d.StateSampleUV(Print(x.UV, d))
	case CellUV:
		return d.CellUV()
	case Index:
		return Print(x.Arr, d) + "[" + Print(x.Idx, d) + "]"
	default:
		return "/* unknown expr */"
	}
}

func printArgs(args []Expr, d Dialect) []string {
	parts := make([]string, len(args))
	for i, a := range args {
		parts[i] = Print(a, d)
	}
	return parts
}

func joinArgs(parts []string) string {
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
