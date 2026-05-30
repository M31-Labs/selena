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
	if len(funcs) == 0 {
		return m, nil
	}
	fm := make(map[string]hir.FuncDecl, len(funcs))
	for _, f := range funcs {
		fm[f.Name] = f
	}
	in := &inliner{funcs: fm}

	out := m
	out.Surface.Body = make([]hir.Let, 0, len(m.Surface.Body))
	for _, l := range m.Surface.Body {
		v, err := in.expr(l.Value, nil)
		if err != nil {
			return m, err
		}
		out.Surface.Body = append(out.Surface.Body, hir.Let{Name: l.Name, Value: v})
	}
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

// expr returns e with env substitutions applied and user-function calls inlined.
func (in *inliner) expr(e hir.Expr, env map[string]hir.Expr) (hir.Expr, error) {
	switch x := e.(type) {
	case hir.Lit:
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
		return hir.Member{E: obj, Field: x.Field}, nil
	case hir.Binary:
		l, err := in.expr(x.L, env)
		if err != nil {
			return nil, err
		}
		r, err := in.expr(x.R, env)
		if err != nil {
			return nil, err
		}
		return hir.Binary{Op: x.Op, L: l, R: r}, nil
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
			return hir.Call{Func: x.Func, Args: args}, nil // builtin / stdlib
		}
		if len(args) != len(fn.Params) {
			return nil, fmt.Errorf("fn %s expects %d args, got %d", fn.Name, len(fn.Params), len(args))
		}
		in.depth++
		if in.depth > 64 {
			return nil, fmt.Errorf("fn inlining too deep (recursive call to %s?)", fn.Name)
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
	case hir.SuperCall:
		if in.parent == nil {
			return nil, fmt.Errorf("super used in a material with no parent (extends)")
		}
		if x.Method != "surface" {
			return nil, fmt.Errorf("super.%s is not supported (only super.surface)", x.Method)
		}
		if len(x.Args) != 1 {
			return nil, fmt.Errorf("super.surface expects 1 argument (the geometry)")
		}
		geoArg, err := in.expr(x.Args[0], env)
		if err != nil {
			return nil, err
		}
		// inline the parent surface with its geo param bound to the passed arg
		env2 := map[string]hir.Expr{in.parent.Geo: geoArg}
		for _, l := range in.parent.Body {
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
