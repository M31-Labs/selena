package lower

import (
	"fmt"
	"strings"

	"m31labs.dev/selena/hir"
	"m31labs.dev/selena/ir"
)

// typer infers the IR type of a HIR expression within one material surface.
type typer struct {
	paramKind        map[string]hir.Type
	geo              string
	locals           map[string]ir.Type
	// geoFields is the geometry registry for this kind. Nil means mesh stdlib.
	geoFields        map[string]geometrySpec
	// allowSceneSample enables sceneColor/sceneDepth (post kind only).
	allowSceneSample bool
}

func (t *typer) typeOf(e hir.Expr) (ir.Type, error) {
	switch x := e.(type) {
	case hir.Lit:
		return ir.Float, nil
	case hir.Ref:
		if lt, ok := t.locals[x.Name]; ok {
			return lt, nil
		}
		if pk, ok := t.paramKind[x.Name]; ok {
			if it, ok := hirToIRType(pk); ok {
				return it, nil
			}
			return "", diagnostic(CodeInvalidMember, x.Span, "%q has record type %q and must be accessed by field", x.Name, pk)
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
		return t.typeOf(x.E)
	}
	return "", fmt.Errorf("unsupported expression %T", e)
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

	spec, ok := stdlib.builtin(c.Func)
	if !ok {
		if len(c.Args) == 0 {
			return "", diagnostic(CodeInvalidCall, c.Span, "call %q has no arguments", c.Func)
		}
		return t.typeOf(c.Args[0])
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

func (t *typer) binaryType(b hir.Binary) (ir.Type, error) {
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
	if lt == ir.Float {
		return rt, nil
	}
	if rt == ir.Float {
		return lt, nil
	}
	return "", diagnostic(CodeTypeMismatch, b.Span, "operator %s is not defined for %s and %s", b.Op, lt, rt)
}

func isVector(t ir.Type) bool {
	return t == ir.Vec2 || t == ir.Vec3 || t == ir.Vec4
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
