package ir

import (
	"fmt"
	"reflect"
)

// FusedSpecular combines straight-line immutable stages into one evaluation.
// It alpha-renames companion locals and reuses structurally identical typed
// expressions. Control flow or mutation retains the separate-stage path: moving
// derivatives/sampling across discard, early returns, or assignments is unsafe.
// Unknown future expression nodes also decline optimization rather than emit a
// partially renamed program. Inputs are never mutated.
func FusedSpecular(m Module) (body []Stmt, color, specular Expr, ok bool) {
	if m.Specular == nil {
		return nil, nil, nil, false
	}
	for _, stage := range []Stage{m.Fragment, *m.Specular} {
		for _, s := range stage.Body {
			if s.CF != nil || s.Mutable || s.Target == "" {
				return nil, nil, nil, false
			}
		}
	}
	names := map[string]bool{}
	for _, bindings := range [][]Binding{m.Uniforms, m.Attributes, m.Varyings} {
		for _, b := range bindings {
			names[b.Name] = true
		}
	}
	for _, b := range m.ArrayUniforms {
		names[b.Name] = true
	}
	for _, b := range m.Textures {
		names[b.Name] = true
	}
	for _, stage := range []Stage{m.Fragment, *m.Specular} {
		for _, s := range stage.Body {
			names[s.Target] = true
		}
	}
	body = append([]Stmt(nil), m.Fragment.Body...)
	rename := map[string]string{}
	serial := 0
	for _, s := range m.Specular.Body {
		value, valid := renameRefs(s.Value, rename)
		if !valid {
			return nil, nil, nil, false
		}
		shared := ""
		for _, candidate := range body {
			if candidate.Type == s.Type && reflect.DeepEqual(candidate.Value, value) {
				shared = candidate.Target
				break
			}
		}
		if shared == "" {
			for {
				shared = fmt.Sprintf("selenaMRTLocal%d", serial)
				serial++
				if !names[shared] {
					break
				}
			}
			names[shared] = true
			body = append(body, Stmt{Target: shared, Type: s.Type, Value: value})
		}
		rename[s.Target] = shared
	}
	specular, ok = renameRefs(m.Specular.Output, rename)
	if !ok {
		return nil, nil, nil, false
	}
	return body, m.Fragment.Output, specular, true
}

func renameRefs(e Expr, names map[string]string) (Expr, bool) {
	ok := true
	child := func(e Expr) Expr { r, valid := renameRefs(e, names); ok = ok && valid; return r }
	list := func(es []Expr) []Expr {
		out := make([]Expr, len(es))
		for i, e := range es {
			out[i] = child(e)
		}
		return out
	}
	switch x := e.(type) {
	case nil:
		return nil, true
	case Ref:
		if name, exists := names[x.Name]; exists {
			x.Name = name
		}
		return x, true
	case Lit, IntLit, UintLit, SceneSize, StateSample, CellUV:
		return e, true
	case Construct:
		x.Args = list(x.Args)
		return x, ok
	case Call:
		x.Args = list(x.Args)
		return x, ok
	case Binary:
		x.L = child(x.L)
		x.R = child(x.R)
		return x, ok
	case Unary:
		x.E = child(x.E)
		return x, ok
	case Swizzle:
		x.E = child(x.E)
		return x, ok
	case Sample:
		x.UV = child(x.UV)
		return x, ok
	case SampleLevel:
		x.UV = child(x.UV)
		x.LOD = child(x.LOD)
		return x, ok
	case SampleCube:
		x.Dir = child(x.Dir)
		return x, ok
	case SampleCubeLevel:
		x.Dir = child(x.Dir)
		x.LOD = child(x.LOD)
		return x, ok
	case SceneSample:
		x.UV = child(x.UV)
		return x, ok
	case SceneSampleLevel:
		x.UV = child(x.UV)
		x.LOD = child(x.LOD)
		return x, ok
	case Conditional:
		x.Cond = child(x.Cond)
		x.Then = child(x.Then)
		x.Alt = child(x.Alt)
		return x, ok
	case StateSampleUV:
		x.UV = child(x.UV)
		return x, ok
	case Index:
		x.Arr = child(x.Arr)
		x.Idx = child(x.Idx)
		return x, ok
	default:
		return nil, false
	}
}
