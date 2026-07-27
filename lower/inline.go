package lower

import (
	"fmt"

	"m31labs.dev/selena/hir"
)

// inlineFuncs rewrites a material's surface so every call to a user-defined
// function is replaced by that function's result, with parameters (and the
// function's own locals) substituted. Selena functions are total and
// non-recursive, so this terminates and needs no backend function support — the
// LIR and all emitters are untouched.
func inlineFuncs(m hir.Material, funcs []hir.FuncDecl) (hir.Material, error) {
	fm := make(map[string]hir.FuncDecl, len(funcs))
	for _, f := range funcs {
		fm[f.Name] = f
	}
	in := &inliner{funcs: fm}

	out := m
	body, err := in.stmts(m.Surface.Body, nil)
	if err != nil {
		return m, err
	}
	out.Surface.Body = body
	r, err := in.expr(m.Surface.Result, nil)
	if err != nil {
		return m, err
	}
	out.Surface.Result = r
	return out, nil
}

type inliner struct {
	funcs  map[string]hir.FuncDecl
	parent *hir.Func // parent material's surface, for super.surface(...)
	depth  int
}

// stmts applies substitutions + inlining to all statements in a slice.
func (in *inliner) stmts(ss []hir.Stmt, env map[string]hir.Expr) ([]hir.Stmt, error) {
	out := make([]hir.Stmt, 0, len(ss))
	for _, s := range ss {
		processed, err := in.stmt(s, env)
		if err != nil {
			return nil, err
		}
		out = append(out, processed)
	}
	return out, nil
}

// stmt applies substitutions + inlining to a single HIR statement.
func (in *inliner) stmt(s hir.Stmt, env map[string]hir.Expr) (hir.Stmt, error) {
	switch x := s.(type) {
	case hir.Let:
		v, err := in.expr(x.Value, env)
		if err != nil {
			return nil, err
		}
		return hir.Let{Name: x.Name, Value: v, Span: x.Span}, nil
	case hir.VarDecl:
		v, err := in.expr(x.Value, env)
		if err != nil {
			return nil, err
		}
		return hir.VarDecl{Name: x.Name, Value: v, Span: x.Span}, nil
	case hir.Assign:
		v, err := in.expr(x.Value, env)
		if err != nil {
			return nil, err
		}
		return hir.Assign{Name: x.Name, Value: v, Span: x.Span}, nil
	case hir.If:
		cond, err := in.expr(x.Cond, env)
		if err != nil {
			return nil, err
		}
		then, err := in.stmts(x.Then, env)
		if err != nil {
			return nil, err
		}
		var els []hir.Stmt
		if len(x.Else) > 0 {
			els, err = in.stmts(x.Else, env)
			if err != nil {
				return nil, err
			}
		}
		return hir.If{Cond: cond, Then: then, Else: els, Span: x.Span}, nil
	case hir.For:
		initVal, err := in.expr(x.InitValue, env)
		if err != nil {
			return nil, err
		}
		cond, err := in.expr(x.Cond, env)
		if err != nil {
			return nil, err
		}
		postVal, err := in.expr(x.PostValue, env)
		if err != nil {
			return nil, err
		}
		body, err := in.stmts(x.Body, env)
		if err != nil {
			return nil, err
		}
		return hir.For{
			InitName: x.InitName, InitValue: initVal,
			Cond:      cond,
			PostName:  x.PostName, PostValue: postVal,
			Body:      body,
			Span:      x.Span,
		}, nil
	case hir.VarArrayDecl:
		// No expressions to inline inside a typed array declaration.
		return x, nil
	case hir.Discard:
		// No expressions to inline in a bare discard statement.
		return x, nil
	case hir.Break:
		// Likewise: a bare break carries no expressions.
		return x, nil
	case hir.Return:
		v, err := in.expr(x.Value, env)
		if err != nil {
			return nil, err
		}
		return hir.Return{Value: v, Span: x.Span}, nil
	case hir.IndexAssign:
		idx, err := in.expr(x.Index, env)
		if err != nil {
			return nil, err
		}
		val, err := in.expr(x.Value, env)
		if err != nil {
			return nil, err
		}
		return hir.IndexAssign{Name: x.Name, Index: idx, Value: val, Span: x.Span}, nil
	}
	return nil, fmt.Errorf("unsupported stmt type %T in inliner", s)
}

// expr returns e with env substitutions applied and user-function calls inlined.
func (in *inliner) expr(e hir.Expr, env map[string]hir.Expr) (hir.Expr, error) {
	switch x := e.(type) {
	case hir.Lit:
		return x, nil
	case hir.IntLit:
		return x, nil
	case hir.UintLit:
		return x, nil
	case hir.Ref:
		if env != nil {
			if v, ok := env[x.Name]; ok {
				return v, nil
			}
		}
		return x, nil
	case hir.Member:
		obj, err := in.expr(x.E, env)
		if err != nil {
			return nil, err
		}
		return hir.Member{E: obj, Field: x.Field, Span: x.Span}, nil
	case hir.Binary:
		l, err := in.expr(x.L, env)
		if err != nil {
			return nil, err
		}
		r, err := in.expr(x.R, env)
		if err != nil {
			return nil, err
		}
		return hir.Binary{Op: x.Op, L: l, R: r, Span: x.Span}, nil
	case hir.Unary:
		e, err := in.expr(x.E, env)
		if err != nil {
			return nil, err
		}
		return hir.Unary{Op: x.Op, E: e, Span: x.Span}, nil
	case hir.Call:
		args := make([]hir.Expr, len(x.Args))
		for i, a := range x.Args {
			v, err := in.expr(a, env)
			if err != nil {
				return nil, err
			}
			args[i] = v
		}
		fn, ok := in.funcs[x.Func]
		if !ok {
			return hir.Call{Func: x.Func, Args: args, Span: x.Span}, nil // builtin / stdlib
		}
		if len(args) != len(fn.Params) {
			return nil, diagnostic(CodeInvalidCall, x.Span, "fn %s expects %d args, got %d", fn.Name, len(fn.Params), len(args))
		}
		in.depth++
		if in.depth > 64 {
			return nil, diagnostic(CodeInvalidCall, x.Span, "fn inlining too deep (recursive call to %s?)", fn.Name)
		}
		env2 := make(map[string]hir.Expr, len(fn.Params)+len(fn.Body))
		for i, p := range fn.Params {
			env2[p.Name] = args[i]
		}
		for _, l := range fn.Body {
			v, err := in.expr(l.Value, env2)
			if err != nil {
				in.depth--
				return nil, err
			}
			env2[l.Name] = v
		}
		res, err := in.expr(fn.Result, env2)
		in.depth--
		return res, err
	case hir.Conditional:
		cond, err := in.expr(x.Cond, env)
		if err != nil {
			return nil, err
		}
		then, err := in.expr(x.Then, env)
		if err != nil {
			return nil, err
		}
		alt, err := in.expr(x.Alt, env)
		if err != nil {
			return nil, err
		}
		return hir.Conditional{Cond: cond, Then: then, Alt: alt, Span: x.Span}, nil
	case hir.IndexExpr:
		arr, err := in.expr(x.Arr, env)
		if err != nil {
			return nil, err
		}
		idx, err := in.expr(x.Index, env)
		if err != nil {
			return nil, err
		}
		return hir.IndexExpr{Arr: arr, Index: idx, Span: x.Span}, nil
	case hir.SuperCall:
		if in.parent == nil {
			return nil, diagnostic(CodeInvalidCall, x.Span, "super.%s used in a material with no parent (extends)", x.Method)
		}
		if x.Method != "surface" {
			return nil, diagnostic(CodeInvalidCall, x.Span, "super.%s is not supported (only super.surface)", x.Method)
		}
		if len(x.Args) != 1 {
			return nil, diagnostic(CodeInvalidCall, x.Span, "super.surface expects 1 argument (the geometry), got %d", len(x.Args))
		}
		geoArg, err := in.expr(x.Args[0], env)
		if err != nil {
			return nil, err
		}
		// inline the parent surface with its geo param bound to the passed arg.
		// Only Let statements in the parent body are supported for inlining;
		// control flow in a parent surface (var/assign/if/for, including an
		// early return) is expression-substituted, not compiled, so it has no
		// statement stream to splice a child's usage into — super.surface(...)
		// on such a parent is not yet supported.
		env2 := map[string]hir.Expr{in.parent.Geo: geoArg}
		for _, s := range in.parent.Body {
			l, ok := s.(hir.Let)
			if !ok {
				return nil, diagnostic(CodeInvalidCall, x.Span,
					"super.surface: parent surface body contains control flow (if, for, or an early return) that cannot be inlined; only let bindings are supported here")
			}
			v, err := in.expr(l.Value, env2)
			if err != nil {
				return nil, err
			}
			env2[l.Name] = v
		}
		return in.expr(in.parent.Result, env2)
	}
	return nil, fmt.Errorf("unsupported expression %T in inliner", e)
}
