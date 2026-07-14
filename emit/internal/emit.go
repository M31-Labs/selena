// Package internal holds helpers shared by the four backend shader emitters
// (emit/wgsl, emit/glsl, emit/metal, emit/gles). It is importable only by
// packages under emit/.
//
// The emitters differ in stage scaffolding and surface spelling, but the
// IR->gputype mapping, the binding-name set, and the ir.Dialect Ref/TypeName/
// Call/Sample plumbing are identical (or cleanly parameterizable) across them.
// This package is where that shared machinery lives so each emitter does not
// carry its own copy.
package internal

import (
	"fmt"
	"strconv"
	"strings"

	"m31labs.dev/prism/dialect"
	"m31labs.dev/prism/gputype"
	"m31labs.dev/selena/ir"
)

// TypeToGPU maps a Selena ir.Type to a prism gputype.Type.
func TypeToGPU(t ir.Type) gputype.Type {
	switch t {
	case ir.Bool:
		return gputype.Bool
	case ir.Float:
		return gputype.F32
	case ir.Int:
		return gputype.I32
	case ir.Uint:
		return gputype.U32
	case ir.Vec2:
		return gputype.Vec{N: 2, Elem: gputype.F32}
	case ir.Vec3:
		return gputype.Vec{N: 3, Elem: gputype.F32}
	case ir.Vec4:
		return gputype.Vec{N: 4, Elem: gputype.F32}
	case ir.Mat3:
		return gputype.Mat{Cols: 3, Rows: 3, Elem: gputype.F32}
	case ir.Mat4:
		return gputype.Mat{Cols: 4, Rows: 4, Elem: gputype.F32}
	default:
		return gputype.F32
	}
}

// NameSet collects the names of bs into a lookup set.
func NameSet(bs []ir.Binding) map[string]bool {
	m := make(map[string]bool, len(bs))
	for _, b := range bs {
		m[b.Name] = true
	}
	return m
}

// Resolver implements ir.Dialect for one emitter stage by delegating the
// surface spelling (TypeName/Call/Sample) to a prism dialect.Dialect and
// resolving references per the backend's reference model:
//
//   - Qualified backends (WGSL, Metal) address a uniform as u.x and a stage
//     input as in.x, falling back to a bare name for stage-locals.
//   - Bare backends (GLSL, GLES) address everything through bare globals, so
//     Ref returns the name unchanged.
//
// The name sets and Fragment flag drive the qualified case and are ignored when
// Qualified is false.
type Resolver struct {
	Dialect    dialect.Dialect
	Uniforms   map[string]bool
	Attributes map[string]bool
	Varyings   map[string]bool
	Fragment   bool
	Qualified  bool
	// SceneSampleFn renders a SceneSample expression. When nil, SceneSample
	// falls back to a dummy expression. Each post-emitter sets this.
	SceneSampleFn func(name, uv string) string
	// StateSampleFn renders a feedback-kind StateSample (previous-state read at
	// the constant cell offset dx, dy). Each feedback emitter sets this to the
	// backend read form. When nil, StateSample falls back to a dummy expression.
	StateSampleFn func(dx, dy int64) string
	// StateSampleUVFn renders a by-uv statefield read (stateAt(uv)) given the
	// already-rendered uv expression. Each mesh/general emitter that supports a
	// statefield sets this per stage. When nil, StateSampleUV falls back to a
	// dummy expression. B4.
	StateSampleUVFn func(uv string) string
	// CellUVFn renders the normalized [0,1]×[0,1] UV of the current feedback
	// cell. GLSL/GLES feedback emitters set it to "vUV"; WGSL/Metal compute
	// emitters derive it from cellIndex and the injected grid uniforms.
	// When nil, CellUV falls back to a dummy expression.
	CellUVFn func() string
}

// NewQualified builds a struct-qualified resolver (WGSL/Metal) for one stage.
// Both scalar uniforms (m.Uniforms) and array uniforms (m.ArrayUniforms) are
// included in the uniform name set so references like drops[i] resolve via
// the u. prefix in the emitted struct accessor.
func NewQualified(d dialect.Dialect, m ir.Module, fragment bool) Resolver {
	uniforms := NameSet(m.Uniforms)
	for _, au := range m.ArrayUniforms {
		uniforms[au.Name] = true
	}
	return Resolver{
		Dialect:    d,
		Uniforms:   uniforms,
		Attributes: NameSet(m.Attributes),
		Varyings:   NameSet(m.Varyings),
		Fragment:   fragment,
		Qualified:  true,
	}
}

// NewBare builds a bare-global resolver (GLSL/GLES).
func NewBare(d dialect.Dialect) Resolver {
	return Resolver{Dialect: d}
}

// TypeName spells an ir.Type for the backend.
func (r Resolver) TypeName(t ir.Type) string { return r.Dialect.TypeName(TypeToGPU(t)) }

// Call renders a builtin/function call.
func (r Resolver) Call(name string, args []string) string { return r.Dialect.Builtin(name, args) }

// glslNoScalarOverload is the set of component-wise builtins that lack a
// scalar-arg overload in GLSL/GLES. For these, scalar args must be expanded to
// match the vector width to avoid "no matching overloaded function" errors.
// Functions NOT in this set (max, min, clamp, mix, step, smoothstep, mod, …)
// have explicit genType/float overloads in GLSL 3.30 / GLES 3.00.
var glslNoScalarOverload = map[string]bool{
	"pow":     true,
	"reflect": true,
	"atan2":   true,
}

// CallTyped renders a builtin call with per-argument type information.
//
// WGSL requires all arguments to component-wise builtins (max, min, clamp,
// etc.) to have the same type. When the call has a vector+scalar mix, scalar
// float args are wrapped in vecN<f32>(scalar) to satisfy this requirement.
// Exceptions: mix(vecN, vecN, f32) and refract(vecN, vecN, f32) have native
// WGSL overloads where the last argument stays scalar.
//
// GLSL/GLES have explicit scalar-arg overloads for most component-wise
// builtins. The few that do not (pow, reflect, atan2) also receive a splat.
// Metal broadcasts scalar args implicitly and never needs a splat.
func (r Resolver) CallTyped(name string, args []string, argTypes []ir.Type) string {
	_, isWGSL := r.Dialect.(dialect.WGSL)
	_, isGLSL := r.Dialect.(dialect.GLSL)
	_, isGLES := r.Dialect.(dialect.GLES)

	needsSplat := isWGSL || ((isGLSL || isGLES) && glslNoScalarOverload[name])
	if !needsSplat {
		// Metal or GLSL/GLES with a native scalar overload — pass args unchanged.
		return r.Dialect.Builtin(name, args)
	}

	// Find the base vector type: the first vec2/vec3/vec4 among the arg types.
	baseVec := ir.Type("")
	for _, t := range argTypes {
		if t == ir.Vec2 || t == ir.Vec3 || t == ir.Vec4 {
			baseVec = t
			break
		}
	}
	if baseVec == "" {
		// All scalar — no splat needed regardless of backend.
		return r.Dialect.Builtin(name, args)
	}

	// mix(vecN, vecN, f32) and refract(vecN, vecN, f32) are valid WGSL overloads;
	// the last argument may remain scalar. GLSL pow/reflect/atan2 have no such
	// exception — all scalar args must match the vector width.
	exemptLastArg := isWGSL && (name == "mix" || name == "refract")

	vecTypeName := r.TypeName(baseVec)
	splatted := make([]string, len(args))
	for i, arg := range args {
		if argTypes[i] == ir.Float && !(exemptLastArg && i == len(args)-1) {
			splatted[i] = vecTypeName + "(" + arg + ")"
		} else {
			splatted[i] = arg
		}
	}
	return r.Dialect.Builtin(name, splatted)
}

// Sample renders a 2D texture sample.
func (r Resolver) Sample(tex, uv string) string { return r.Dialect.Sample(tex, uv) }

// SampleLevel renders a 2D texture sample at an explicit LOD (mip level),
// bypassing the dialect's implicit-LOD Sample. This is a selena-only
// extension layered on top of the prism dialect.Dialect interface (which has
// no SampleLevel method), spelled here directly per backend — the same
// pattern IntLit/UintLit/Discard already use to add backend-specific
// behaviour prism doesn't carry.
//
// WGSL:         textureSampleLevel(tex, texSampler, uv, lod) — unlike
//
//	textureSample (Sample), this form is valid inside non-uniform
//	control flow under naga's validator.
//
// GLSL ES 1.00: texture2DLod(tex, uv, lod) — per the GLSL ES 1.00 spec this
//
//	form is only defined in the vertex stage unless the
//	GL_EXT_shader_texture_lod extension is enabled; no current
//	selena GLSL emitter enables that extension, so fragment-stage
//	use requires the caller to arrange for it.
//
// GLSL ES 3.00 (GLES): textureLod(tex, uv, lod) — valid in any stage.
// Metal:        tex.sample(texSampler, uv, level(lod)).
func (r Resolver) SampleLevel(tex, uv, lod string) string {
	switch r.Dialect.(type) {
	case dialect.WGSL:
		return "textureSampleLevel(" + tex + ", " + tex + "Sampler, " + uv + ", " + lod + ")"
	case dialect.Metal:
		return tex + ".sample(" + tex + "Sampler, " + uv + ", level(" + lod + "))"
	case dialect.GLES:
		return "textureLod(" + tex + ", " + uv + ", " + lod + ")"
	default:
		// dialect.GLSL (GLSL ES 1.00).
		return "texture2DLod(" + tex + ", " + uv + ", " + lod + ")"
	}
}

// SampleCube renders a cube-map texture sample by a vec3 direction vector.
func (r Resolver) SampleCube(tex, dir string) string { return r.Dialect.SampleCube(tex, dir) }

// SceneSample renders a post-pass engine scene texture sample.
func (r Resolver) SceneSample(name, uv string) string {
	if r.SceneSampleFn != nil {
		return r.SceneSampleFn(name, uv)
	}
	return "vec4<f32>(0.0)" // unreachable in valid programs; guard only
}

// StateSample renders a feedback-kind previous-state read at the constant cell
// offset (dx, dy).
func (r Resolver) StateSample(dx, dy int64) string {
	if r.StateSampleFn != nil {
		return r.StateSampleFn(dx, dy)
	}
	return "vec4<f32>(0.0)" // unreachable in valid programs; guard only
}

// StateSampleUV renders a by-uv statefield read given the rendered uv expression.
func (r Resolver) StateSampleUV(uv string) string {
	if r.StateSampleUVFn != nil {
		return r.StateSampleUVFn(uv)
	}
	return "vec4<f32>(0.0)" // unreachable in valid programs; guard only
}

// CellUV renders the normalized [0,1]×[0,1] UV of the current feedback cell.
func (r Resolver) CellUV() string {
	if r.CellUVFn != nil {
		return r.CellUVFn()
	}
	return "vec2(0.0, 0.0)" // unreachable in valid programs; guard only
}

// Ternary renders a conditional expression, delegating to the backend dialect.
// WGSL: select(alt, then, cond); GLSL/GLES/Metal: (cond ? then : alt).
func (r Resolver) Ternary(cond, then, alt string) string {
	return r.Dialect.Ternary(cond, then, alt)
}

// Discard renders the fragment-discard statement spelling. Metal uses the
// function form discard_fragment(); every other backend (WGSL, GLSL, GLES) uses
// the discard keyword. The trailing semicolon is included.
func (r Resolver) Discard() string {
	if _, ok := r.Dialect.(dialect.Metal); ok {
		return "discard_fragment();"
	}
	return "discard;"
}

// IntLit renders a signed integer literal. WGSL uses the "i" suffix; all other
// backends use plain decimal digits.
func (r Resolver) IntLit(v int64) string {
	s := strconv.FormatInt(v, 10)
	if _, ok := r.Dialect.(dialect.WGSL); ok {
		return s + "i"
	}
	return s
}

// UintLit renders an unsigned integer literal. WGSL uses the "u" suffix.
func (r Resolver) UintLit(v uint64) string {
	s := strconv.FormatUint(v, 10)
	if _, ok := r.Dialect.(dialect.WGSL); ok {
		return s + "u"
	}
	return s
}

// Ref resolves a reference in the current stage scope.
func (r Resolver) Ref(name string) string {
	if !r.Qualified {
		return name
	}
	switch {
	case r.Uniforms[name]:
		return "u." + name
	case !r.Fragment && r.Attributes[name]:
		return "in." + name
	case r.Fragment && r.Varyings[name]:
		return "in." + name
	default:
		return name // stage-local
	}
}

// EmitStmtList writes all IR statements in stmts into b, using d for expression
// rendering. indent is prepended to each line. When wgsl is true the function
// uses WGSL `let`/`var` declaration syntax; otherwise it uses C-style `type name`
// syntax (GLSL, GLES, Metal).
//
// d.Fragment and d.Varyings jointly gate the one place vertex and fragment
// bodies differ in shape: when d.Fragment is false (an authored vertex() body,
// B4/B5) and a CF-less statement's Target names a declared varying, it is
// rendered as an output write (out.<name> = expr for qualified/WGSL+Metal
// backends, a bare <name> = expr for GLSL/GLES) instead of a let/var/typed
// declaration. Every existing call site renders a fragment body (d.Fragment is
// always true there), so this branch is unreachable for fragment/surface
// codegen and those call sites are unaffected byte-for-byte. Control flow (if/
// for) recurses with the same d, so a varying write nested inside a vertex-body
// if/for block is rendered identically.
func EmitStmtList(b *strings.Builder, stmts []ir.Stmt, d Resolver, indent string, wgsl bool) {
	for i := range stmts {
		emitStmt(b, &stmts[i], d, indent, wgsl)
	}
}

func emitStmt(b *strings.Builder, s *ir.Stmt, d Resolver, indent string, wgsl bool) {
	if s.CF == nil {
		if !d.Fragment && d.Varyings[s.Target] {
			// Authored vertex() varying write.
			if d.Qualified {
				fmt.Fprintf(b, "%sout.%s = %s;\n", indent, s.Target, ir.Print(s.Value, d))
			} else {
				fmt.Fprintf(b, "%s%s = %s;\n", indent, s.Target, ir.Print(s.Value, d))
			}
			return
		}
		// Simple let/var declaration.
		val := ir.Print(s.Value, d)
		if wgsl {
			if s.Mutable {
				fmt.Fprintf(b, "%svar %s : %s = %s;\n", indent, s.Target, d.TypeName(s.Type), val)
			} else {
				fmt.Fprintf(b, "%slet %s = %s;\n", indent, s.Target, val)
			}
		} else {
			fmt.Fprintf(b, "%s%s %s = %s;\n", indent, d.TypeName(s.Type), s.Target, val)
		}
		return
	}
	switch cf := s.CF.(type) {
	case ir.VarArrayCF:
		// Array declaration — backend-specific syntax.
		elemStr := d.TypeName(cf.ElemType)
		if wgsl {
			// WGSL: var name : array<elemType, N>;  (zero-initialised)
			fmt.Fprintf(b, "%svar %s : array<%s, %d>;\n", indent, cf.Target, elemStr, cf.Size)
		} else {
			// GLSL/GLES/Metal: elemType name[N];  (C-style, uninitialised)
			fmt.Fprintf(b, "%s%s %s[%d];\n", indent, elemStr, cf.Target, cf.Size)
		}
	case ir.IndexAssignCF:
		// Array element write — identical syntax across all four backends.
		fmt.Fprintf(b, "%s%s[%s] = %s;\n", indent, cf.Target, ir.Print(cf.Index, d), ir.Print(cf.Value, d))
	case ir.BreakCF:
		// Same spelling on WGSL, GLSL, GLES and Metal.
		fmt.Fprintf(b, "%sbreak;\n", indent)
	case ir.DiscardCF:
		// Fragment discard — spelling is backend-specific (Metal uses
		// discard_fragment(); others use discard;).
		fmt.Fprintf(b, "%s%s\n", indent, d.Discard())
	case ir.AssignCF:
		fmt.Fprintf(b, "%s%s = %s;\n", indent, cf.Target, ir.Print(cf.Value, d))
	case ir.IfCF:
		fmt.Fprintf(b, "%sif (%s) {\n", indent, ir.Print(cf.Cond, d))
		EmitStmtList(b, cf.Then, d, indent+"  ", wgsl)
		if len(cf.Else) > 0 {
			fmt.Fprintf(b, "%s} else {\n", indent)
			EmitStmtList(b, cf.Else, d, indent+"  ", wgsl)
		}
		fmt.Fprintf(b, "%s}\n", indent)
	case ir.ForCF:
		initVal := ir.Print(cf.InitValue, d)
		cond := ir.Print(cf.Cond, d)
		postVal := ir.Print(cf.PostValue, d)
		typStr := d.TypeName(cf.InitType)
		var initDecl string
		if wgsl {
			initDecl = "var " + cf.InitTarget + " : " + typStr + " = " + initVal
		} else {
			initDecl = typStr + " " + cf.InitTarget + " = " + initVal
		}
		fmt.Fprintf(b, "%sfor (%s; %s; %s = %s) {\n",
			indent, initDecl, cond, cf.PostTarget, postVal)
		EmitStmtList(b, cf.Body, d, indent+"  ", wgsl)
		fmt.Fprintf(b, "%s}\n", indent)
	}
}
