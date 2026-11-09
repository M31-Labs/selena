package lsp

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"m31labs.dev/selena/bindings"
	"m31labs.dev/selena/hir"
	"m31labs.dev/selena/ir"
	"m31labs.dev/selena/lower"
	"m31labs.dev/selena/materiallib"
)

// geoDocs explains the geometry fields of a mesh surface.
var geoDocs = map[string]string{
	"position":    "Vertex position attribute. Scene3D bakes mesh positions to world space.",
	"normal":      "Vertex normal attribute.",
	"uv":          "Texture coordinate.",
	"worldNormal": "World-space normal (normalMatrix times normal, normalized); synthesized from the normal attribute.",
	"worldPos":    "World-space surface position.",
	"pointUV":     "Position across the point billboard, 0..1 on both axes.",
	"color":       "Point color from the point buffer.",
	"alpha":       "Point alpha, already multiplied by material opacity.",
	"pointSize":   "Billboard size in pixels.",
	"fogFactor":   "Exponential fog factor 0..1 for this point.",
}

// symbol is what a name at the cursor refers to.
type symbol struct {
	Name  string
	Hover string
	// Decl is the declaration range in this document; DeclEnd <= DeclStart
	// means there is none here.
	DeclStart, DeclEnd int
	Lib                *libFunc
}

// resolve finds the symbol under offset.
func (a *analysis) resolve(offset int) (symbol, int, int, bool) {
	word, start, end, ok := wordAt(a.src, offset)
	if !ok || inComment(a.src, start) {
		return symbol{}, 0, 0, false
	}
	sym, ok := a.resolveWord(word, start, end)
	return sym, start, end, ok
}

func (a *analysis) resolveWord(word string, start, end int) (symbol, bool) {
	none := symbol{Name: word, DeclStart: -1, DeclEnd: -1}
	m, inMaterial := a.material(start)
	fn, inFunc := a.funcAt(start)

	if prev, at := prevNonSpace(a.src, start); prev == '.' {
		recv, _, _, ok := wordAt(a.src, at)
		if !ok {
			return none, false
		}
		return a.member(m, inMaterial, recv, word, start)
	}
	if nextNonSpace(a.src, end) == '(' {
		if sym, ok := a.callee(word); ok {
			return sym, true
		}
	}
	if inFunc {
		for _, p := range fn.Params {
			if p.Name == word {
				return a.declared(word, "(parameter) "+word+": "+paramType(p.Type, p.IsArray, p.ArraySize), p.Span), true
			}
		}
		for _, l := range fn.Body {
			if l.Name == word {
				return a.declared(word, "(local) let "+word, l.Span), true
			}
		}
		// Helpers read context fields by name from the calling material.
		if in, ok := bindings.LookupEngineInput(word); ok {
			sym := none
			sym.Hover = engineHover(in) + "\n\nA function reads this from the calling material's `context` block."
			return sym, true
		}
	}
	if inMaterial {
		if sym, ok := a.materialName(m, word, start); ok {
			return sym, true
		}
	}
	if other, ok := a.materialNamed(word); ok {
		return a.declared(word, materialHover(other), nameSpan(a.src, other.Span, other.Name)), true
	}
	if sym, ok := a.callee(word); ok {
		return sym, true
	}
	if doc, ok := typeDocs[word]; ok {
		none.Hover = "```selena\n" + word + "\n```\n" + doc
		return none, true
	}
	if doc, ok := kindDocs[word]; ok {
		none.Hover = "```selena\nkind " + word + "\n```\n" + doc
		return none, true
	}
	if in, ok := bindings.LookupEngineInput(word); ok {
		none.Hover = engineHover(in) + "\n\nDeclare it in the material's `context` block to read it."
		return none, true
	}
	return none, false
}

// materialName resolves a plain name inside material m: locals of the stage
// at the cursor, then params, context fields, varyings and statefields of m
// and the materials it extends.
func (a *analysis) materialName(m hir.Material, word string, offset int) (symbol, bool) {
	if body, vertex := stageAt(m, offset); body != nil {
		if word == body.Geo {
			return a.declared(word, "(geometry) "+word+" — the geometry record; read fields with `"+word+".`", hir.Span{}), true
		}
		for _, l := range locals(body.Body) {
			if l.Name != word {
				continue
			}
			text := "(local) " + l.Kind + " " + word
			if t, ok := a.localType(m, word, vertex); ok {
				text += ": " + string(t)
			}
			return a.declared(word, text, nameSpan(a.src, l.Span, word)), true
		}
	}
	for _, owner := range a.lineage(m) {
		inherited := ""
		if owner.Name != m.Name {
			inherited = "\n\nInherited from `" + owner.Name + "`."
		}
		for _, p := range owner.Params {
			if p.Name == word {
				text := "(param) " + word + " : " + paramType(p.Type, p.IsArray, p.ArraySize)
				if p.Format != "" {
					text += " format " + p.Format
				}
				return a.declared(word, "```selena\n"+text+"\n```"+typeNote(p.Type)+inherited, p.Span), true
			}
		}
		for _, c := range owner.Context {
			if c.Name != word {
				continue
			}
			text := "```selena\n(context) " + word + " : " + paramType(c.Type, c.IsArray, c.ArraySize) + "\n```"
			if in, ok := bindings.LookupEngineInput(word); ok {
				text += "\n" + engineDoc(in)
			} else {
				text += "\nHost-supplied context field; not in the engine input registry."
			}
			return a.declared(word, text+inherited, c.Span), true
		}
		for _, v := range owner.Varyings {
			if v.Name == word {
				return a.declared(word, "```selena\n(varying) "+word+" : "+string(v.Type)+"\n```"+inherited, v.Span), true
			}
		}
		for _, s := range owner.States {
			if s.Name == word {
				return a.declared(word, "```selena\n(statefield) state "+word+"\n```\nvec4 per cell; read with state(dx, dy) or stateAt(uv)."+inherited, s.Span), true
			}
		}
	}
	return symbol{}, false
}

func (a *analysis) localType(m hir.Material, name string, vertex bool) (ir.Type, bool) {
	res, ok := a.results[m.Name]
	if !ok {
		return "", false
	}
	stage := res.Module.Fragment
	if vertex {
		stage = res.Module.Vertex
	}
	return irLocalType(stage.Body, name)
}

// member resolves field after "recv.".
func (a *analysis) member(m hir.Material, inMaterial bool, recv, field string, offset int) (symbol, bool) {
	none := symbol{Name: field, DeclStart: -1, DeclEnd: -1}
	if recv == "super" && inMaterial {
		if parent, ok := a.materialNamed(m.Extends); ok {
			return a.declared(field, "```selena\nsuper.surface(geo)\n```\nThe surface of the parent material `"+parent.Name+"`, inlined.", parent.Surface.Span), true
		}
		return none, false
	}
	if !inMaterial {
		return none, false
	}
	if body, vertex := stageAt(m, offset); body != nil && recv == body.Geo {
		if t, ok := lower.GeometryFields(kindOf(m), vertex)[field]; ok {
			none.Hover = "```selena\n(geometry) " + recv + "." + field + " : " + string(t) + "\n```\n" + geoDocs[field]
			return none, true
		}
		for _, v := range m.Varyings {
			if v.Name == field && !vertex {
				return a.declared(field, "```selena\n(varying) "+recv+"."+field+" : "+string(v.Type)+"\n```\nWritten by vertex().", v.Span), true
			}
		}
		return none, false
	}
	for _, owner := range a.lineage(m) {
		for _, p := range owner.Params {
			if p.Name != recv {
				continue
			}
			if t, ok := lower.RecordFields(p.Type)[field]; ok {
				none.Hover = "```selena\n" + recv + "." + field + " : " + string(t) + "\n```\nField of the `" + string(p.Type) + "` record."
				return none, true
			}
		}
	}
	return none, false
}

// callee resolves a function name: the file's functions, then builtins, then
// material library helpers.
func (a *analysis) callee(word string) (symbol, bool) {
	for _, fn := range a.program.Funcs {
		if fn.Name == word {
			text := "```selena\n" + funcSignature(fn) + "\n```"
			if doc := leadingComment(a.src, fn.Span.Start.Offset); doc != "" {
				text += "\n" + doc
			}
			return a.declared(word, text, nameSpan(a.src, fn.Span, fn.Name)), true
		}
	}
	if doc, ok := builtinDocs[word]; ok {
		return symbol{Name: word, DeclStart: -1, DeclEnd: -1, Hover: "```selena\n" + doc.Signature + "\n```\n" + doc.Doc + "\n\nSelena builtin."}, true
	}
	if lib, ok := libraryIndex()[word]; ok {
		lib := lib
		text := "```selena\n" + funcSignature(lib.Decl) + "\n```"
		if lib.Doc != "" {
			text += "\n" + lib.Doc
		}
		text += "\n\nMaterial library module `" + string(lib.Module) + "`, linked when the file calls it."
		return symbol{Name: word, DeclStart: -1, DeclEnd: -1, Hover: text, Lib: &lib}, true
	}
	return symbol{}, false
}

func (a *analysis) declared(name, hover string, span hir.Span) symbol {
	sym := symbol{Name: name, Hover: hover, DeclStart: -1, DeclEnd: -1}
	if !span.IsZero() {
		decl := nameSpan(a.src, span, name)
		sym.DeclStart, sym.DeclEnd = decl.Start.Offset, decl.End.Offset
	}
	return sym
}

func materialHover(m hir.Material) string {
	head := "material " + m.Name
	if m.Kind != "" && m.Kind != hir.KindMesh {
		head += " kind " + string(m.Kind)
	}
	if m.Extends != "" {
		head += " extends " + m.Extends
	}
	text := "```selena\n" + head + "\n```\n" + fmt.Sprintf("%d params, %d context fields.", len(m.Params), len(m.Context))
	return text
}

func engineDoc(in bindings.EngineInput) string {
	kind := "Engine value"
	if in.Kind == bindings.EngineTexture {
		kind = "Engine texture"
	}
	return fmt.Sprintf("%s (%s): %s\n\nSpace or encoding: %s.", kind, bindings.EngineVersion, in.Doc, in.Space)
}

func engineHover(in bindings.EngineInput) string {
	return "```selena\n(engine input) " + in.Name + " : " + in.Declaration() + "\n```\n" + engineDoc(in)
}

func typeNote(t hir.Type) string {
	if doc, ok := typeDocs[string(t)]; ok {
		return "\n" + doc
	}
	return ""
}

func kindOf(m hir.Material) hir.Kind {
	if m.Kind == "" {
		return hir.KindMesh
	}
	return m.Kind
}

// hover answers textDocument/hover.
func (a *analysis) hover(pos Position) *Hover {
	sym, start, end, ok := a.resolve(offsetAt(a.src, pos))
	if !ok || sym.Hover == "" {
		return nil
	}
	r := rangeOf(a.src, start, end)
	return &Hover{Contents: markdown(sym.Hover), Range: &r}
}

// definition answers textDocument/definition.
func (a *analysis) definition(uri string, pos Position) []Location {
	sym, _, _, ok := a.resolve(offsetAt(a.src, pos))
	if !ok {
		return nil
	}
	if sym.DeclEnd > sym.DeclStart {
		return []Location{{URI: uri, Range: rangeOf(a.src, sym.DeclStart, sym.DeclEnd)}}
	}
	if sym.Lib != nil {
		path, err := libraryFile(sym.Lib.Module, sym.Lib.Source)
		if err != nil {
			return nil
		}
		decl := nameSpan(sym.Lib.Source, sym.Lib.Decl.Span, sym.Lib.Decl.Name)
		return []Location{{URI: pathURI(path), Range: rangeOf(sym.Lib.Source, decl.Start.Offset, decl.End.Offset)}}
	}
	return nil
}

// libraryFile writes a module's source to the user cache directory, so an
// editor can open it, and returns the path. The file is rewritten only when
// its bytes differ.
func libraryFile(module materiallib.Module, data []byte) (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		dir = os.TempDir()
	}
	dir = filepath.Join(dir, "selena", "materiallib")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, string(module)+".sel")
	if have, err := os.ReadFile(path); err == nil && string(have) == string(data) {
		return path, nil
	}
	return path, os.WriteFile(path, data, 0o644)
}

// completion answers textDocument/completion.
func (a *analysis) completion(pos Position) []CompletionItem {
	offset := offsetAt(a.src, pos)
	if inComment(a.src, offset) {
		return nil
	}
	if !a.parsed {
		// The line being typed is the likeliest syntax error: parse without it.
		if program, ok := repairedProgram(a.src, offset); ok && len(program.Materials) >= len(a.program.Materials) {
			repaired := *a
			repaired.program = program
			a = &repaired
		}
	}
	start := offset
	for start > 0 && isIdentByte(a.src[start-1]) {
		start--
	}
	prefix := string(a.src[start:offset])
	var items []CompletionItem
	add := func(item CompletionItem) {
		if strings.HasPrefix(item.Label, prefix) {
			items = append(items, item)
		}
	}
	m, inMaterial := a.material(start)
	prev, at := prevNonSpace(a.src, start)
	switch {
	case prev == '.':
		a.memberCompletions(m, inMaterial, at, start, add)
	case a.inContextBlock(start):
		for _, in := range bindings.EngineInputs() {
			add(CompletionItem{Label: in.Name, Kind: CompletionField, Detail: in.Declaration(),
				Documentation: ptr(markdown(engineDoc(in))), InsertText: in.Name + " : " + in.Declaration()})
		}
	case prev == ':' || prev == '<' || prev == ',' && a.inTypeArgs(start):
		for _, name := range sortedKeys(typeDocs) {
			add(CompletionItem{Label: name, Kind: CompletionTypeParameter, Documentation: ptr(markdown(typeDocs[name]))})
		}
	case precededByWord(a.src, start, "kind"):
		for _, name := range sortedKeys(kindDocs) {
			add(CompletionItem{Label: name, Kind: CompletionKeyword, Documentation: ptr(markdown(kindDocs[name]))})
		}
	case precededByWord(a.src, start, "format"):
		for _, name := range textureFormats {
			add(CompletionItem{Label: name, Kind: CompletionKeyword})
		}
	case precededByWord(a.src, start, "extends"):
		for _, other := range a.program.Materials {
			add(CompletionItem{Label: other.Name, Kind: CompletionStruct, Detail: "material"})
		}
	default:
		a.scopeCompletions(m, inMaterial, start, add)
	}
	return items
}

func (a *analysis) memberCompletions(m hir.Material, inMaterial bool, dot, offset int, add func(CompletionItem)) {
	recv, _, _, ok := wordAt(a.src, dot)
	if !ok || !inMaterial {
		return
	}
	if recv == "super" {
		add(CompletionItem{Label: "surface", Kind: CompletionFunction, Detail: "super.surface(geo)", InsertText: "surface"})
		return
	}
	if body, vertex := stageAt(m, offset); body != nil && recv == body.Geo {
		fields := lower.GeometryFields(kindOf(m), vertex)
		for _, name := range sortedKeys(fields) {
			add(CompletionItem{Label: name, Kind: CompletionProperty, Detail: string(fields[name]), Documentation: ptr(markdown(geoDocs[name]))})
		}
		if !vertex {
			for _, v := range m.Varyings {
				add(CompletionItem{Label: v.Name, Kind: CompletionProperty, Detail: string(v.Type) + " (varying)"})
			}
		}
		return
	}
	for _, owner := range a.lineage(m) {
		for _, p := range owner.Params {
			if p.Name != recv {
				continue
			}
			fields := lower.RecordFields(p.Type)
			for _, name := range sortedKeys(fields) {
				add(CompletionItem{Label: name, Kind: CompletionProperty, Detail: string(fields[name])})
			}
			return
		}
	}
}

func (a *analysis) scopeCompletions(m hir.Material, inMaterial bool, offset int, add func(CompletionItem)) {
	for _, kw := range keywordList {
		add(CompletionItem{Label: kw, Kind: CompletionKeyword})
	}
	if inMaterial {
		if body, vertex := stageAt(m, offset); body != nil {
			add(CompletionItem{Label: body.Geo, Kind: CompletionVariable, Detail: "geometry record"})
			for _, l := range locals(body.Body) {
				if l.Span.Start.Offset >= offset {
					continue
				}
				detail := l.Kind
				if t, ok := a.localType(m, l.Name, vertex); ok {
					detail += " " + string(t)
				}
				add(CompletionItem{Label: l.Name, Kind: CompletionVariable, Detail: detail})
			}
		}
		for _, owner := range a.lineage(m) {
			for _, p := range owner.Params {
				add(CompletionItem{Label: p.Name, Kind: CompletionVariable, Detail: "param " + paramType(p.Type, p.IsArray, p.ArraySize)})
			}
			for _, c := range owner.Context {
				add(CompletionItem{Label: c.Name, Kind: CompletionVariable, Detail: "context " + paramType(c.Type, c.IsArray, c.ArraySize)})
			}
		}
	}
	for _, fn := range a.program.Funcs {
		add(CompletionItem{Label: fn.Name, Kind: CompletionFunction, Detail: funcSignature(fn)})
	}
	for _, name := range sortedKeys(builtinDocs) {
		doc := builtinDocs[name]
		add(CompletionItem{Label: name, Kind: CompletionFunction, Detail: doc.Signature, Documentation: ptr(markdown(doc.Doc))})
	}
	index := libraryIndex()
	for _, name := range sortedKeys(index) {
		lib := index[name]
		text := lib.Doc
		if text != "" {
			text += "\n\n"
		}
		text += "Material library module `" + string(lib.Module) + "`."
		add(CompletionItem{Label: name, Kind: CompletionFunction, Detail: funcSignature(lib.Decl), Documentation: ptr(markdown(text))})
	}
}

// inContextBlock reports whether offset sits inside a `context { ... }` block.
func (a *analysis) inContextBlock(offset int) bool {
	text := stripComments(a.src[:offset])
	i := lastWord(text, "context")
	if i < 0 {
		return false
	}
	open := strings.IndexByte(string(text[i:]), '{')
	if open < 0 {
		return false
	}
	return !strings.Contains(string(text[i+open:]), "}")
}

// inTypeArgs reports whether offset sits inside array< ... >.
func (a *analysis) inTypeArgs(offset int) bool {
	text := string(a.src[:offset])
	open := strings.LastIndex(text, "array<")
	return open >= 0 && !strings.Contains(text[open:], ">")
}

// lastWord returns the index of the last whole-word occurrence of word.
func lastWord(text []byte, word string) int {
	s := string(text)
	for i := strings.LastIndex(s, word); i >= 0; i = strings.LastIndex(s[:i], word) {
		before := i == 0 || !isIdentByte(s[i-1])
		after := i+len(word) >= len(s) || !isIdentByte(s[i+len(word)])
		if before && after {
			return i
		}
		if i == 0 {
			break
		}
	}
	return -1
}

func precededByWord(src []byte, offset int, word string) bool {
	_, at := prevNonSpace(src, offset)
	if at < 0 {
		return false
	}
	w, _, _, ok := wordAt(src, at+1)
	return ok && w == word
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func ptr[T any](v T) *T { return &v }
