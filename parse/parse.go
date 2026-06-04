// Package parse turns .sel source into the high-level material model (hir).
// It generates the Selena tree-sitter language from grammar.SelenaGrammar via
// grammargen (the same engine as .gsx), parses the source, and walks the tree
// into a hir.Material. This is the front-end: with it, materials are authored
// as .sel files rather than hand-built HIR.
package parse

import (
	"fmt"
	"strconv"
	"strings"

	gts "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/taproot"

	"m31labs.dev/selena/grammar"
	"m31labs.dev/selena/hir"
)

// Error is a parser error that can be anchored back to source.
type Error struct {
	Message string
	Span    hir.Span
}

func (e *Error) Error() string {
	if !e.Span.IsZero() {
		return fmt.Sprintf("%d:%d: %s", e.Span.Start.Line, e.Span.Start.Column, e.Message)
	}
	return e.Message
}

// language generates (and caches) the Selena tree-sitter language.
func language() (*gts.Language, error) {
	return taproot.LanguageFromBlob("selena", grammarBlob, grammar.SelenaGrammar)
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
	w := &walker{Walker: taproot.NewWalker(l, src)}
	if root.HasError() {
		return hir.Material{}, syntaxError(w, root)
	}
	for i := 0; i < root.NamedChildCount(); i++ {
		if c := root.NamedChild(i); w.Type(c) == "material" {
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
	w := &walker{Walker: taproot.NewWalker(l, src)}
	if root.HasError() {
		return hir.Program{}, syntaxError(w, root)
	}
	var p hir.Program
	for i := 0; i < root.NamedChildCount(); i++ {
		c := root.NamedChild(i)
		switch w.Type(c) {
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
	*taproot.Walker
}

func (w *walker) span(n *gts.Node) hir.Span {
	if n == nil {
		return hir.Span{}
	}
	startLine, startCol := w.Pos(n)
	end := n.EndPoint()
	return hir.Span{
		Start: hir.Position{Offset: int(n.StartByte()), Line: startLine, Column: startCol},
		End:   hir.Position{Offset: int(n.EndByte()), Line: int(end.Row) + 1, Column: int(end.Column) + 1},
	}
}

func firstError(n *gts.Node) *gts.Node {
	if n == nil {
		return nil
	}
	if n.IsMissing() {
		return n
	}
	for i := 0; i < n.ChildCount(); i++ {
		c := n.Child(i)
		if c == nil {
			continue
		}
		if c.HasError() || c.IsError() || c.IsMissing() {
			return firstError(c)
		}
	}
	if n.IsError() || n.HasError() {
		return n
	}
	return nil
}

func syntaxError(w *walker, root *gts.Node) *Error {
	n := firstError(root)
	if expected, span, ok := expectedFromSource(w.Src); ok {
		return &Error{Message: "syntax error in .sel source; expected " + expected, Span: span}
	}
	return &Error{Message: syntaxMessage(w, n), Span: w.span(n)}
}

func syntaxMessage(w *walker, n *gts.Node) string {
	const prefix = "syntax error in .sel source"
	if n == nil {
		return prefix
	}
	if n.IsMissing() {
		return prefix + "; expected " + readableExpected(w.Type(n))
	}
	if expected := expectedFromContext(w, n); expected != "" {
		return prefix + "; expected " + expected
	}
	if near := strings.TrimSpace(w.Text(n)); near != "" {
		return fmt.Sprintf("%s near %q", prefix, near)
	}
	return prefix
}

func expectedFromContext(w *walker, n *gts.Node) string {
	for cur := n; cur != nil; cur = cur.Parent() {
		switch w.Type(cur) {
		case "source_file":
			return "a material or fn declaration"
		case "material":
			return "`param`, `surface`, or `}`"
		case "member":
			return "`param` or `surface`"
		case "param":
			return "`:` after the parameter name, a type, optional `= default`, or the next member"
		case "surface":
			return "`->`, a return type, and a surface body"
		case "fn_decl":
			return "`(`, parameters, `->`, a return type, and a function body"
		case "fn_param":
			return "`:` after the function parameter name"
		case "block":
			return "`let`, `return`, or `}`"
		case "statement":
			return "`let` or `return`"
		case "let_stmt":
			return "`=` and an expression"
		case "return_stmt":
			return "a return expression"
		case "call", "arguments":
			return "an argument expression or `)`"
		case "super_call":
			return "`super.<method>(...)`"
		case "member_expression":
			return "`.` and a field name"
		case "binary_expression":
			return "a right-hand expression"
		case "paren_expression":
			return "an expression and `)`"
		}
	}
	return ""
}

func readableExpected(token string) string {
	switch token {
	case "{", "}", "(", ")", ":", "=", "->", ",", ".":
		return "`" + token + "`"
	case "identifier":
		return "an identifier"
	case "expression":
		return "an expression"
	case "block":
		return "a `{ ... }` block"
	default:
		return "`" + token + "`"
	}
}

func expectedFromSource(src []byte) (string, hir.Span, bool) {
	text := string(src)
	offset := 0
	lineNo := 1
	openBraces := 0
	for _, line := range strings.SplitAfter(text, "\n") {
		body := strings.TrimRight(line, "\r\n")
		trimmed := strings.TrimSpace(body)
		indent := firstNonSpace(body)
		switch {
		case strings.HasPrefix(trimmed, "param ") && !strings.Contains(trimmed, ":"):
			return "`:` after the parameter name", spanAt(lineNo, offset+indent, indent+1, len(trimmed)), true
		case strings.HasPrefix(trimmed, "surface") && !strings.Contains(trimmed, "->"):
			return "`->` before the surface return type", spanAt(lineNo, offset+indent, indent+1, len(trimmed)), true
		case looksLikeBareStatement(trimmed):
			return "`let` or `return`", spanAt(lineNo, offset+indent, indent+1, len(trimmed)), true
		}
		for _, r := range body {
			switch r {
			case '{':
				openBraces++
			case '}':
				if openBraces > 0 {
					openBraces--
				}
			}
		}
		offset += len(line)
		lineNo++
	}
	if openBraces > 0 {
		return "`}`", spanAt(lineNo-1, len(src), 1, 1), true
	}
	return "", hir.Span{}, false
}

func firstNonSpace(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] != ' ' && s[i] != '\t' {
			return i
		}
	}
	return 0
}

func looksLikeBareStatement(s string) bool {
	if s == "" || !isIdentStart(s[0]) {
		return false
	}
	for _, prefix := range []string{"material ", "fn ", "param ", "surface", "let ", "return "} {
		if strings.HasPrefix(s, prefix) {
			return false
		}
	}
	return true
}

func isIdentStart(b byte) bool {
	return b == '_' || (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
}

func spanAt(lineNo, offset, column, width int) hir.Span {
	if width < 1 {
		width = 1
	}
	return hir.Span{
		Start: hir.Position{Offset: offset, Line: lineNo, Column: column},
		End:   hir.Position{Offset: offset + width, Line: lineNo, Column: column + width},
	}
}

func (w *walker) material(n *gts.Node) (hir.Material, error) {
	m := hir.Material{Name: w.Text(w.Field(n, "name")), Span: w.span(n)}
	if p := w.Field(n, "parent"); p != nil {
		m.Extends = w.Text(p)
	}
	for i := 0; i < n.NamedChildCount(); i++ {
		c := n.NamedChild(i)
		if w.Type(c) != "member" {
			continue
		}
		inner := c.NamedChild(0)
		switch w.Type(inner) {
		case "param":
			p, err := w.param(inner)
			if err != nil {
				return m, err
			}
			m.Params = append(m.Params, p)
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

func (w *walker) param(n *gts.Node) (hir.Param, error) {
	p := hir.Param{
		Name: w.Text(w.Field(n, "name")),
		Type: hir.Type(w.Text(w.Field(n, "type"))),
		Span: w.span(n),
	}
	if d := w.Field(n, "default"); d != nil {
		e, err := w.expr(d)
		if err != nil {
			return p, err
		}
		p.Default = e
	}
	return p, nil
}

func (w *walker) fn(n *gts.Node) (hir.Func, error) {
	f := hir.Func{Geo: w.Text(w.Field(n, "geo")), Span: w.span(n)}
	body := w.Field(n, "body")
	for i := 0; i < body.NamedChildCount(); i++ {
		st := body.NamedChild(i)
		if w.Type(st) != "statement" {
			continue
		}
		s := st.NamedChild(0)
		switch w.Type(s) {
		case "let_stmt":
			e, err := w.expr(w.Field(s, "value"))
			if err != nil {
				return f, err
			}
			f.Body = append(f.Body, hir.Let{Name: w.Text(w.Field(s, "name")), Value: e, Span: w.span(s)})
		case "return_stmt":
			e, err := w.expr(w.Field(s, "value"))
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
		Name:    w.Text(w.Field(n, "name")),
		Span:    w.span(n),
		Returns: hir.Type(w.Text(w.Field(n, "returns"))),
	}
	for i := 0; i < n.NamedChildCount(); i++ {
		c := n.NamedChild(i)
		if w.Type(c) != "fn_params" {
			continue
		}
		for j := 0; j < c.NamedChildCount(); j++ {
			p := c.NamedChild(j)
			if w.Type(p) != "fn_param" {
				continue
			}
			fd.Params = append(fd.Params, hir.Param{
				Name: w.Text(w.Field(p, "name")),
				Type: hir.Type(w.Text(w.Field(p, "type"))),
				Span: w.span(p),
			})
		}
	}
	body := w.Field(n, "body")
	for i := 0; i < body.NamedChildCount(); i++ {
		st := body.NamedChild(i)
		if w.Type(st) != "statement" {
			continue
		}
		s := st.NamedChild(0)
		switch w.Type(s) {
		case "let_stmt":
			e, err := w.expr(w.Field(s, "value"))
			if err != nil {
				return fd, err
			}
			fd.Body = append(fd.Body, hir.Let{Name: w.Text(w.Field(s, "name")), Value: e, Span: w.span(s)})
		case "return_stmt":
			e, err := w.expr(w.Field(s, "value"))
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
	switch w.Type(n) {
	case "number":
		v, err := strconv.ParseFloat(w.Text(n), 64)
		if err != nil {
			return nil, fmt.Errorf("bad number %q: %w", w.Text(n), err)
		}
		return hir.Lit{Value: v, Span: w.span(n)}, nil
	case "identifier":
		return hir.Ref{Name: w.Text(n), Span: w.span(n)}, nil
	case "paren_expression":
		return w.expr(n.NamedChild(0))
	case "member_expression":
		obj, err := w.expr(w.Field(n, "object"))
		if err != nil {
			return nil, err
		}
		return hir.Member{E: obj, Field: w.Text(w.Field(n, "field")), Span: w.span(n)}, nil
	case "binary_expression":
		l, err := w.expr(w.Field(n, "left"))
		if err != nil {
			return nil, err
		}
		r, err := w.expr(w.Field(n, "right"))
		if err != nil {
			return nil, err
		}
		return hir.Binary{Op: w.Text(w.Field(n, "operator")), L: l, R: r, Span: w.span(n)}, nil
	case "unary_expression":
		e, err := w.expr(w.Field(n, "operand"))
		if err != nil {
			return nil, err
		}
		return hir.Unary{Op: "-", E: e, Span: w.span(n)}, nil
	case "call":
		var args []hir.Expr
		for i := 0; i < n.NamedChildCount(); i++ {
			c := n.NamedChild(i)
			if w.Type(c) != "arguments" {
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
		return hir.Call{Func: w.Text(w.Field(n, "callee")), Args: args, Span: w.span(n)}, nil
	case "super_call":
		var args []hir.Expr
		for i := 0; i < n.NamedChildCount(); i++ {
			c := n.NamedChild(i)
			if w.Type(c) != "arguments" {
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
		return hir.SuperCall{Method: w.Text(w.Field(n, "method")), Args: args, Span: w.span(n)}, nil
	}
	return nil, fmt.Errorf("unexpected expression node %q", w.Type(n))
}
