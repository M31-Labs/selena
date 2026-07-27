package lower

import (
	"fmt"
	"strings"

	"m31labs.dev/selena/hir"
	"m31labs.dev/selena/ir"
)

// arrayInfo holds element type and size for a local fixed-size array variable.
type arrayInfo struct {
	elemType ir.Type
	size     int
}

// typer infers the IR type of a HIR expression within one material surface.
type typer struct {
	paramKind     map[string]hir.Type
	geo           string
	locals        map[string]ir.Type
	mutableLocals map[string]bool // tracks which locals were declared with var
	// geoFields is the geometry registry for this kind. Nil means mesh stdlib.
	geoFields map[string]geometrySpec
	// allowSceneSample enables sceneColor/sceneDepth (post kind only).
	allowSceneSample bool
	// allowState enables state(dx, dy) (feedback kind only).
	allowState bool
	// allowVertexIndex enables the vertexIndex builtin (authored vertex stage only). B4.
	allowVertexIndex bool
	// allowStateAt enables stateAt(uv) reads (mesh/general materials that declare
	// a statefield; valid in both the authored vertex stage and the surface). B4.
	allowStateAt bool
	// arrayLocals tracks local fixed-size array variables (B2b).
	arrayLocals map[string]arrayInfo
	// paramArrays tracks uniform array params (B3.2): `param name : array<T, N>`.
	// A bare Ref to a param array is a type error; indexing it returns ElemType.
	paramArrays map[string]arrayInfo
}

func (t *typer) typeOf(e hir.Expr) (ir.Type, error) {
	switch x := e.(type) {
	case hir.Lit:
		return ir.Float, nil
	case hir.IntLit:
		_ = x
		return ir.Int, nil
	case hir.UintLit:
		_ = x
		return ir.Uint, nil
	case hir.Ref:
		if lt, ok := t.locals[x.Name]; ok {
			return lt, nil
		}
		if x.Name == "vertexIndex" {
			if !t.allowVertexIndex {
				return "", diagnostic(CodeUnknownName, x.Span, "vertexIndex is only available in a vertex() stage")
			}
			return ir.Uint, nil
		}
		if pk, ok := t.paramKind[x.Name]; ok {
			if it, ok := hirToIRType(pk); ok {
				return it, nil
			}
			return "", diagnostic(CodeInvalidMember, x.Span, "%q has record type %q and must be accessed by field", x.Name, pk)
		}
		if t.paramArrays != nil {
			if _, ok := t.paramArrays[x.Name]; ok {
				return "", diagnostic(CodeInvalidMember, x.Span, "array param %q must be indexed with [i], not used directly", x.Name)
			}
		}
		return "", diagnostic(CodeUnknownName, x.Span, "unknown name %q", x.Name)
	case hir.Member:
		if base, ok := x.E.(hir.Ref); ok {
			if base.Name == t.geo {
				if t.geoFields != nil {
					if gf, ok := t.geoFields[x.Field]; ok {
						return gf.typ, nil
					}
					return "", diagnostic(CodeInvalidMember, x.Span, "unknown geometry field geo.%s", x.Field)
				}
				if gf, ok := stdlib.geometryField(x.Field); ok {
					return gf.typ, nil
				}
				return "", diagnostic(CodeInvalidMember, x.Span, "unknown geometry field geo.%s", x.Field)
			}
			if t.paramKind[base.Name] == hir.Sun {
				if ft, ok := stdlib.recordField(string(hir.Sun), x.Field); ok {
					return ft, nil
				}
				return "", diagnostic(CodeInvalidMember, x.Span, "Sun has no field %q", x.Field)
			}
		}
		bt, err := t.typeOf(x.E)
		if err != nil {
			return "", err
		}
		st, err := swizzleType(bt, x.Field)
		if err != nil {
			return "", diagnostic(CodeInvalidSwizzle, x.Span, "%v", err)
		}
		return st, nil
	case hir.Call:
		return t.callType(x)
	case hir.Binary:
		return t.binaryType(x)
	case hir.Unary:
		if x.Op == "!" {
			inner, err := t.typeOf(x.E)
			if err != nil {
				return "", err
			}
			if inner != ir.Bool {
				return "", diagnostic(CodeTypeMismatch, x.Span, "operator ! requires bool operand, got %s", inner)
			}
			return ir.Bool, nil
		}
		return t.typeOf(x.E)
	case hir.Conditional:
		condT, err := t.typeOf(x.Cond)
		if err != nil {
			return "", err
		}
		if condT != ir.Bool {
			return "", diagnostic(CodeTypeMismatch, x.Span, "ternary condition must be bool, got %s", condT)
		}
		thenT, err := t.typeOf(x.Then)
		if err != nil {
			return "", err
		}
		altT, err := t.typeOf(x.Alt)
		if err != nil {
			return "", err
		}
		if thenT != altT {
			return "", diagnostic(CodeTypeMismatch, x.Span, "ternary branches must have the same type, got %s and %s", thenT, altT)
		}
		return thenT, nil
	case hir.IndexExpr:
		// Base must be a Ref naming a local array variable or a uniform array param.
		arrRef, ok := x.Arr.(hir.Ref)
		if !ok {
			return "", fmt.Errorf("array index expression: base must be a local array name or uniform array param")
		}
		// Check local arrays first (B2b).
		if t.arrayLocals != nil {
			if info, ok := t.arrayLocals[arrRef.Name]; ok {
				idxT, err := t.typeOf(x.Index)
				if err != nil {
					return "", err
				}
				if idxT != ir.Int && idxT != ir.Uint {
					return "", diagnostic(CodeTypeMismatch, x.Span, "%s", arrayIndexTypeMessage(idxT, x.Index))
				}
				return info.elemType, nil
			}
		}
		// Check uniform array params (B3.2).
		if t.paramArrays != nil {
			if info, ok := t.paramArrays[arrRef.Name]; ok {
				idxT, err := t.typeOf(x.Index)
				if err != nil {
					return "", err
				}
				if idxT != ir.Int && idxT != ir.Uint {
					return "", diagnostic(CodeTypeMismatch, x.Span, "%s", arrayIndexTypeMessage(idxT, x.Index))
				}
				return info.elemType, nil
			}
		}
		return "", diagnostic(CodeUnknownName, x.Span, "%q is not a local array or uniform array param", arrRef.Name)
	}
	return "", fmt.Errorf("unsupported expression %T", e)
}

// arrayIndexTypeMessage explains a rejected array index.
//
// A bare `0` is a float literal in Selena, so `for (var i = 0; ...)` gives a
// float counter and `rects[i]` is ill-typed. The bare "array index must be int
// or uint, got float" left the reader with no way to know that the `i` literal
// suffix exists, so the fix is named whenever the index is a plain variable.
func arrayIndexTypeMessage(idxT ir.Type, index hir.Expr) string {
	base := "array index must be int or uint, got " + string(idxT)
	if idxT != ir.Float {
		return base
	}
	if ref, ok := index.(hir.Ref); ok {
		return base + "; declare the counter with an int literal — `var " + ref.Name +
			" = 0i` — or convert at the index with `int(" + ref.Name + ")`"
	}
	return base + "; int literals carry the `i` suffix (0i, 8i), or convert with int(...)"
}

func (t *typer) callType(c hir.Call) (ir.Type, error) {
	// Handle scene samplers before the general stdlib lookup.
	if c.Func == "sceneColor" || c.Func == "sceneDepth" {
		if !t.allowSceneSample {
			return "", diagnostic(CodeInvalidCall, c.Span, "%s is only available in post-kind materials", c.Func)
		}
		if len(c.Args) != 1 {
			return "", diagnostic(CodeInvalidCall, c.Span, "%s(uv) takes 1 argument", c.Func)
		}
		uvt, err := t.typeOf(c.Args[0])
		if err != nil {
			return "", err
		}
		if uvt != ir.Vec2 {
			return "", diagnostic(CodeTypeMismatch, c.Span, "%s: argument must be vec2 uv, got %s", c.Func, uvt)
		}
		if c.Func == "sceneDepth" {
			return ir.Float, nil
		}
		return ir.Vec4, nil
	}

	// Backdrop LOD tap: sceneColorLevel(uv, lod) -> vec4.
	if c.Func == "sceneColorLevel" {
		if !t.allowSceneSample {
			return "", diagnostic(CodeInvalidCall, c.Span, "%s is only available in post-kind materials", c.Func)
		}
		if len(c.Args) != 2 {
			return "", diagnostic(CodeInvalidCall, c.Span, "sceneColorLevel(uv, lod) takes 2 arguments")
		}
		uvt, err := t.typeOf(c.Args[0])
		if err != nil {
			return "", err
		}
		if uvt != ir.Vec2 {
			return "", diagnostic(CodeTypeMismatch, c.Span, "sceneColorLevel: first argument must be vec2 uv, got %s", uvt)
		}
		lodt, err := t.typeOf(c.Args[1])
		if err != nil {
			return "", err
		}
		if lodt != ir.Float {
			return "", diagnostic(CodeTypeMismatch, c.Span, "sceneColorLevel: second argument must be a float lod, got %s", lodt)
		}
		return ir.Vec4, nil
	}

	// Backdrop resolution: sceneSize() -> vec2.
	if c.Func == "sceneSize" {
		if !t.allowSceneSample {
			return "", diagnostic(CodeInvalidCall, c.Span, "%s is only available in post-kind materials", c.Func)
		}
		if len(c.Args) != 0 {
			return "", diagnostic(CodeInvalidCall, c.Span, "sceneSize() takes no arguments")
		}
		return ir.Vec2, nil
	}

	// Feedback-kind previous-state read: state(dx, dy) -> vec4.
	if c.Func == "state" {
		if !t.allowState {
			return "", diagnostic(CodeInvalidCall, c.Span, "state(dx, dy) is only available in feedback-kind materials")
		}
		if len(c.Args) != 2 {
			return "", diagnostic(CodeInvalidCall, c.Span, "state(dx, dy) takes 2 arguments")
		}
		return ir.Vec4, nil
	}

	// By-uv statefield read: stateAt(uv) -> vec4 (B4).
	if c.Func == "stateAt" {
		if !t.allowStateAt {
			return "", diagnostic(CodeInvalidCall, c.Span, "stateAt(uv) requires a statefield; declare `state <name>` in the material")
		}
		if len(c.Args) != 1 {
			return "", diagnostic(CodeInvalidCall, c.Span, "stateAt(uv) takes 1 argument")
		}
		uvt, err := t.typeOf(c.Args[0])
		if err != nil {
			return "", err
		}
		if uvt != ir.Vec2 {
			return "", diagnostic(CodeTypeMismatch, c.Span, "stateAt: argument must be vec2 uv, got %s", uvt)
		}
		return ir.Vec4, nil
	}

	spec, ok := stdlib.builtin(c.Func)
	if !ok {
		// An unregistered callee is an unknown name, not a typed call. Selena
		// inlines every author `fn` before lowering, so anything still spelled as
		// a call here names nothing the language provides. Reporting it as
		// SEL2001 at the call site is the whole diagnosis; the earlier behaviour
		// (infer the call's type from its first argument, or complain that a
		// zero-argument call "has no arguments") reported a downstream type
		// mismatch — for example `sceneColorLevel(uv, lod)` surfaced as "surface
		// must return color/vec3/vec4, got vec2" — and let an unknown name pass
		// straight through into every emitted backend source. unknownFunctionError
		// also names the fix for common mistakes, such as color -> rgb.
		return "", unknownFunctionError(c.Func, c.Span)
	}

	switch spec.kind {
	case builtinDot:
		if len(c.Args) != spec.arity {
			return "", diagnostic(CodeInvalidCall, c.Span, "%s expects %d arguments, got %d", c.Func, spec.arity, len(c.Args))
		}
		a, err := t.typeOf(c.Args[0])
		if err != nil {
			return "", err
		}
		b, err := t.typeOf(c.Args[1])
		if err != nil {
			return "", err
		}
		if !isVector(a) || a != b {
			return "", diagnostic(CodeTypeMismatch, c.Span, "dot arguments must be matching vectors, got %s and %s", a, b)
		}
		return ir.Float, nil
	case builtinLength:
		if len(c.Args) != spec.arity {
			return "", diagnostic(CodeInvalidCall, c.Span, "%s expects %d arguments, got %d", c.Func, spec.arity, len(c.Args))
		}
		a, err := t.typeOf(c.Args[0])
		if err != nil {
			return "", err
		}
		if !isVector(a) {
			return "", diagnostic(CodeTypeMismatch, c.Span, "length argument must be a vector, got %s", a)
		}
		return ir.Float, nil
	case builtinDistance:
		if len(c.Args) != spec.arity {
			return "", diagnostic(CodeInvalidCall, c.Span, "%s expects %d arguments, got %d", c.Func, spec.arity, len(c.Args))
		}
		a, err := t.typeOf(c.Args[0])
		if err != nil {
			return "", err
		}
		b, err := t.typeOf(c.Args[1])
		if err != nil {
			return "", err
		}
		if !isVector(a) || a != b {
			return "", diagnostic(CodeTypeMismatch, c.Span, "distance arguments must be matching vectors, got %s and %s", a, b)
		}
		return ir.Float, nil
	case builtinSample:
		if len(c.Args) != spec.arity {
			return "", diagnostic(CodeInvalidCall, c.Span, "sample(texture, uv) takes 2 arguments")
		}
		tex, ok := c.Args[0].(hir.Ref)
		if !ok || t.paramKind[tex.Name] != hir.Texture2D {
			return "", diagnostic(CodeInvalidCall, c.Span, "sample: first argument must be a texture2d param")
		}
		uv, err := t.typeOf(c.Args[1])
		if err != nil {
			return "", err
		}
		if uv != ir.Vec2 {
			return "", diagnostic(CodeTypeMismatch, c.Span, "sample: second argument must be vec2 uv, got %s", uv)
		}
		return ir.Vec4, nil
	case builtinSampleLevel:
		if len(c.Args) != spec.arity {
			return "", diagnostic(CodeInvalidCall, c.Span, "sampleLevel(texture, uv, lod) takes 3 arguments")
		}
		tex, ok := c.Args[0].(hir.Ref)
		if !ok || t.paramKind[tex.Name] != hir.Texture2D {
			if ok && (tex.Name == "sceneColor" || tex.Name == "sceneDepth") {
				// The engine backdrop is not a param, so sampleLevel can never
				// name it. Point at the builtin that does.
				return "", diagnostic(CodeInvalidCall, c.Span,
					"sampleLevel: %s is an engine backdrop, not a texture2d param; use sceneColorLevel(uv, lod)", tex.Name)
			}
			return "", diagnostic(CodeInvalidCall, c.Span, "sampleLevel: first argument must be a texture2d param")
		}
		uv, err := t.typeOf(c.Args[1])
		if err != nil {
			return "", err
		}
		if uv != ir.Vec2 {
			return "", diagnostic(CodeTypeMismatch, c.Span, "sampleLevel: second argument must be vec2 uv, got %s", uv)
		}
		lod, err := t.typeOf(c.Args[2])
		if err != nil {
			return "", err
		}
		if lod != ir.Float {
			return "", diagnostic(CodeTypeMismatch, c.Span, "sampleLevel: third argument must be a float lod, got %s", lod)
		}
		return ir.Vec4, nil
	case builtinSampleCube:
		if len(c.Args) != spec.arity {
			return "", diagnostic(CodeInvalidCall, c.Span, "sampleCube(texture, dir) takes 2 arguments")
		}
		tex, ok := c.Args[0].(hir.Ref)
		if !ok || t.paramKind[tex.Name] != hir.TextureCube {
			return "", diagnostic(CodeInvalidCall, c.Span, "sampleCube: first argument must be a textureCube param")
		}
		dir, err := t.typeOf(c.Args[1])
		if err != nil {
			return "", err
		}
		if dir != ir.Vec3 {
			return "", diagnostic(CodeTypeMismatch, c.Span, "sampleCube: second argument must be vec3 direction, got %s", dir)
		}
		return ir.Vec4, nil
	case builtinRGB:
		if len(c.Args) != 3 && len(c.Args) != 4 {
			return "", diagnostic(CodeInvalidCall, c.Span, "rgb expects 3 or 4 arguments, got %d", len(c.Args))
		}
		for i, a := range c.Args {
			at, err := t.typeOf(a)
			if err != nil {
				return "", err
			}
			if at != ir.Float {
				return "", diagnostic(CodeTypeMismatch, c.Span, "rgb argument %d must be float, got %s", i+1, at)
			}
		}
		if len(c.Args) == 4 {
			return ir.Vec4, nil
		}
		return ir.Vec3, nil
	case builtinVec2:
		if len(c.Args) != 2 {
			return "", diagnostic(CodeInvalidCall, c.Span, "vec2f expects 2 arguments, got %d", len(c.Args))
		}
		for i, a := range c.Args {
			at, err := t.typeOf(a)
			if err != nil {
				return "", err
			}
			if at != ir.Float {
				return "", diagnostic(CodeTypeMismatch, c.Span, "vec2f argument %d must be float, got %s", i+1, at)
			}
		}
		return ir.Vec2, nil
	case builtinVec3:
		// vec3f(x,y,z) or vec3f(vec2,z): args must contribute exactly 3 float components.
		total := 0
		for i, a := range c.Args {
			at, err := t.typeOf(a)
			if err != nil {
				return "", err
			}
			n := floatComponents(at)
			if n == 0 || n > 3 {
				return "", diagnostic(CodeTypeMismatch, c.Span, "vec3f argument %d must be float, vec2, or vec3, got %s", i+1, at)
			}
			total += n
		}
		if total != 3 {
			return "", diagnostic(CodeInvalidCall, c.Span, "vec3f arguments must provide exactly 3 components, got %d", total)
		}
		return ir.Vec3, nil
	case builtinVec4:
		// vec4f(x,y,z,w) / vec4f(vec3,w) / vec4f(vec2,z,w): args must contribute exactly 4 float components.
		total := 0
		for i, a := range c.Args {
			at, err := t.typeOf(a)
			if err != nil {
				return "", err
			}
			n := floatComponents(at)
			if n == 0 {
				return "", diagnostic(CodeTypeMismatch, c.Span, "vec4f argument %d must be float, vec2, vec3, or vec4, got %s", i+1, at)
			}
			total += n
		}
		if total != 4 {
			return "", diagnostic(CodeInvalidCall, c.Span, "vec4f arguments must provide exactly 4 components, got %d", total)
		}
		return ir.Vec4, nil
	case builtinCastFloat, builtinCastInt, builtinCastUint:
		if len(c.Args) != spec.arity {
			return "", diagnostic(CodeInvalidCall, c.Span, "%s expects %d argument, got %d", c.Func, spec.arity, len(c.Args))
		}
		at, err := t.typeOf(c.Args[0])
		if err != nil {
			return "", err
		}
		if !isNumericScalar(at) {
			return "", diagnostic(CodeTypeMismatch, c.Span, "%s: argument must be a numeric scalar (float/int/uint), got %s", c.Func, at)
		}
		switch spec.kind {
		case builtinCastFloat:
			return ir.Float, nil
		case builtinCastInt:
			return ir.Int, nil
		default:
			return ir.Uint, nil
		}
	case builtinUnarySame:
		if len(c.Args) != spec.arity {
			return "", diagnostic(CodeInvalidCall, c.Span, "%s expects %d arguments, got %d", c.Func, spec.arity, len(c.Args))
		}
		return t.typeOf(c.Args[0])
	case builtinSameOrScalar:
		return t.sameOrScalarCall(c.Func, c.Args, spec.arity, c.Span)
	case builtinCross:
		if len(c.Args) != spec.arity {
			return "", diagnostic(CodeInvalidCall, c.Span, "%s expects %d arguments, got %d", c.Func, spec.arity, len(c.Args))
		}
		a, err := t.typeOf(c.Args[0])
		if err != nil {
			return "", err
		}
		b, err := t.typeOf(c.Args[1])
		if err != nil {
			return "", err
		}
		if a != ir.Vec3 || b != ir.Vec3 {
			return "", diagnostic(CodeTypeMismatch, c.Span, "cross arguments must both be vec3, got %s and %s", a, b)
		}
		return ir.Vec3, nil
	case builtinStep:
		if len(c.Args) != spec.arity {
			return "", diagnostic(CodeInvalidCall, c.Span, "%s expects %d arguments, got %d", c.Func, spec.arity, len(c.Args))
		}
		last, err := t.typeOf(c.Args[len(c.Args)-1])
		if err != nil {
			return "", err
		}
		for i := 0; i < len(c.Args)-1; i++ {
			at, err := t.typeOf(c.Args[i])
			if err != nil {
				return "", err
			}
			if at != last && at != ir.Float {
				return "", diagnostic(CodeTypeMismatch, c.Span, "%s argument %d must be %s or float, got %s", c.Func, i+1, last, at)
			}
		}
		return last, nil
	default:
		return "", diagnostic(CodeInvalidCall, c.Span, "builtin %q has unsupported registry kind %q", c.Func, spec.kind)
	}
}

func (t *typer) sameOrScalarCall(name string, args []hir.Expr, n int, span hir.Span) (ir.Type, error) {
	if len(args) != n {
		return "", diagnostic(CodeInvalidCall, span, "%s expects %d arguments, got %d", name, n, len(args))
	}
	base, err := t.typeOf(args[0])
	if err != nil {
		return "", err
	}
	for i := 1; i < len(args); i++ {
		at, err := t.typeOf(args[i])
		if err != nil {
			return "", err
		}
		if at != base && at != ir.Float {
			return "", diagnostic(CodeTypeMismatch, span, "%s argument %d must be %s or float, got %s", name, i+1, base, at)
		}
	}
	return base, nil
}

func isNumericScalar(t ir.Type) bool {
	return t == ir.Float || t == ir.Int || t == ir.Uint
}

func (t *typer) binaryType(b hir.Binary) (ir.Type, error) {
	// Comparison operators: operands must be the same numeric scalar, result is bool.
	// Scalar-only for now; vector component comparisons deferred.
	switch b.Op {
	case "<", ">", "<=", ">=", "==", "!=":
		lt, err := t.typeOf(b.L)
		if err != nil {
			return "", err
		}
		rt, err := t.typeOf(b.R)
		if err != nil {
			return "", err
		}
		if !isNumericScalar(lt) || lt != rt {
			return "", diagnostic(CodeTypeMismatch, b.Span, "comparison operator %s requires matching numeric operands, got %s and %s", b.Op, lt, rt)
		}
		return ir.Bool, nil
	// Boolean binary operators: both operands must be bool, result is bool.
	case "&&", "||":
		lt, err := t.typeOf(b.L)
		if err != nil {
			return "", err
		}
		rt, err := t.typeOf(b.R)
		if err != nil {
			return "", err
		}
		if lt != ir.Bool || rt != ir.Bool {
			return "", diagnostic(CodeTypeMismatch, b.Span, "operator %s requires bool operands, got %s and %s", b.Op, lt, rt)
		}
		return ir.Bool, nil
	}
	// Arithmetic operators.
	if b.Op != "+" && b.Op != "-" && b.Op != "*" && b.Op != "/" {
		return "", diagnostic(CodeInvalidCall, b.Span, "unsupported operator %q", b.Op)
	}
	lt, err := t.typeOf(b.L)
	if err != nil {
		return "", err
	}
	rt, err := t.typeOf(b.R)
	if err != nil {
		return "", err
	}
	if lt == rt {
		return lt, nil
	}
	if b.Op == "*" {
		if lt == ir.Mat4 && rt == ir.Vec4 {
			return ir.Vec4, nil
		}
		if lt == ir.Mat3 && rt == ir.Vec3 {
			return ir.Vec3, nil
		}
	}
	// float can mix with vector/matrix (scalar promotion).
	if lt == ir.Float {
		return rt, nil
	}
	if rt == ir.Float {
		return lt, nil
	}
	// int and uint only mix with themselves (no implicit coercion).
	return "", diagnostic(CodeTypeMismatch, b.Span, "operator %s is not defined for %s and %s", b.Op, lt, rt)
}

func isVector(t ir.Type) bool {
	return t == ir.Vec2 || t == ir.Vec3 || t == ir.Vec4
}

// floatComponents returns the number of float scalar components in t,
// or 0 if t is not a float-family type (e.g. mat3, mat4).
func floatComponents(t ir.Type) int {
	switch t {
	case ir.Float:
		return 1
	case ir.Vec2:
		return 2
	case ir.Vec3:
		return 3
	case ir.Vec4:
		return 4
	default:
		return 0
	}
}

func vectorWidth(t ir.Type) int {
	switch t {
	case ir.Vec2:
		return 2
	case ir.Vec3:
		return 3
	case ir.Vec4:
		return 4
	default:
		return 0
	}
}

func swizzleType(base ir.Type, field string) (ir.Type, error) {
	width := vectorWidth(base)
	if width == 0 {
		return "", fmt.Errorf("cannot swizzle %s with .%s", base, field)
	}
	var family byte
	for _, c := range field {
		var idx int
		var fam byte
		switch c {
		case 'x', 'y', 'z', 'w':
			fam = 'x'
			idx = strings.IndexRune("xyzw", c)
		case 'r', 'g', 'b', 'a':
			fam = 'r'
			idx = strings.IndexRune("rgba", c)
		default:
			return "", fmt.Errorf("invalid swizzle .%s for %s", field, base)
		}
		if family == 0 {
			family = fam
		} else if family != fam {
			return "", fmt.Errorf("invalid mixed swizzle .%s for %s", field, base)
		}
		if idx >= width {
			return "", fmt.Errorf("invalid swizzle .%s for %s", field, base)
		}
	}
	switch len(field) {
	case 1:
		return ir.Float, nil
	case 2:
		return ir.Vec2, nil
	case 3:
		return ir.Vec3, nil
	case 4:
		return ir.Vec4, nil
	}
	return "", fmt.Errorf("invalid swizzle .%s", field)
}
