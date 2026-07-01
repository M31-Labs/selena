package lower

import (
	"fmt"

	"m31labs.dev/selena/bindings"
	"m31labs.dev/selena/hir"
	"m31labs.dev/selena/ir"
)

// resolveExtends flattens m's `extends` chain: it merges the parent's params
// (child overrides by name) and inlines super.surface(geo) into the child's
// surface using the parent's (recursively flattened) surface. all is the set of
// materials parents resolve against.
func resolveExtends(m hir.Material, all []hir.Material) (hir.Material, error) {
	if m.Extends == "" {
		return m, nil
	}
	parent, ok := findMaterial(all, m.Extends)
	if !ok {
		return m, fmt.Errorf("material %q extends unknown material %q", m.Name, m.Extends)
	}
	parent, err := resolveExtends(parent, all)
	if err != nil {
		return m, err
	}

	out := m
	out.Extends = ""
	out.Params = mergeParams(parent.Params, m.Params)

	in := &inliner{parent: &parent.Surface}
	surf := m.Surface
	body, err := in.stmts(m.Surface.Body, nil)
	if err != nil {
		return m, err
	}
	surf.Body = body
	r, err := in.expr(m.Surface.Result, nil)
	if err != nil {
		return m, err
	}
	surf.Result = r
	out.Surface = surf
	return out, nil
}

func findMaterial(all []hir.Material, name string) (hir.Material, bool) {
	for _, m := range all {
		if m.Name == name {
			return m, true
		}
	}
	return hir.Material{}, false
}

// mergeParams returns parent params followed by child params, child overriding
// a parent of the same name (kept in the child's position, not duplicated).
func mergeParams(parent, child []hir.Param) []hir.Param {
	childHas := make(map[string]bool, len(child))
	for _, p := range child {
		childHas[p.Name] = true
	}
	out := make([]hir.Param, 0, len(parent)+len(child))
	for _, p := range parent {
		if !childHas[p.Name] {
			out = append(out, p)
		}
	}
	return append(out, child...)
}

// LowerProgram resolves extends and inlines functions for material i in p, then
// lowers it to an ir.Module + binding layout.
func LowerProgram(p hir.Program, i int) (ir.Module, bindings.Layout, error) {
	if i < 0 || i >= len(p.Materials) {
		return ir.Module{}, bindings.Layout{}, fmt.Errorf("material index %d out of range", i)
	}
	flat, err := resolveExtends(p.Materials[i], p.Materials)
	if err != nil {
		return ir.Module{}, bindings.Layout{}, err
	}
	inlined, err := inlineFuncs(flat, p.Funcs)
	if err != nil {
		return ir.Module{}, bindings.Layout{}, err
	}
	return Lower(inlined)
}
