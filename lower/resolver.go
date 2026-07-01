package lower

import (
	"fmt"
	"math"

	"m31labs.dev/selena/hir"
	"m31labs.dev/selena/ir"
)

// resolver lowers HIR expressions into backend-neutral IR expressions.
type resolver struct {
	paramKind map[string]hir.Type
	uniformOf map[string]string
	varyingOf map[string]string
	geo       string
	// geoFields is the geometry registry used for this kind. When nil the
	// default mesh geometry (stdlib.geometry) is used.
	geoFields map[string]geometrySpec
	// geoBuild maps geometry field names to builder functions that return the
	// IR expression for that field. Takes precedence over varyingOf for the same
	// field. Used by feedback materials to expose cell.uv → ir.CellUV{}.
	geoBuild map[string]func() ir.Expr
	// allowSceneSample enables sceneColor/sceneDepth calls (post kind only).
	allowSceneSample bool
	// allowState enables the state(dx, dy) previous-state read (feedback kind only).
	allowState bool
	// allowVertexIndex enables the vertexIndex builtin (authored vertex stage). B4.
	allowVertexIndex bool
	// allowStateAt enables stateAt(uv) reads (mesh/general statefield). B4.
	allowStateAt bool
	// allowDiscard enables the fragment-only `discard` statement. Set for the
	// fragment stage of mesh and post kinds; left false for vertex stages and for
	// kinds (points, feedback) that do not support discard.
	allowDiscard bool
	// tp is a reference to the companion typer used during lowering. When set,
	// the resolver populates ir.Call.ArgTypes for component-wise builtins that
	// accept mixed vec/scalar arguments, enabling WGSL auto-splat in the emitter.
	tp *typer
}

func (r *resolver) expr(e hir.Expr) (ir.Expr, error) {
	switch x := e.(type) {
	case hir.Lit:
		return ir.Lit{Value: x.Value}, nil
	case hir.IntLit:
		return ir.IntLit{Value: x.Value}, nil
	case hir.UintLit:
		return ir.UintLit{Value: x.Value}, nil
	case hir.Ref:
		if x.Name == "vertexIndex" && r.allowVertexIndex {
			// The vertexIndex builtin keeps its name in the IR; each backend's
			// authored-vertex emitter binds `vertexIndex` to its index source.
			return ir.Ref{Name: "vertexIndex"}, nil
		}
		if un, ok := r.uniformOf[x.Name]; ok {
			return ir.Ref{Name: un}, nil
		}
		return ir.Ref{Name: x.Name}, nil // stage-local
	case hir.Member:
		if base, ok := x.E.(hir.Ref); ok {
			if base.Name == r.geo {
				if build, ok := r.geoBuild[x.Field]; ok {
					return build(), nil
				}
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
		if x.Func == "state" {
			if !r.allowState {
				return nil, diagnostic(CodeInvalidCall, x.Span, "state(dx, dy) is only available in feedback-kind materials")
			}
			if len(x.Args) != 2 {
				return nil, diagnostic(CodeInvalidCall, x.Span, "state(dx, dy) takes 2 arguments")
			}
			dx, ok := constInt(x.Args[0])
			if !ok {
				return nil, diagnostic(CodeInvalidCall, x.Span, "state: dx must be a constant integer offset")
			}
			dy, ok := constInt(x.Args[1])
			if !ok {
				return nil, diagnostic(CodeInvalidCall, x.Span, "state: dy must be a constant integer offset")
			}
			return ir.StateSample{DX: dx, DY: dy}, nil
		}
		if x.Func == "stateAt" {
			if !r.allowStateAt {
				return nil, diagnostic(CodeInvalidCall, x.Span, "stateAt(uv) requires a statefield; declare `state <name>` in the material")
			}
			if len(x.Args) != 1 {
				return nil, diagnostic(CodeInvalidCall, x.Span, "stateAt(uv) takes 1 argument")
			}
			uv, err := r.expr(x.Args[0])
			if err != nil {
				return nil, err
			}
			return ir.StateSampleUV{UV: uv}, nil
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
		if x.Func == "sampleCube" {
			if len(x.Args) != 2 {
				return nil, diagnostic(CodeInvalidCall, x.Span, "sampleCube(texture, dir) takes 2 arguments")
			}
			texRef, ok := x.Args[0].(hir.Ref)
			if !ok || r.paramKind[texRef.Name] != hir.TextureCube {
				return nil, diagnostic(CodeInvalidCall, x.Span, "sampleCube: first argument must be a textureCube param")
			}
			dir, err := r.expr(x.Args[1])
			if err != nil {
				return nil, err
			}
			return ir.SampleCube{Texture: texRef.Name, Dir: dir}, nil
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
		if x.Func == "vec3f" {
			return ir.Construct{Type: ir.Vec3, Args: args}, nil
		}
		if x.Func == "vec4f" {
			return ir.Construct{Type: ir.Vec4, Args: args}, nil
		}
		// Scalar conversions lower to an ir.Construct of the target scalar type
		// (WGSL f32()/i32()/u32(), GLSL/GLES/Metal float()/int()/uint()). B4.
		if x.Func == "float" {
			return ir.Construct{Type: ir.Float, Args: args}, nil
		}
		if x.Func == "int" {
			return ir.Construct{Type: ir.Int, Args: args}, nil
		}
		if x.Func == "uint" {
			return ir.Construct{Type: ir.Uint, Args: args}, nil
		}
		// For builtinSameOrScalar and builtinStep, populate ArgTypes so the
		// WGSL emitter can auto-splat scalar args to the base vector type.
		// GLSL/GLES/Metal broadcast scalar args natively and ignore ArgTypes.
		if r.tp != nil {
			if spec, ok := stdlib.builtin(x.Func); ok {
				switch spec.kind {
				case builtinSameOrScalar, builtinStep:
					argTypes := r.buildArgTypes(x.Args)
					if resolverHasVecScalarMix(argTypes) {
						return ir.Call{Func: x.Func, Args: args, ArgTypes: argTypes}, nil
					}
				}
			}
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
	case hir.Conditional:
		cond, err := r.expr(x.Cond)
		if err != nil {
			return nil, err
		}
		then, err := r.expr(x.Then)
		if err != nil {
			return nil, err
		}
		alt, err := r.expr(x.Alt)
		if err != nil {
			return nil, err
		}
		return ir.Conditional{Cond: cond, Then: then, Alt: alt}, nil
	case hir.IndexExpr:
		arr, err := r.expr(x.Arr)
		if err != nil {
			return nil, err
		}
		idx, err := r.expr(x.Index)
		if err != nil {
			return nil, err
		}
		return ir.Index{Arr: arr, Idx: idx}, nil
	}
	return nil, fmt.Errorf("unsupported expression %T", e)
}

// constInt evaluates a compile-time integer constant from a HIR expression:
// an integer literal, an integer-valued number literal, or a unary negation of
// either. Used for the state(dx, dy) stencil offsets.
func constInt(e hir.Expr) (int64, bool) {
	switch x := e.(type) {
	case hir.IntLit:
		return x.Value, true
	case hir.Lit:
		if x.Value == math.Trunc(x.Value) {
			return int64(x.Value), true
		}
		return 0, false
	case hir.Unary:
		if x.Op == "-" {
			if v, ok := constInt(x.E); ok {
				return -v, true
			}
		}
		return 0, false
	}
	return 0, false
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

// buildArgTypes infers the IR type of each HIR expression by delegating to the
// companion typer. Empty string ("") is stored for any arg whose type cannot be
// determined; those positions are treated as "unknown" and never splatted.
func (r *resolver) buildArgTypes(args []hir.Expr) []ir.Type {
	types := make([]ir.Type, len(args))
	for i, a := range args {
		if t, err := r.tp.typeOf(a); err == nil {
			types[i] = t
		}
	}
	return types
}

// resolverHasVecScalarMix reports whether types contains at least one vector
// type (vec2/vec3/vec4) and at least one float scalar, signalling that WGSL
// needs a splat to satisfy its same-type-per-argument rule.
func resolverHasVecScalarMix(types []ir.Type) bool {
	hasVec, hasScalar := false, false
	for _, t := range types {
		switch t {
		case ir.Vec2, ir.Vec3, ir.Vec4:
			hasVec = true
		case ir.Float:
			hasScalar = true
		}
	}
	return hasVec && hasScalar
}
