package lower

import (
	"fmt"
	"sort"

	"m31labs.dev/selena/bindings"
	"m31labs.dev/selena/hir"
	"m31labs.dev/selena/ir"
)

func collectGeo(e hir.Expr, geo string, used map[string]bool) {
	switch x := e.(type) {
	case hir.Member:
		if b, ok := x.E.(hir.Ref); ok && b.Name == geo {
			used[x.Field] = true
		}
		collectGeo(x.E, geo, used)
	case hir.Call:
		for _, a := range x.Args {
			collectGeo(a, geo, used)
		}
	case hir.Binary:
		collectGeo(x.L, geo, used)
		collectGeo(x.R, geo, used)
	case hir.Unary:
		collectGeo(x.E, geo, used)
	case hir.Conditional:
		collectGeo(x.Cond, geo, used)
		collectGeo(x.Then, geo, used)
		collectGeo(x.Alt, geo, used)
	case hir.IndexExpr:
		collectGeo(x.Arr, geo, used)
		collectGeo(x.Index, geo, used)
	}
}

// collectGeoStmt recursively collects geometry field references from all
// expressions inside a HIR statement (including nested if/for bodies).
func collectGeoStmt(s hir.Stmt, geo string, used map[string]bool) {
	switch x := s.(type) {
	case hir.Let:
		collectGeo(x.Value, geo, used)
	case hir.VarDecl:
		collectGeo(x.Value, geo, used)
	case hir.Assign:
		collectGeo(x.Value, geo, used)
	case hir.If:
		collectGeo(x.Cond, geo, used)
		for _, st := range x.Then {
			collectGeoStmt(st, geo, used)
		}
		for _, st := range x.Else {
			collectGeoStmt(st, geo, used)
		}
	case hir.For:
		collectGeo(x.InitValue, geo, used)
		collectGeo(x.Cond, geo, used)
		collectGeo(x.PostValue, geo, used)
		for _, st := range x.Body {
			collectGeoStmt(st, geo, used)
		}
	case hir.VarArrayDecl:
		// No expressions in a bare array declaration.
	case hir.IndexAssign:
		collectGeo(x.Index, geo, used)
		collectGeo(x.Value, geo, used)
	}
}

func toBindings(ns []bindings.NamedType) []ir.Binding {
	out := make([]ir.Binding, 0, len(ns))
	for _, n := range ns {
		if n.Count > 1 {
			continue // array uniforms go into ir.Module.ArrayUniforms, not Uniforms
		}
		out = append(out, ir.Binding{Name: n.Name, Type: n.Type})
	}
	return out
}

// toArrayBindings extracts fixed-size array uniforms (Count > 1) from ns.
func toArrayBindings(ns []bindings.NamedType) []ir.ArrayBinding {
	var out []ir.ArrayBinding
	for _, n := range ns {
		if n.Count > 1 {
			out = append(out, ir.ArrayBinding{Name: n.Name, ElemType: n.Type, Count: n.Count})
		}
	}
	return out
}

func toAttrs(bs []ir.Binding) []bindings.Attribute {
	out := make([]bindings.Attribute, len(bs))
	for i, b := range bs {
		out[i] = bindings.Attribute{Name: b.Name, Location: i, Type: string(b.Type)}
	}
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

func sortedSet(m map[string]bool) []string {
	ks := make([]string, 0, len(m))
	for k, v := range m {
		if v {
			ks = append(ks, k)
		}
	}
	sort.Strings(ks)
	return ks
}

func interfaceNames(uniforms []bindings.NamedType, textures []string, attributes, varyings []ir.Binding) (map[string]string, error) {
	seen := map[string]string{}
	claim := func(kind, name string) error {
		if prev, ok := seen[name]; ok {
			return fmt.Errorf("%s %q conflicts with %s", kind, name, prev)
		}
		seen[name] = kind
		return nil
	}
	for _, u := range uniforms {
		if err := claim("uniform", u.Name); err != nil {
			return nil, err
		}
	}
	for _, t := range textures {
		if err := claim("texture", t); err != nil {
			return nil, err
		}
		if err := claim("texture sampler", t+"Sampler"); err != nil {
			return nil, err
		}
	}
	for _, a := range attributes {
		if err := claim("attribute", a.Name); err != nil {
			return nil, err
		}
	}
	for _, v := range varyings {
		if err := claim("varying", v.Name); err != nil {
			return nil, err
		}
	}
	return seen, nil
}
