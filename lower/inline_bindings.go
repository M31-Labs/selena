package lower

import (
	"fmt"
	"m31labs.dev/selena/hir"
)

func (in *inliner) bind(value hir.Expr, expected hir.Type, span hir.Span) hir.Expr {
	for {
		name := fmt.Sprintf("selenaLib%d", in.serial)
		in.serial++
		if in.names[name] {
			continue
		}
		in.pending = append(in.pending, hir.Let{Name: name, Value: value, Expected: expected, Span: span})
		return hir.Ref{Name: name, Span: span}
	}
}

func materialNames(m hir.Material) map[string]bool {
	names := map[string]bool{}
	for _, p := range m.Params {
		names[p.Name] = true
	}
	for _, p := range m.Context {
		names[p.Name] = true
	}
	for _, v := range m.Varyings {
		names[v.Name] = true
	}
	var visit func([]hir.Stmt)
	visit = func(ss []hir.Stmt) {
		for _, st := range ss {
			switch s := st.(type) {
			case hir.Let:
				names[s.Name] = true
			case hir.VarDecl:
				names[s.Name] = true
			case hir.VarArrayDecl:
				names[s.Name] = true
			case hir.If:
				visit(s.Then)
				visit(s.Else)
			case hir.For:
				names[s.InitName] = true
				visit(s.Body)
			}
		}
	}
	names[m.Surface.Geo] = true
	visit(m.Surface.Body)
	if m.Vertex != nil {
		names[m.Vertex.Geo] = true
		visit(m.Vertex.Body)
	}
	return names
}
