// Package parse turns .sel source into the high-level material model (hir).
// It loads the Selena tree-sitter language from the embedded pre-generated parse
// table (grammar.bin) via taproot/walk, parses the source, and walks the tree
// into a hir.Material. Loading the blob keeps this package grammar-free (no
// grammargen / grammars registry); the grammar DSL used to regenerate the blob
// lives in the selena/grammar subpackage. This is the front-end: with it,
// materials are authored as .sel files rather than hand-built HIR.
package parse

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	gts "github.com/odvcencio/gotreesitter"
	walk "github.com/odvcencio/gotreesitter/taproot/walk"

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

// language loads (and caches) the Selena tree-sitter language from the embedded
// pre-generated parse table. Grammar-free: no grammargen fallback.
func language() (*gts.Language, error) {
	return walk.LanguageFromBlob("selena", grammarBlob)
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
	w := &walker{Walker: walk.NewWalker(l, src)}
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
	w := &walker{Walker: walk.NewWalker(l, src)}
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
	*walk.Walker
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
	if err := bracketArraySyntaxError(w.Src); err != nil {
		return err
	}
	n := firstError(root)
	if expected, span, ok := expectedFromSource(w.Src); ok {
		return &Error{Message: "syntax error in .sel source; expected " + expected, Span: span}
	}
	return &Error{Message: syntaxMessage(w, n), Span: w.span(n)}
}

// bracketArrayParamLine matches a param declaration written with the C-style
// array suffix `type[N]`. A newer grammar release stopped silently dropping
// this suffix (see checkDroppedTokens) and instead cascades it into a hard
// parse error covering the rest of the material. Checked here, ahead of the
// generic syntax-error heuristics, so the fix is still the first thing an
// author sees.
var bracketArrayParamLine = regexp.MustCompile(`^param\s+\S+\s*:\s*(\S+?)\s*(\[\s*[0-9]+\s*\])`)

// bracketArraySyntaxError scans src line by line for the C-style array param
// spelling and, when found, reports it with the same correction as
// droppedTokenMessage. It returns nil when no line matches.
func bracketArraySyntaxError(src []byte) *Error {
	offset := 0
	lineNo := 1
	for _, line := range strings.SplitAfter(string(src), "\n") {
		body := strings.TrimRight(line, "\r\n")
		trimmed := strings.TrimSpace(body)
		indent := firstNonSpace(body)
		if m := bracketArrayParamLine.FindStringSubmatch(trimmed); m != nil {
			elem, bracket := m[1], m[2]
			pos := strings.Index(trimmed, bracket)
			return &Error{
				Message: droppedTokenMessage(bracket, elem),
				Span:    spanAt(lineNo, offset+indent+pos, indent+pos+1, len(bracket)),
			}
		}
		offset += len(line)
		lineNo++
	}
	return nil
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
			return "`param`, `surface`, `vertex`, `varying`, `feedback`, `state`, or `}`"
		case "vertex":
			return "`->`, a return type, and a vertex body"
		case "varying_decl":
			return "`:` after the varying name, then a type"
		case "feedback":
			return "`->`, a return type, and a feedback body"
		case "statefield":
			return "a statefield name after `state`"
		case "param":
			return "`:` after the parameter name, a type, optional `= default`, or the next member"
		case "surface":
			return "`->`, a return type, and a surface body"
		case "fn_decl":
			return "`(`, parameters, `->`, a return type, and a function body"
		case "fn_param":
			return "`:` after the function parameter name"
		case "block":
			return "`let`, `var`, `return`, `if`, `for`, or `}`"
		case "statement":
			return "`let`, `var`, `return`, `if`, `for`, or an assignment"
		case "let_stmt":
			return "`=` and an expression"
		case "var_stmt":
			return "`=` and an expression"
		case "assign_stmt":
			return "`=` and an expression"
		case "if_stmt":
			return "`(`, a condition, `)`, and a `{ ... }` block"
		case "for_stmt":
			return "`(var`, loop variable, condition, update, `)`, and a `{ ... }` block"
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
	for _, prefix := range []string{"material ", "fn ", "param ", "surface", "let ", "return ", "var ", "if ", "if(", "for ", "for("} {
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
	if k := w.Field(n, "kind"); k != nil {
		kindVal := w.Text(k)
		switch kindVal {
		case "points":
			m.Kind = hir.KindPoints
		case "post":
			m.Kind = hir.KindPost
		case "feedback":
			m.Kind = hir.KindFeedback
		case "mesh":
			m.Kind = hir.KindMesh
		default:
			return m, fmt.Errorf("material %q: unknown kind %q (expected mesh, points, post, or feedback)", m.Name, kindVal)
		}
	}
	if err := w.checkDroppedTokens(n); err != nil {
		return m, err
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
		case "param_array":
			p, err := w.paramArray(inner)
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
		case "feedback":
			sf, err := w.feedbackFn(inner)
			if err != nil {
				return m, err
			}
			m.Surface = sf
		case "statefield":
			m.States = append(m.States, hir.StateField{
				Name: w.Text(w.Field(inner, "name")),
				Span: w.span(inner),
			})
		case "vertex":
			vf, err := w.vertexFn(inner)
			if err != nil {
				return m, err
			}
			m.Vertex = &vf
		case "varying_decl":
			m.Varyings = append(m.Varyings, hir.Varying{
				Name: w.Text(w.Field(inner, "name")),
				Type: hir.Type(w.Text(w.Field(inner, "type"))),
				Span: w.span(inner),
			})
		case "context_block":
			cfs, err := w.contextBlock(inner)
			if err != nil {
				return m, err
			}
			m.Context = append(m.Context, cfs...)
		}
	}
	if m.Surface.Geo == "" {
		if m.Kind == hir.KindFeedback {
			return m, fmt.Errorf("material %q has no feedback entry", m.Name)
		}
		return m, fmt.Errorf("material %q has no surface", m.Name)
	}
	return m, nil
}

// feedbackFn parses a `feedback(cell) -> vec4 { ... }` entry. It mirrors fn but
// reads the "cell" field (the current-cell handle binding) instead of "geo".
func (w *walker) feedbackFn(n *gts.Node) (hir.Func, error) {
	f := hir.Func{Geo: w.Text(w.Field(n, "cell")), Span: w.span(n)}
	body := w.Field(n, "body")
	stmts, result, _, err := w.blockBody(body)
	if err != nil {
		return f, err
	}
	f.Body = stmts
	f.Result = result
	if f.Result == nil {
		return f, fmt.Errorf("feedback has no return")
	}
	return f, nil
}

// vertexFn parses a `vertex() -> vec4 { ... }` or `vertex(geo) -> vec4 { ... }`
// entry. It mirrors fn but the geometry binding is optional: when the parens are
// empty, Geo is "" (procedural geometry from vertexIndex, no attributes). The
// result expression is the clip-space position. B4.
func (w *walker) vertexFn(n *gts.Node) (hir.Func, error) {
	f := hir.Func{Span: w.span(n)}
	if g := w.Field(n, "geo"); g != nil {
		f.Geo = w.Text(g)
	}
	body := w.Field(n, "body")
	stmts, result, _, err := w.blockBody(body)
	if err != nil {
		return f, err
	}
	f.Body = stmts
	f.Result = result
	if f.Result == nil {
		return f, fmt.Errorf("vertex() has no return (must return the clip-space position)")
	}
	return f, nil
}

// arrayBracketSuffix matches the C-style fixed-size array spelling `[N]`,
// optionally surrounded by spaces: `param rects : vec4[8]`.
var arrayBracketSuffix = regexp.MustCompile(`^\[\s*([0-9]+)\s*\]$`)

// checkDroppedTokens reports source inside a material body that the parser
// consumed without putting it in the tree.
//
// The Selena parse table silently skips a `[N]` suffix on a param type: the
// material node reports no error, the member node ends at the type identifier,
// and `param rects : vec4[8]` lowers as a plain scalar vec4. The first sign of
// trouble then lands far away, at `rects[i]`, as SEL2001 "rects is not a local
// array or uniform array param" — a diagnosis pointing at correct code.
//
// Rather than trust the tree to be complete, this walks the gaps between the
// material's children. Anything in a gap other than whitespace or a comment was
// dropped, so it is reported at its own position with the array spelling
// suggested when the dropped text is a `[N]` suffix.
func (w *walker) checkDroppedTokens(n *gts.Node) error {
	count := n.ChildCount()
	if count == 0 {
		return nil
	}
	prevEnd := int(n.Child(0).EndByte())
	prevText := strings.TrimSpace(w.Text(n.Child(0)))
	for i := 1; i < count; i++ {
		c := n.Child(i)
		if c == nil {
			continue
		}
		start := int(c.StartByte())
		if start > prevEnd && prevEnd >= 0 && start <= len(w.Src) {
			gap := strings.TrimSpace(stripComments(string(w.Src[prevEnd:start])))
			if gap != "" {
				return &Error{Message: droppedTokenMessage(gap, prevText), Span: w.byteSpan(prevEnd, start)}
			}
		}
		prevEnd = int(c.EndByte())
		prevText = strings.TrimSpace(w.Text(c))
	}
	return nil
}

// droppedTokenMessage explains dropped source, naming the correct spelling when
// the dropped text is the C-style array suffix.
func droppedTokenMessage(gap, prev string) string {
	if m := arrayBracketSuffix.FindStringSubmatch(gap); m != nil {
		elem := prev
		if idx := strings.LastIndex(prev, " "); idx >= 0 {
			elem = strings.TrimSpace(prev[idx+1:])
		}
		if elem == "" {
			elem = "T"
		}
		return fmt.Sprintf(
			"unexpected %q: a fixed-size array param is spelled `array<%s, %s>`, not `%s[%s]`",
			gap, elem, m[1], elem, m[1],
		)
	}
	return fmt.Sprintf("unexpected %q in material body", gap)
}

// stripComments removes `//` line comments from s so a gap that holds only a
// comment is not mistaken for dropped source. Comments are lexer extras and
// never appear in the tree.
func stripComments(s string) string {
	var b strings.Builder
	for _, line := range strings.Split(s, "\n") {
		if idx := strings.Index(line, "//"); idx >= 0 {
			line = line[:idx]
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	return b.String()
}

// byteSpan builds a Span for the [start, end) byte range of the source.
func (w *walker) byteSpan(start, end int) hir.Span {
	return hir.Span{
		Start: w.position(start),
		End:   w.position(end),
	}
}

func (w *walker) position(offset int) hir.Position {
	line, col := 1, 1
	for i := 0; i < offset && i < len(w.Src); i++ {
		if w.Src[i] == '\n' {
			line++
			col = 1
			continue
		}
		col++
	}
	return hir.Position{Offset: offset, Line: line, Column: col}
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

// paramArray parses a `param name : array<T, N>` member (B3.2). The array_type
// sub-node carries the element type identifier and the size number literal.
func (w *walker) paramArray(n *gts.Node) (hir.Param, error) {
	typNode := w.Field(n, "typ")
	elemText := w.Text(w.Field(typNode, "elem"))
	sizeText := w.Text(w.Field(typNode, "size"))
	sz, err := strconv.Atoi(sizeText)
	if err != nil || sz <= 0 {
		return hir.Param{}, &Error{
			Message: fmt.Sprintf("array size must be a positive integer, got %q", sizeText),
			Span:    w.span(n),
		}
	}
	return hir.Param{
		Name:      w.Text(w.Field(n, "name")),
		Type:      hir.Type(elemText),
		IsArray:   true,
		ArraySize: sz,
		Span:      w.span(n),
	}, nil
}

// contextBlock parses a `context { field... }` member (the context-uniform
// design §3.1). Each repeated context_member child wraps one of two shapes —
// context_field (scalar/vector/matrix) or context_field_array (fixed-size
// array, B3.2-style) — mirroring how `member` wraps param vs param_array.
// Fields are collected into hir.ContextField in declaration order.
func (w *walker) contextBlock(n *gts.Node) ([]hir.ContextField, error) {
	var fields []hir.ContextField
	for i := 0; i < n.NamedChildCount(); i++ {
		c := n.NamedChild(i)
		if w.Type(c) != "context_member" {
			continue
		}
		inner := c.NamedChild(0)
		switch w.Type(inner) {
		case "context_field":
			cf, err := w.contextField(inner)
			if err != nil {
				return nil, err
			}
			fields = append(fields, cf)
		case "context_field_array":
			cf, err := w.contextFieldArray(inner)
			if err != nil {
				return nil, err
			}
			fields = append(fields, cf)
		}
	}
	return fields, nil
}

// contextField parses one scalar/vector/matrix `name : type [= default]`
// entry inside a context block. Structurally mirrors param (:422).
func (w *walker) contextField(n *gts.Node) (hir.ContextField, error) {
	cf := hir.ContextField{
		Name: w.Text(w.Field(n, "name")),
		Type: hir.Type(w.Text(w.Field(n, "type"))),
		Span: w.span(n),
	}
	if d := w.Field(n, "default"); d != nil {
		e, err := w.expr(d)
		if err != nil {
			return hir.ContextField{}, err
		}
		cf.Default = e
	}
	return cf, nil
}

// contextFieldArray parses a `name : array<T, N>` entry inside a context
// block. Structurally mirrors paramArray (:440); the array type sub-node
// (context_array_type) carries the element type identifier and size literal.
func (w *walker) contextFieldArray(n *gts.Node) (hir.ContextField, error) {
	typNode := w.Field(n, "typ")
	elemText := w.Text(w.Field(typNode, "elem"))
	sizeText := w.Text(w.Field(typNode, "size"))
	sz, err := strconv.Atoi(sizeText)
	if err != nil || sz <= 0 {
		return hir.ContextField{}, &Error{
			Message: fmt.Sprintf("array size must be a positive integer, got %q", sizeText),
			Span:    w.span(n),
		}
	}
	return hir.ContextField{
		Name:      w.Text(w.Field(n, "name")),
		IsArray:   true,
		ArraySize: sz,
		Type:      hir.Type(elemText),
		Span:      w.span(n),
	}, nil
}

func (w *walker) fn(n *gts.Node) (hir.Func, error) {
	f := hir.Func{Geo: w.Text(w.Field(n, "geo")), Span: w.span(n)}
	body := w.Field(n, "body")
	stmts, result, _, err := w.blockBody(body)
	if err != nil {
		return f, err
	}
	f.Body = stmts
	f.Result = result
	if f.Result == nil {
		return f, fmt.Errorf("surface has no return")
	}
	return f, nil
}

// blockBody parses the statements inside a block node into (HIR body stmts, return expr).
// return_stmt sets the result expression (and its span); all other statement
// kinds go into stmts. Sub-blocks (inside if/for) call subBlock, which wraps
// blockBody and turns a trailing return into a hir.Return statement appended
// to the block's own stmts (early returns, legal at any nesting depth).
//
// A return_stmt must be the last statement in ITS block, at every nesting
// depth: nothing may follow it, in the top-level body or inside an if/for.
// Without this check a later statement (including a second return) would
// silently overwrite an earlier return's result, or hoist unreachable code
// into the executed body — the exact defect this check exists to prevent.
func (w *walker) blockBody(block *gts.Node) (stmts []hir.Stmt, result hir.Expr, resultSpan hir.Span, err error) {
	for i := 0; i < block.NamedChildCount(); i++ {
		st := block.NamedChild(i)
		if w.Type(st) != "statement" {
			continue
		}
		s := st.NamedChild(0)
		if result != nil {
			return nil, nil, hir.Span{}, &Error{
				Message: "unreachable statement after return (a return must be the last statement in its block)",
				Span:    w.span(s),
			}
		}
		switch w.Type(s) {
		case "let_stmt":
			e, err := w.expr(w.Field(s, "value"))
			if err != nil {
				return nil, nil, hir.Span{}, err
			}
			stmts = append(stmts, hir.Let{Name: w.Text(w.Field(s, "name")), Value: e, Span: w.span(s)})
		case "var_stmt":
			e, err := w.expr(w.Field(s, "value"))
			if err != nil {
				return nil, nil, hir.Span{}, err
			}
			stmts = append(stmts, hir.VarDecl{Name: w.Text(w.Field(s, "name")), Value: e, Span: w.span(s)})
		case "var_typed_stmt":
			typNode := w.Field(s, "typ")
			elemIdent := w.Text(w.Field(typNode, "elem"))
			sizeText := w.Text(w.Field(typNode, "size"))
			sz, err := strconv.Atoi(sizeText)
			if err != nil || sz <= 0 {
				return nil, nil, hir.Span{}, &Error{Message: fmt.Sprintf("array size must be a positive integer, got %q", sizeText), Span: w.span(s)}
			}
			stmts = append(stmts, hir.VarArrayDecl{
				Name:     w.Text(w.Field(s, "name")),
				ElemType: hir.Type(elemIdent),
				Size:     sz,
				Span:     w.span(s),
			})
		case "assign_stmt":
			e, err := w.expr(w.Field(s, "value"))
			if err != nil {
				return nil, nil, hir.Span{}, err
			}
			stmts = append(stmts, hir.Assign{Name: w.Text(w.Field(s, "name")), Value: e, Span: w.span(s)})
		case "index_assign_stmt":
			idx, err := w.expr(w.Field(s, "index"))
			if err != nil {
				return nil, nil, hir.Span{}, err
			}
			val, err := w.expr(w.Field(s, "value"))
			if err != nil {
				return nil, nil, hir.Span{}, err
			}
			stmts = append(stmts, hir.IndexAssign{
				Name:  w.Text(w.Field(s, "name")),
				Index: idx,
				Value: val,
				Span:  w.span(s),
			})
		case "if_stmt":
			stmt, err := w.ifStmt(s)
			if err != nil {
				return nil, nil, hir.Span{}, err
			}
			stmts = append(stmts, stmt)
		case "for_stmt":
			stmt, err := w.forStmt(s)
			if err != nil {
				return nil, nil, hir.Span{}, err
			}
			stmts = append(stmts, stmt)
		case "return_stmt":
			e, err := w.expr(w.Field(s, "value"))
			if err != nil {
				return nil, nil, hir.Span{}, err
			}
			result = e
			resultSpan = w.span(s)
		case "discard_stmt":
			stmts = append(stmts, hir.Discard{Span: w.span(s)})
		case "break_stmt":
			stmts = append(stmts, hir.Break{Span: w.span(s)})
		}
	}
	return stmts, result, resultSpan, nil
}

// subBlock parses an if/for body block. A trailing return_stmt becomes a
// hir.Return statement appended to the block's own stmts: an early return,
// legal at any nesting depth. blockBody's own "return must be last" check
// still rejects any statement after that return, at this block's level.
func (w *walker) subBlock(block *gts.Node) ([]hir.Stmt, error) {
	stmts, result, resultSpan, err := w.blockBody(block)
	if err != nil {
		return nil, err
	}
	if result != nil {
		stmts = append(stmts, hir.Return{Value: result, Span: resultSpan})
	}
	return stmts, nil
}

func (w *walker) ifStmt(n *gts.Node) (hir.If, error) {
	cond, err := w.expr(w.Field(n, "cond"))
	if err != nil {
		return hir.If{}, err
	}
	then, err := w.subBlock(w.Field(n, "then"))
	if err != nil {
		return hir.If{}, err
	}
	var els []hir.Stmt
	if altBlock := w.Field(n, "alt"); altBlock != nil {
		els, err = w.subBlock(altBlock)
		if err != nil {
			return hir.If{}, err
		}
	}
	return hir.If{Cond: cond, Then: then, Else: els, Span: w.span(n)}, nil
}

func (w *walker) forStmt(n *gts.Node) (hir.For, error) {
	initVal, err := w.expr(w.Field(n, "init_val"))
	if err != nil {
		return hir.For{}, err
	}
	cond, err := w.expr(w.Field(n, "cond"))
	if err != nil {
		return hir.For{}, err
	}
	updateVal, err := w.expr(w.Field(n, "update_val"))
	if err != nil {
		return hir.For{}, err
	}
	body, err := w.subBlock(w.Field(n, "body"))
	if err != nil {
		return hir.For{}, err
	}
	return hir.For{
		InitName:  w.Text(w.Field(n, "init_name")),
		InitValue: initVal,
		Cond:      cond,
		PostName:  w.Text(w.Field(n, "update_name")),
		PostValue: updateVal,
		Body:      body,
		Span:      w.span(n),
	}, nil
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
	case "int_literal":
		// Strip trailing 'i' suffix then parse as int64.
		text := w.Text(n)
		v, err := strconv.ParseInt(text[:len(text)-1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("bad int literal %q: %w", text, err)
		}
		return hir.IntLit{Value: v, Span: w.span(n)}, nil
	case "uint_literal":
		// Strip trailing 'u' suffix then parse as uint64.
		text := w.Text(n)
		v, err := strconv.ParseUint(text[:len(text)-1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("bad uint literal %q: %w", text, err)
		}
		return hir.UintLit{Value: v, Span: w.span(n)}, nil
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
	case "unary_expression", "unary_not_expression":
		e, err := w.expr(w.Field(n, "operand"))
		if err != nil {
			return nil, err
		}
		op := w.Text(w.Field(n, "operator"))
		return hir.Unary{Op: op, E: e, Span: w.span(n)}, nil
	case "conditional_expression":
		cond, err := w.expr(w.Field(n, "cond"))
		if err != nil {
			return nil, err
		}
		then, err := w.expr(w.Field(n, "then"))
		if err != nil {
			return nil, err
		}
		alt, err := w.expr(w.Field(n, "alt"))
		if err != nil {
			return nil, err
		}
		return hir.Conditional{Cond: cond, Then: then, Alt: alt, Span: w.span(n)}, nil
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
	case "index_expression":
		obj, err := w.expr(w.Field(n, "object"))
		if err != nil {
			return nil, err
		}
		idx, err := w.expr(w.Field(n, "index"))
		if err != nil {
			return nil, err
		}
		return hir.IndexExpr{Arr: obj, Index: idx, Span: w.span(n)}, nil
	}
	return nil, fmt.Errorf("unexpected expression node %q", w.Type(n))
}
