package lower

import (
	"fmt"

	"m31labs.dev/selena/hir"
	"m31labs.dev/selena/ir"
)

// resolver lowers HIR expressions into backend-neutral IR expressions.
type resolver struct {
	paramKind        map[string]hir.Type
	uniformOf        map[string]string
	varyingOf        map[string]string
	geo              string
	// geoFields is the geometry registry used for this kind. When nil the
	// default mesh geometry (stdlib.geometry) is used.
	geoFields        map[string]geometrySpec
	// allowSceneSample enables sceneColor/sceneDepth calls (post kind only).
	allowSceneSample bool
}

func (r *resolver) expr(e hir.Expr) (ir.Expr, error) {
	switch x := e.(type) {
	case hir.Lit:
		return ir.Lit{Value: x.Value}, nil
	case hir.Ref:
		if un, ok := r.uniformOf[x.Name]; ok {
			return ir.Ref{Name: un}, nil
		}
		return ir.Ref{Name: x.Name}, nil // stage-local
	case hir.Member:
		if base, ok := x.E.(hir.Ref); ok {
			if base.Name == r.geo {
				vn, ok := r.varyingOf[x.Field]
				if !ok {
					return nil, diagnostic(CodeInvalidMember, x.Span, "geo.%s is not available in the surface", x.Field)
				}
				return ir.Ref{Name: vn}, nil
			}
			if r.paramKind[base.Name] == hir.Sun {
				un, ok := r.uniformOf[base.Name+"."+x.Field]
				if !ok {
					return nil, diagnostic(CodeInvalidMember, x.Span, "Sun has no field %q", x.Field)
				}
				return ir.Ref{Name: un}, nil
			}
		}
		// Swizzle on a scene sample result: lower the inner expression first.
		_ = 0 // fall through to default member handling below
		inner, err := r.expr(x.E)
		if err != nil {
			return nil, err
		}
		return ir.Swizzle{E: inner, Field: x.Field}, nil
	case hir.Call:
		if x.Func == "sceneColor" || x.Func == "sceneDepth" {
			if !r.allowSceneSample {
				return nil, diagnostic(CodeInvalidCall, x.Span, "%s is only available in post-kind materials", x.Func)
			}
			if len(x.Args) != 1 {
				return nil, diagnostic(CodeInvalidCall, x.Span, "%s(uv) takes 1 argument", x.Func)
			}
			uv, err := r.expr(x.Args[0])
			if err != nil {
				return nil, err
			}
			return ir.SceneSample{Name: x.Func, UV: uv}, nil
		}
		if x.Func == "sample" {
			if len(x.Args) != 2 {
				return nil, diagnostic(CodeInvalidCall, x.Span, "sample(texture, uv) takes 2 arguments")
			}
			texRef, ok := x.Args[0].(hir.Ref)
			if !ok || r.paramKind[texRef.Name] != hir.Texture2D {
				return nil, diagnostic(CodeInvalidCall, x.Span, "sample: first argument must be a texture2d param")
			}
			uv, err := r.expr(x.Args[1])
			if err != nil {
				return nil, err
			}
			return ir.Sample{Texture: texRef.Name, UV: uv}, nil
		}
		args, err := r.args(x.Args)
		if err != nil {
			return nil, err
		}
		if x.Func == "rgb" {
			t := ir.Vec3
			if len(args) == 4 {
				t = ir.Vec4
			}
			return ir.Construct{Type: t, Args: args}, nil
		}
		if x.Func == "vec2f" {
			return ir.Construct{Type: ir.Vec2, Args: args}, nil
		}
		return ir.Call{Func: x.Func, Args: args}, nil
	case hir.Binary:
		l, err := r.expr(x.L)
		if err != nil {
			return nil, err
		}
		rr, err := r.expr(x.R)
		if err != nil {
			return nil, err
		}
		return ir.Binary{Op: x.Op, L: l, R: rr}, nil
	case hir.Unary:
		e, err := r.expr(x.E)
		if err != nil {
			return nil, err
		}
		return ir.Unary{Op: x.Op, E: e}, nil
	}
	return nil, fmt.Errorf("unsupported expression %T", e)
}

func (r *resolver) args(in []hir.Expr) ([]ir.Expr, error) {
	out := make([]ir.Expr, len(in))
	for i, a := range in {
		v, err := r.expr(a)
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}
