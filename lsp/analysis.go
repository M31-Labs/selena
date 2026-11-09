package lsp

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"m31labs.dev/selena"
	"m31labs.dev/selena/hir"
	"m31labs.dev/selena/ir"
	"m31labs.dev/selena/materiallib"
	"m31labs.dev/selena/parse"
)

// Options configures analysis.
type Options struct {
	// Modules are linked into every document, like build.selena.modules in
	// gosx.config.json. Modules a document calls into are linked anyway.
	Modules []materiallib.Module
}

// libFunc is one material library helper.
type libFunc struct {
	Module materiallib.Module
	Decl   hir.FuncDecl
	Doc    string
	Source []byte
}

var (
	libOnce  sync.Once
	libIndex map[string]libFunc
)

// libraryIndex parses every bundled module once and indexes its functions by
// name. A module that fails to parse is skipped; the compiler reports it.
func libraryIndex() map[string]libFunc {
	libOnce.Do(func() {
		libIndex = map[string]libFunc{}
		for _, module := range materiallib.Modules() {
			src, err := materiallib.Source(module)
			if err != nil {
				continue
			}
			program, err := parse.Program(src)
			if err != nil {
				continue
			}
			for _, fn := range program.Funcs {
				if _, dup := libIndex[fn.Name]; dup {
					continue
				}
				libIndex[fn.Name] = libFunc{Module: module, Decl: fn, Doc: leadingComment(src, fn.Span.Start.Offset), Source: src}
			}
		}
	})
	return libIndex
}

// analysis is everything the features need about one version of a document.
type analysis struct {
	src      []byte
	program  hir.Program
	parsed   bool
	modules  []materiallib.Module
	results  map[string]selena.Result
	diags    []Diagnostic
	parseErr error
}

func analyze(src []byte, opts Options) *analysis {
	a := &analysis{src: src, results: map[string]selena.Result{}}
	program, err := parse.Program(src)
	if err != nil {
		a.parseErr = err
		a.addError(err, hir.Span{})
		// Declarations stay useful for hover and completion while a line is
		// half typed: parse a copy with the broken lines blanked.
		a.program, _ = repairedProgram(src, -1)
		return a
	}
	a.program, a.parsed = program, true
	a.modules = modulesFor(src, program, opts.Modules)
	linked, err := materiallib.Link(program, a.modules...)
	if err != nil {
		a.addError(err, hir.Span{})
		return a
	}
	for _, m := range program.Materials {
		res, err := selena.CompileProgram(linked, selena.CompileOptions{Material: m.Name})
		if err != nil {
			a.addError(err, nameSpan(src, m.Span, m.Name))
			continue
		}
		a.results[m.Name] = res
	}
	return a
}

// modulesFor links the configured modules plus every module that defines a
// function the document calls and does not declare itself.
func modulesFor(src []byte, program hir.Program, configured []materiallib.Module) []materiallib.Module {
	out := slices.Clone(configured)
	own := map[string]bool{}
	for _, fn := range program.Funcs {
		own[fn.Name] = true
	}
	index := libraryIndex()
	for _, name := range calledNames(src) {
		if own[name] {
			continue
		}
		if lib, ok := index[name]; ok && !slices.Contains(out, lib.Module) {
			out = append(out, lib.Module)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// calledNames lists identifiers followed by "(" outside comments.
func calledNames(src []byte) []string {
	text := stripComments(src)
	seen := map[string]bool{}
	var out []string
	for i := 0; i < len(text); {
		if !isIdentByte(text[i]) || i > 0 && isIdentByte(text[i-1]) {
			i++
			continue
		}
		j := i
		for j < len(text) && isIdentByte(text[j]) {
			j++
		}
		if nextNonSpace(text, j) == '(' {
			name := string(text[i:j])
			if !seen[name] {
				seen[name] = true
				out = append(out, name)
			}
		}
		i = j
	}
	return out
}

func (a *analysis) addError(err error, fallback hir.Span) {
	var ce *selena.CompileError
	if errors.As(err, &ce) && len(ce.Diagnostics) > 0 {
		for _, d := range ce.Diagnostics {
			span := d.Range
			if span.IsZero() || span.End.Offset > len(a.src) {
				span = fallback
			}
			msg := d.Message
			if d.Hint != "" {
				msg += "\nhint: " + d.Hint
			}
			a.add(Diagnostic{Range: spanRange(a.src, span), Severity: SeverityError, Code: d.Code, Source: "selena", Message: msg})
		}
		return
	}
	var pe *parse.Error
	if errors.As(err, &pe) {
		a.add(Diagnostic{Range: spanRange(a.src, pe.Span), Severity: SeverityError, Code: "SEL0001", Source: "selena", Message: pe.Message})
		return
	}
	a.add(Diagnostic{Range: spanRange(a.src, fallback), Severity: SeverityError, Source: "selena", Message: err.Error()})
}

// add appends d unless the same message already sits at the same range, as
// it does when a child material inherits a broken parent.
func (a *analysis) add(d Diagnostic) {
	for _, have := range a.diags {
		if have.Range == d.Range && have.Code == d.Code && have.Message == d.Message {
			return
		}
	}
	a.diags = append(a.diags, d)
}

// nameSpan locates name as a whole word at or after the start of decl.
func nameSpan(src []byte, decl hir.Span, name string) hir.Span {
	start := decl.Start.Offset
	if decl.IsZero() || start >= len(src) || name == "" {
		return decl
	}
	text := src[start:]
	for i := 0; i+len(name) <= len(text); i++ {
		if string(text[i:i+len(name)]) != name {
			continue
		}
		if i > 0 && isIdentByte(text[i-1]) || i+len(name) < len(text) && isIdentByte(text[i+len(name)]) {
			continue
		}
		return hir.Span{Start: hir.Position{Offset: start + i, Line: 1}, End: hir.Position{Offset: start + i + len(name), Line: 1}}
	}
	return decl
}

// leadingComment returns the `//` comment block that ends on the line above
// offset, without the slashes.
func leadingComment(src []byte, offset int) string {
	lineStart := offset
	for lineStart > 0 && src[lineStart-1] != '\n' {
		lineStart--
	}
	var lines []string
	for end := lineStart - 1; end > 0; {
		start := end
		for start > 0 && src[start-1] != '\n' {
			start--
		}
		line := strings.TrimSpace(string(src[start:end]))
		if !strings.HasPrefix(line, "//") {
			break
		}
		lines = append(lines, strings.TrimSpace(strings.TrimPrefix(line, "//")))
		end = start - 1
	}
	slices.Reverse(lines)
	return strings.Join(lines, "\n")
}

func funcSignature(fn hir.FuncDecl) string {
	params := make([]string, len(fn.Params))
	for i, p := range fn.Params {
		params[i] = p.Name + ": " + paramType(p.Type, p.IsArray, p.ArraySize)
	}
	return fmt.Sprintf("fn %s(%s) -> %s", fn.Name, strings.Join(params, ", "), fn.Returns)
}

func paramType(t hir.Type, isArray bool, size int) string {
	if isArray {
		return fmt.Sprintf("array<%s, %d>", t, size)
	}
	return string(t)
}

// material returns the material whose source contains offset.
func (a *analysis) material(offset int) (hir.Material, bool) {
	for _, m := range a.program.Materials {
		if contains(m.Span, offset) {
			return m, true
		}
	}
	return hir.Material{}, false
}

func (a *analysis) materialNamed(name string) (hir.Material, bool) {
	for _, m := range a.program.Materials {
		if m.Name == name {
			return m, true
		}
	}
	return hir.Material{}, false
}

// lineage returns m followed by the materials it extends, nearest first.
func (a *analysis) lineage(m hir.Material) []hir.Material {
	out := []hir.Material{m}
	for m.Extends != "" && len(out) <= len(a.program.Materials) {
		parent, ok := a.materialNamed(m.Extends)
		if !ok {
			break
		}
		out = append(out, parent)
		m = parent
	}
	return out
}

func (a *analysis) funcAt(offset int) (hir.FuncDecl, bool) {
	for _, fn := range a.program.Funcs {
		if contains(fn.Span, offset) {
			return fn, true
		}
	}
	return hir.FuncDecl{}, false
}

func contains(span hir.Span, offset int) bool {
	return !span.IsZero() && offset >= span.Start.Offset && offset <= span.End.Offset
}

// local is one let, var or loop counter.
type local struct {
	Name string
	Span hir.Span
	Kind string // "let", "var", "for"
}

// locals lists the bindings declared in body, nested blocks included.
func locals(body []hir.Stmt) []local {
	var out []local
	var walk func([]hir.Stmt)
	walk = func(stmts []hir.Stmt) {
		for _, s := range stmts {
			switch x := s.(type) {
			case hir.Let:
				out = append(out, local{x.Name, x.Span, "let"})
			case hir.VarDecl:
				out = append(out, local{x.Name, x.Span, "var"})
			case hir.VarArrayDecl:
				out = append(out, local{x.Name, x.Span, "var"})
			case hir.For:
				out = append(out, local{x.InitName, x.Span, "for"})
				walk(x.Body)
			case hir.If:
				walk(x.Then)
				walk(x.Else)
			}
		}
	}
	walk(body)
	return out
}

// stageBodies returns the authored function bodies of m that contain offset,
// with the stage they belong to.
func stageAt(m hir.Material, offset int) (fn *hir.Func, vertex bool) {
	if m.Vertex != nil && contains(m.Vertex.Span, offset) {
		return m.Vertex, true
	}
	if m.Specular != nil && contains(m.Specular.Span, offset) {
		return m.Specular, false
	}
	if contains(m.Surface.Span, offset) {
		return &m.Surface, false
	}
	return nil, false
}

// irLocalType finds the type the compiler gave a local in a stage.
func irLocalType(stmts []ir.Stmt, name string) (ir.Type, bool) {
	for _, s := range stmts {
		switch cf := s.CF.(type) {
		case nil:
			if s.Target == name {
				return s.Type, true
			}
		case ir.ForCF:
			if cf.InitTarget == name {
				return cf.InitType, true
			}
			if t, ok := irLocalType(cf.Body, name); ok {
				return t, true
			}
		case ir.IfCF:
			if t, ok := irLocalType(cf.Then, name); ok {
				return t, true
			}
			if t, ok := irLocalType(cf.Else, name); ok {
				return t, true
			}
		}
	}
	return "", false
}

// repairedProgram parses a copy of src with broken lines blanked, keeping
// byte offsets. When hint is not negative it first blanks the line at hint,
// where a completion request's cursor sits, if that line looks unfinished.
// Then it blanks one line per syntax error (see blankFrom), up to twelve.
func repairedProgram(src []byte, hint int) (hir.Program, bool) {
	text := append([]byte(nil), src...)
	if hint >= 0 {
		if line := lineAt(text, hint); unfinished(line) {
			blankLine(text, lineStart(text, hint))
		}
	}
	for range 12 {
		program, err := parse.Program(text)
		if err == nil {
			return program, true
		}
		var pe *parse.Error
		if !errors.As(err, &pe) || pe.Span.IsZero() || !blankFrom(text, lineStart(text, pe.Span.Start.Offset)) {
			return hir.Program{}, false
		}
	}
	return hir.Program{}, false
}

func lineAt(text []byte, offset int) string {
	start, end := lineStart(text, offset), offset
	for end < len(text) && text[end] != '\n' {
		end++
	}
	return strings.TrimSpace(string(text[start:end]))
}

func lineStart(text []byte, offset int) int {
	if offset > len(text) {
		offset = len(text)
	}
	for offset > 0 && text[offset-1] != '\n' {
		offset--
	}
	return offset
}

// blankFrom blanks one line at or after start. It prefers the first line
// that looks unfinished (see unfinished) within the next 40 lines, and falls
// back to the first line that has text and no brace, so block structure
// survives. It reports false when no line qualifies.
func blankFrom(text []byte, start int) bool {
	fallback := -1
	for i, at := 0, start; at < len(text) && i < 40; i++ {
		end := at
		for end < len(text) && text[end] != '\n' {
			end++
		}
		line := strings.TrimSpace(string(text[at:end]))
		if line != "" && unfinished(line) {
			return blankLine(text, at)
		}
		if fallback < 0 && line != "" && !strings.ContainsAny(line, "{}") {
			fallback = at
		}
		at = end + 1
	}
	return fallback >= 0 && blankLine(text, fallback)
}

// unfinished reports a line that is probably half typed: it ends in an
// operator, dot, comma or open bracket, its parentheses do not balance, or it
// is a lone identifier such as a context field without its type.
func unfinished(line string) bool {
	if i := strings.Index(line, "//"); i >= 0 {
		line = strings.TrimSpace(line[:i])
	}
	if line == "" || strings.ContainsAny(line, "{}") {
		return false
	}
	if strings.ContainsAny(line[len(line)-1:], ".=+-*/(,:<") {
		return true
	}
	if strings.Count(line, "(") != strings.Count(line, ")") {
		return true
	}
	lone := true
	for i := 0; i < len(line); i++ {
		if !isIdentByte(line[i]) {
			lone = false
			break
		}
	}
	return lone && line != "break" && line != "discard"
}

// blankLine replaces the line that starts at start with spaces. It reports
// false when that line was already blank.
func blankLine(text []byte, start int) bool {
	changed := false
	for i := start; i < len(text) && text[i] != '\n'; i++ {
		if text[i] != ' ' && text[i] != '\t' && text[i] != '\r' {
			text[i] = ' '
			changed = true
		}
	}
	return changed
}
