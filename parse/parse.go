// Package parse turns .sel source into the high-level material model (hir).
// It generates the Selena tree-sitter language from grammar.SelenaGrammar via
// grammargen (the same engine as .gsx), parses the source, and walks the tree
// into a hir.Material. This is the front-end: with it, materials are authored
// as .sel files rather than hand-built HIR.
package parse

import (
	"fmt"
	"strconv"
	"sync"

	gts "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammargen"

	"m31labs.dev/selena/grammar"
	"m31labs.dev/selena/hir"
)

var (
	langOnce sync.Once
	lang     *gts.Language
	langErr  error
)

// language generates (and caches) the Selena tree-sitter language.
func language() (*gts.Language, error) {
	langOnce.Do(func() {
		lang, _, langErr = grammargen.GenerateLanguageAndBlob(grammar.SelenaGrammar())
	})
	return lang, langErr
}

// Material parses src and returns the first material it declares.
func Material(src []byte) (hir.Material, error) {
	l, err := language()
	if err != nil {
		return hir.Material{}, fmt.Errorf("generate selena language: %w", err)
	}
	tree, err := gts.NewParser(l).Parse(src)
	if err != nil {
		return hir.Material{}, fmt.Errorf("parse: %w", err)
	}
	root := tree.RootNode()
	if root.HasError() {
		return hir.Material{}, fmt.Errorf("syntax error in .sel source")
	}
	w := &walker{lang: l, src: src}
	for i := 0; i < root.NamedChildCount(); i++ {
		if c := root.NamedChild(i); w.typ(c) == "material" {
			return w.material(c)
		}
	}
	return hir.Material{}, fmt.Errorf("no material declared")
}

// Program parses src into all of its functions and materials.
func Program(src []byte) (hir.Program, error) {
	l, err := language()
	if err != nil {
		return hir.Program{}, fmt.Errorf("generate selena language: %w", err)
	}
	tree, err := gts.NewParser(l).Parse(src)
	if err != nil {
		return hir.Program{}, fmt.Errorf("parse: %w", err)
	}
	root := tree.RootNode()
	if root.HasError() {
		return hir.Program{}, fmt.Errorf("syntax error in .sel source")
	}
	w := &walker{lang: l, src: src}
	var p hir.Program
	for i := 0; i < root.NamedChildCount(); i++ {
		c := root.NamedChild(i)
		switch w.typ(c) {
		case "material":
			m, err := w.material(c)
			if err != nil {
				return p, err
			}
			p.Materials = append(p.Materials, m)
		case "fn_decl":
			fd, err := w.funcDecl(c)
			if err != nil {
				return p, err
			}
			p.Funcs = append(p.Funcs, fd)
		}
	}
	return p, nil
}

type walker struct {
	lang *gts.Language
	src  []byte
}

func (w *walker) typ(n *gts.Node) string                { return n.Type(w.lang) }
func (w *walker) text(n *gts.Node) string               { return n.Text(w.src) }
func (w *walker) field(n *gts.Node, f string) *gts.Node { return n.ChildByFieldName(f, w.lang) }

func (w *walker) material(n *gts.Node) (hir.Material, error) {
	m := hir.Material{Name: w.text(w.field(n, "name"))}
	for i := 0; i < n.NamedChildCount(); i++ {
		c := n.NamedChild(i)
		if w.typ(c) != "member" {
			continue
		}
		inner := c.NamedChild(0)
		switch w.typ(inner) {
		case "param":
			m.Params = append(m.Params, hir.Param{
				Name: w.text(w.field(inner, "name")),
				Type: hir.Type(w.text(w.field(inner, "type"))),
			})
		case "surface":
			sf, err := w.fn(inner)
			if err != nil {
				return m, err
			}
			m.Surface = sf
		}
	}
	if m.Surface.Geo == "" {
		return m, fmt.Errorf("material %q has no surface", m.Name)
	}
	return m, nil
}

func (w *walker) fn(n *gts.Node) (hir.Func, error) {
	f := hir.Func{Geo: w.text(w.field(n, "geo"))}
	body := w.field(n, "body")
	for i := 0; i < body.NamedChildCount(); i++ {
		st := body.NamedChild(i)
		if w.typ(st) != "statement" {
			continue
		}
		s := st.NamedChild(0)
		switch w.typ(s) {
		case "let_stmt":
			e, err := w.expr(w.field(s, "value"))
			if err != nil {
				return f, err
			}
			f.Body = append(f.Body, hir.Let{Name: w.text(w.field(s, "name")), Value: e})
		case "return_stmt":
			e, err := w.expr(w.field(s, "value"))
			if err != nil {
				return f, err
			}
			f.Result = e
		}
	}
	if f.Result == nil {
		return f, fmt.Errorf("surface has no return")
	}
	return f, nil
}

func (w *walker) funcDecl(n *gts.Node) (hir.FuncDecl, error) {
	fd := hir.FuncDecl{
		Name:    w.text(w.field(n, "name")),
		Returns: hir.Type(w.text(w.field(n, "returns"))),
	}
	for i := 0; i < n.NamedChildCount(); i++ {
		c := n.NamedChild(i)
		if w.typ(c) != "fn_params" {
			continue
		}
		for j := 0; j < c.NamedChildCount(); j++ {
			p := c.NamedChild(j)
			if w.typ(p) != "fn_param" {
				continue
			}
			fd.Params = append(fd.Params, hir.Param{
				Name: w.text(w.field(p, "name")),
				Type: hir.Type(w.text(w.field(p, "type"))),
			})
		}
	}
	body := w.field(n, "body")
	for i := 0; i < body.NamedChildCount(); i++ {
		st := body.NamedChild(i)
		if w.typ(st) != "statement" {
			continue
		}
		s := st.NamedChild(0)
		switch w.typ(s) {
		case "let_stmt":
			e, err := w.expr(w.field(s, "value"))
			if err != nil {
				return fd, err
			}
			fd.Body = append(fd.Body, hir.Let{Name: w.text(w.field(s, "name")), Value: e})
		case "return_stmt":
			e, err := w.expr(w.field(s, "value"))
			if err != nil {
				return fd, err
			}
			fd.Result = e
		}
	}
	if fd.Result == nil {
		return fd, fmt.Errorf("fn %q has no return", fd.Name)
	}
	return fd, nil
}

func (w *walker) expr(n *gts.Node) (hir.Expr, error) {
	switch w.typ(n) {
	case "number":
		v, err := strconv.ParseFloat(w.text(n), 64)
		if err != nil {
			return nil, fmt.Errorf("bad number %q: %w", w.text(n), err)
		}
		return hir.Lit{Value: v}, nil
	case "identifier":
		return hir.Ref{Name: w.text(n)}, nil
	case "paren_expression":
		return w.expr(n.NamedChild(0))
	case "member_expression":
		obj, err := w.expr(w.field(n, "object"))
		if err != nil {
			return nil, err
		}
		return hir.Member{E: obj, Field: w.text(w.field(n, "field"))}, nil
	case "binary_expression":
		l, err := w.expr(w.field(n, "left"))
		if err != nil {
			return nil, err
		}
		r, err := w.expr(w.field(n, "right"))
		if err != nil {
			return nil, err
		}
		return hir.Binary{Op: w.text(w.field(n, "operator")), L: l, R: r}, nil
	case "call":
		var args []hir.Expr
		for i := 0; i < n.NamedChildCount(); i++ {
			c := n.NamedChild(i)
			if w.typ(c) != "arguments" {
				continue
			}
			for j := 0; j < c.NamedChildCount(); j++ {
				a, err := w.expr(c.NamedChild(j))
				if err != nil {
					return nil, err
				}
				args = append(args, a)
			}
		}
		return hir.Call{Func: w.text(w.field(n, "callee")), Args: args}, nil
	}
	return nil, fmt.Errorf("unexpected expression node %q", w.typ(n))
}
