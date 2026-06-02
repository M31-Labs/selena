package lower

import (
	"fmt"
	"sort"
	"strings"

	"m31labs.dev/selena/bindings"
	"m31labs.dev/selena/hir"
	"m31labs.dev/selena/ir"
)

func hirToIRType(t hir.Type) (ir.Type, bool) {
	switch t {
	case hir.Float:
		return ir.Float, true
	case hir.Vec2:
		return ir.Vec2, true
	case hir.Vec3, hir.Color:
		return ir.Vec3, true
	case hir.Vec4:
		return ir.Vec4, true
	case hir.Mat3:
		return ir.Mat3, true
	case hir.Mat4:
		return ir.Mat4, true
	}
	return "", false
}

// LowerWith inlines the given user functions into the material, then lowers it.
// Materials with no user functions can call Lower directly.
func LowerWith(m hir.Material, funcs []hir.FuncDecl) (ir.Module, bindings.Layout, error) {
	flat, err := resolveExtends(m, nil)
	if err != nil {
		return ir.Module{}, bindings.Layout{}, err
	}
	inlined, err := inlineFuncs(flat, funcs)
	if err != nil {
		return ir.Module{}, bindings.Layout{}, err
	}
	return Lower(inlined)
}

// Lower compiles a high-level material into the low-level ir.Module plus the
// host binding layout. This is where the pains disappear: bindings are
// allocated, the std140 layout is computed, interpolants are inferred and the
// vertex stage synthesized from the stdlib transform, stdlib records expand into
// uniforms, locals are typed, and a vec3 surface result is wrapped to vec4.
func Lower(m hir.Material) (ir.Module, bindings.Layout, error) {
	fail := func(format string, a ...any) (ir.Module, bindings.Layout, error) {
		return ir.Module{}, bindings.Layout{}, fmt.Errorf(format, a...)
	}
	if m.Vertex != nil {
		return ir.Module{}, bindings.Layout{}, diagnostic(
			CodeUnsupportedFeat,
			m.Vertex.Span,
			"vertex hooks are not supported yet; Selena currently emits the default transform",
		)
	}

	paramKind := map[string]hir.Type{}
	for _, p := range m.Params {
		if err := validateAuthorName("param", p.Name, p.Span); err != nil {
			return ir.Module{}, bindings.Layout{}, err
		}
		if _, ok := paramKind[p.Name]; ok {
			return ir.Module{}, bindings.Layout{}, diagnostic(CodeDuplicateParam, p.Span, "duplicate param %q", p.Name)
		}
		paramKind[p.Name] = p.Type
	}

	// --- ordered uniforms: implicit Transform first, then params/records ---
	var uniforms []bindings.NamedType
	var defaults []bindings.DefaultValue
	uniType := map[string]ir.Type{}
	addUniform := func(name string, t ir.Type) error {
		if _, ok := uniType[name]; ok {
			return fmt.Errorf("uniform %q is declared more than once", name)
		}
		uniforms = append(uniforms, bindings.NamedType{Name: name, Type: t})
		uniType[name] = t
		return nil
	}
	if err := addUniform("mvp", ir.Mat4); err != nil {
		return fail("%w", err)
	}
	if err := addUniform("normalMatrix", ir.Mat3); err != nil {
		return fail("%w", err)
	}

	uniformOf := map[string]string{} // "baseColor" or "light.dir" -> uniform name
	var textures []string            // texture params, in declaration order
	for _, p := range m.Params {
		switch p.Type {
		case hir.Sun:
			var sunFields map[string][]float32
			if p.Default != nil {
				fields, err := sunDefault(p.Default)
				if err != nil {
					return ir.Module{}, bindings.Layout{}, diagnostic(CodeInvalidDefault, hir.ExprSpan(p.Default), "param %q default: %v", p.Name, err)
				}
				sunFields = fields
			}
			for _, f := range sortedKeys(stdlib.recordFields(string(hir.Sun))) {
				un := p.Name + "_" + f
				ft, _ := stdlib.recordField(string(hir.Sun), f)
				if err := addUniform(un, ft); err != nil {
					return fail("param %q: %w", p.Name, err)
				}
				uniformOf[p.Name+"."+f] = un
				if sunFields != nil {
					defaults = append(defaults, bindings.DefaultValue{Name: un, Type: string(ft), Values: sunFields[f]})
				}
			}
		case hir.Texture2D:
			if p.Default != nil {
				return ir.Module{}, bindings.Layout{}, diagnostic(CodeInvalidDefault, hir.ExprSpan(p.Default), "param %q: defaults for texture2d are not supported", p.Name)
			}
			textures = append(textures, p.Name)
		default:
			t, ok := hirToIRType(p.Type)
			if !ok {
				return ir.Module{}, bindings.Layout{}, diagnostic(CodeUnsupportedType, p.Span, "param %q: unsupported type %q", p.Name, p.Type)
			}
			if err := addUniform(p.Name, t); err != nil {
				return fail("param %q: %w", p.Name, err)
			}
			uniformOf[p.Name] = p.Name
			if p.Default != nil {
				vals, err := constDefault(p.Default, p.Type)
				if err != nil {
					return ir.Module{}, bindings.Layout{}, diagnostic(CodeInvalidDefault, hir.ExprSpan(p.Default), "param %q default: %v", p.Name, err)
				}
				defaults = append(defaults, bindings.DefaultValue{Name: p.Name, Type: string(t), Values: vals})
			}
		}
	}

	// --- which geo fields does the surface read? ---
	usedGeo := map[string]bool{}
	collectGeo(m.Surface.Result, m.Surface.Geo, usedGeo)
	for _, l := range m.Surface.Body {
		collectGeo(l.Value, m.Surface.Geo, usedGeo)
	}

	// --- synthesize varyings + vertex body from used geo fields ---
	attrNeeded := map[string]bool{"position": true} // transform always needs position
	varyingOf := map[string]string{}
	var varyings []ir.Binding
	var vertexBody []ir.Stmt
	for _, f := range sortedSet(usedGeo) {
		gf, ok := stdlib.geometryField(f)
		if !ok {
			return fail("unknown geometry field geo.%s", f)
		}
		for _, a := range gf.attrs {
			attrNeeded[a] = true
		}
		varyingOf[f] = gf.varying
		varyings = append(varyings, ir.Binding{Name: gf.varying, Type: gf.typ})
		vertexBody = append(vertexBody, ir.Stmt{Target: gf.varying, Type: gf.typ, Value: gf.build()})
	}

	// --- attributes: position first (location 0), then the rest, deterministically ---
	position, _ := stdlib.geometryField("position")
	attributes := []ir.Binding{{Name: "position", Type: position.typ}}
	for _, a := range sortedSet(attrNeeded) {
		if a != "position" {
			gf, ok := stdlib.geometryField(a)
			if !ok {
				return fail("attribute %q is not registered", a)
			}
			attributes = append(attributes, ir.Binding{Name: a, Type: gf.typ})
		}
	}
	reserved, err := interfaceNames(uniforms, textures, attributes, varyings)
	if err != nil {
		return fail("%w", err)
	}

	rs := &resolver{paramKind: paramKind, uniformOf: uniformOf, varyingOf: varyingOf, geo: m.Surface.Geo}
	tp := &typer{paramKind: paramKind, geo: m.Surface.Geo, locals: map[string]ir.Type{}}

	// --- fragment body: type each local, then lower it ---
	var fragBody []ir.Stmt
	seenLocals := map[string]bool{}
	for _, l := range m.Surface.Body {
		if err := validateAuthorName("surface local", l.Name, l.Span); err != nil {
			return ir.Module{}, bindings.Layout{}, err
		}
		if kind, ok := reserved[l.Name]; ok {
			return ir.Module{}, bindings.Layout{}, diagnostic(CodeDuplicateLocal, l.Span, "surface local %q conflicts with %s", l.Name, kind)
		}
		if seenLocals[l.Name] {
			return ir.Module{}, bindings.Layout{}, diagnostic(CodeDuplicateLocal, l.Span, "duplicate surface local %q", l.Name)
		}
		t, err := tp.typeOf(l.Value)
		if err != nil {
			return fail("surface local %q: %w", l.Name, err)
		}
		ve, err := rs.expr(l.Value)
		if err != nil {
			return fail("surface local %q: %w", l.Name, err)
		}
		seenLocals[l.Name] = true
		tp.locals[l.Name] = t
		fragBody = append(fragBody, ir.Stmt{Target: l.Name, Type: t, Value: ve})
	}

	// --- fragment output: wrap a vec3/color result to vec4 ---
	rt, err := tp.typeOf(m.Surface.Result)
	if err != nil {
		return fail("surface result: %w", err)
	}
	re, err := rs.expr(m.Surface.Result)
	if err != nil {
		return fail("surface result: %w", err)
	}
	var fragOut ir.Expr
	switch rt {
	case ir.Vec4:
		fragOut = re
	case ir.Vec3:
		fragOut = ir.Construct{Type: ir.Vec4, Args: []ir.Expr{re, ir.Lit{Value: 1.0}}}
	default:
		return fail("surface must return color/vec3/vec4, got %s", rt)
	}

	vertexOut := ir.Binary{Op: "*", L: ir.Ref{Name: "mvp"},
		R: ir.Construct{Type: ir.Vec4, Args: []ir.Expr{ir.Ref{Name: "position"}, ir.Lit{Value: 1.0}}}}

	irTextures := make([]ir.Texture, len(textures))
	for i, t := range textures {
		irTextures[i] = ir.Texture{Name: t}
	}

	mod := ir.Module{
		Name:       m.Name,
		Uniforms:   toBindings(uniforms),
		Attributes: attributes,
		Varyings:   varyings,
		Textures:   irTextures,
		Vertex:     ir.Stage{Body: vertexBody, Output: vertexOut},
		Fragment:   ir.Stage{Body: fragBody, Output: fragOut},
	}
	layout := bindings.Layout{
		SchemaVersion:   bindings.DescriptorSchemaVersion,
		LanguageVersion: bindings.LanguageVersion,
		Material:        m.Name,
		UniformBlock:    bindings.ComputeUniformBlock(uniforms),
		Attributes:      toAttrs(attributes),
		Textures:        bindings.ComputeTextures(textures),
		WGSL:            bindings.WGSLBinding{Group: 0, Binding: 0},
		Metal:           bindings.MetalBinding{Buffer: 0},
	}
	if layout.Textures == nil {
		layout.Textures = []bindings.Texture{}
	}
	layout.UniformBlock.Defaults = defaults
	return mod, layout, nil
}

// --- resolver: HIR expression -> LIR expression ---

type resolver struct {
	paramKind map[string]hir.Type
	uniformOf map[string]string
	varyingOf map[string]string
	geo       string
}

func (r *resolver) expr(e hir.Expr) (ir.Expr, error) {
	switch x := e.(type) {
	case hir.Lit:
		return ir.Lit{Value: x.Value}, nil
	case hir.Ref:
		if un, ok := r.uniformOf[x.Name]; ok {
			return ir.Ref{Name: un}, nil
		}
		return ir.Ref{Name: x.Name}, nil // stage-local
	case hir.Member:
		if base, ok := x.E.(hir.Ref); ok {
			if base.Name == r.geo {
				vn, ok := r.varyingOf[x.Field]
				if !ok {
					return nil, diagnostic(CodeInvalidMember, x.Span, "geo.%s is not available in the surface", x.Field)
				}
				return ir.Ref{Name: vn}, nil
			}
			if r.paramKind[base.Name] == hir.Sun {
				un, ok := r.uniformOf[base.Name+"."+x.Field]
				if !ok {
					return nil, diagnostic(CodeInvalidMember, x.Span, "Sun has no field %q", x.Field)
				}
				return ir.Ref{Name: un}, nil
			}
		}
		// otherwise a vector swizzle (.rgb, .xyz, .x)
		inner, err := r.expr(x.E)
		if err != nil {
			return nil, err
		}
		return ir.Swizzle{E: inner, Field: x.Field}, nil
	case hir.Call:
		if x.Func == "sample" {
			if len(x.Args) != 2 {
				return nil, diagnostic(CodeInvalidCall, x.Span, "sample(texture, uv) takes 2 arguments")
			}
			texRef, ok := x.Args[0].(hir.Ref)
			if !ok || r.paramKind[texRef.Name] != hir.Texture2D {
				return nil, diagnostic(CodeInvalidCall, x.Span, "sample: first argument must be a texture2d param")
			}
			uv, err := r.expr(x.Args[1])
			if err != nil {
				return nil, err
			}
			return ir.Sample{Texture: texRef.Name, UV: uv}, nil
		}
		args, err := r.args(x.Args)
		if err != nil {
			return nil, err
		}
		if x.Func == "rgb" {
			t := ir.Vec3
			if len(args) == 4 {
				t = ir.Vec4
			}
			return ir.Construct{Type: t, Args: args}, nil
		}
		return ir.Call{Func: x.Func, Args: args}, nil
	case hir.Binary:
		l, err := r.expr(x.L)
		if err != nil {
			return nil, err
		}
		rr, err := r.expr(x.R)
		if err != nil {
			return nil, err
		}
		return ir.Binary{Op: x.Op, L: l, R: rr}, nil
	}
	return nil, fmt.Errorf("unsupported expression %T", e)
}

func (r *resolver) args(in []hir.Expr) ([]ir.Expr, error) {
	out := make([]ir.Expr, len(in))
	for i, a := range in {
		v, err := r.expr(a)
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

// --- typer: infer the LIR type of an HIR expression ---

type typer struct {
	paramKind map[string]hir.Type
	geo       string
	locals    map[string]ir.Type
}

func (t *typer) typeOf(e hir.Expr) (ir.Type, error) {
	switch x := e.(type) {
	case hir.Lit:
		return ir.Float, nil
	case hir.Ref:
		if lt, ok := t.locals[x.Name]; ok {
			return lt, nil
		}
		if pk, ok := t.paramKind[x.Name]; ok {
			if it, ok := hirToIRType(pk); ok {
				return it, nil
			}
			return "", diagnostic(CodeInvalidMember, x.Span, "%q has record type %q and must be accessed by field", x.Name, pk)
		}
		return "", diagnostic(CodeUnknownName, x.Span, "unknown name %q", x.Name)
	case hir.Member:
		if base, ok := x.E.(hir.Ref); ok {
			if base.Name == t.geo {
				if gf, ok := stdlib.geometryField(x.Field); ok {
					return gf.typ, nil
				}
				return "", diagnostic(CodeInvalidMember, x.Span, "unknown geometry field geo.%s", x.Field)
			}
			if t.paramKind[base.Name] == hir.Sun {
				if ft, ok := stdlib.recordField(string(hir.Sun), x.Field); ok {
					return ft, nil
				}
				return "", diagnostic(CodeInvalidMember, x.Span, "Sun has no field %q", x.Field)
			}
		}
		bt, err := t.typeOf(x.E)
		if err != nil {
			return "", err
		}
		st, err := swizzleType(bt, x.Field) // .rgb, .xyz, .x …
		if err != nil {
			return "", diagnostic(CodeInvalidSwizzle, x.Span, "%v", err)
		}
		return st, nil
	case hir.Call:
		return t.callType(x)
	case hir.Binary:
		return t.binaryType(x)
	}
	return "", fmt.Errorf("unsupported expression %T", e)
}

func (t *typer) callType(c hir.Call) (ir.Type, error) {
	spec, ok := stdlib.builtin(c.Func)
	if !ok {
		if len(c.Args) == 0 {
			return "", diagnostic(CodeInvalidCall, c.Span, "call %q has no arguments", c.Func)
		}
		return t.typeOf(c.Args[0])
	}

	switch spec.kind {
	case builtinDot:
		if len(c.Args) != spec.arity {
			return "", diagnostic(CodeInvalidCall, c.Span, "%s expects %d arguments, got %d", c.Func, spec.arity, len(c.Args))
		}
		a, err := t.typeOf(c.Args[0])
		if err != nil {
			return "", err
		}
		b, err := t.typeOf(c.Args[1])
		if err != nil {
			return "", err
		}
		if !isVector(a) || a != b {
			return "", diagnostic(CodeTypeMismatch, c.Span, "dot arguments must be matching vectors, got %s and %s", a, b)
		}
		return ir.Float, nil
	case builtinLength:
		if len(c.Args) != spec.arity {
			return "", diagnostic(CodeInvalidCall, c.Span, "%s expects %d arguments, got %d", c.Func, spec.arity, len(c.Args))
		}
		a, err := t.typeOf(c.Args[0])
		if err != nil {
			return "", err
		}
		if !isVector(a) {
			return "", diagnostic(CodeTypeMismatch, c.Span, "length argument must be a vector, got %s", a)
		}
		return ir.Float, nil
	case builtinDistance:
		if len(c.Args) != spec.arity {
			return "", diagnostic(CodeInvalidCall, c.Span, "%s expects %d arguments, got %d", c.Func, spec.arity, len(c.Args))
		}
		a, err := t.typeOf(c.Args[0])
		if err != nil {
			return "", err
		}
		b, err := t.typeOf(c.Args[1])
		if err != nil {
			return "", err
		}
		if !isVector(a) || a != b {
			return "", diagnostic(CodeTypeMismatch, c.Span, "distance arguments must be matching vectors, got %s and %s", a, b)
		}
		return ir.Float, nil
	case builtinSample:
		if len(c.Args) != spec.arity {
			return "", diagnostic(CodeInvalidCall, c.Span, "sample(texture, uv) takes 2 arguments")
		}
		tex, ok := c.Args[0].(hir.Ref)
		if !ok || t.paramKind[tex.Name] != hir.Texture2D {
			return "", diagnostic(CodeInvalidCall, c.Span, "sample: first argument must be a texture2d param")
		}
		uv, err := t.typeOf(c.Args[1])
		if err != nil {
			return "", err
		}
		if uv != ir.Vec2 {
			return "", diagnostic(CodeTypeMismatch, c.Span, "sample: second argument must be vec2 uv, got %s", uv)
		}
		return ir.Vec4, nil
	case builtinRGB:
		if len(c.Args) != 3 && len(c.Args) != 4 {
			return "", diagnostic(CodeInvalidCall, c.Span, "rgb expects 3 or 4 arguments, got %d", len(c.Args))
		}
		for i, a := range c.Args {
			at, err := t.typeOf(a)
			if err != nil {
				return "", err
			}
			if at != ir.Float {
				return "", diagnostic(CodeTypeMismatch, c.Span, "rgb argument %d must be float, got %s", i+1, at)
			}
		}
		if len(c.Args) == 4 {
			return ir.Vec4, nil
		}
		return ir.Vec3, nil
	case builtinUnarySame:
		if len(c.Args) != spec.arity {
			return "", diagnostic(CodeInvalidCall, c.Span, "%s expects %d arguments, got %d", c.Func, spec.arity, len(c.Args))
		}
		return t.typeOf(c.Args[0])
	case builtinSameOrScalar:
		return t.sameOrScalarCall(c.Func, c.Args, spec.arity, c.Span)
	case builtinCross:
		if len(c.Args) != spec.arity {
			return "", diagnostic(CodeInvalidCall, c.Span, "%s expects %d arguments, got %d", c.Func, spec.arity, len(c.Args))
		}
		a, err := t.typeOf(c.Args[0])
		if err != nil {
			return "", err
		}
		b, err := t.typeOf(c.Args[1])
		if err != nil {
			return "", err
		}
		if a != ir.Vec3 || b != ir.Vec3 {
			return "", diagnostic(CodeTypeMismatch, c.Span, "cross arguments must both be vec3, got %s and %s", a, b)
		}
		return ir.Vec3, nil
	case builtinStep:
		if len(c.Args) != spec.arity {
			return "", diagnostic(CodeInvalidCall, c.Span, "%s expects %d arguments, got %d", c.Func, spec.arity, len(c.Args))
		}
		last, err := t.typeOf(c.Args[len(c.Args)-1])
		if err != nil {
			return "", err
		}
		for i := 0; i < len(c.Args)-1; i++ {
			at, err := t.typeOf(c.Args[i])
			if err != nil {
				return "", err
			}
			if at != last && at != ir.Float {
				return "", diagnostic(CodeTypeMismatch, c.Span, "%s argument %d must be %s or float, got %s", c.Func, i+1, last, at)
			}
		}
		return last, nil
	default:
		return "", diagnostic(CodeInvalidCall, c.Span, "builtin %q has unsupported registry kind %q", c.Func, spec.kind)
	}
}

func (t *typer) sameOrScalarCall(name string, args []hir.Expr, n int, span hir.Span) (ir.Type, error) {
	if len(args) != n {
		return "", diagnostic(CodeInvalidCall, span, "%s expects %d arguments, got %d", name, n, len(args))
	}
	base, err := t.typeOf(args[0])
	if err != nil {
		return "", err
	}
	for i := 1; i < len(args); i++ {
		at, err := t.typeOf(args[i])
		if err != nil {
			return "", err
		}
		if at != base && at != ir.Float {
			return "", diagnostic(CodeTypeMismatch, span, "%s argument %d must be %s or float, got %s", name, i+1, base, at)
		}
	}
	return base, nil
}

func (t *typer) binaryType(b hir.Binary) (ir.Type, error) {
	if b.Op != "+" && b.Op != "-" && b.Op != "*" && b.Op != "/" {
		return "", diagnostic(CodeInvalidCall, b.Span, "unsupported operator %q", b.Op)
	}
	lt, err := t.typeOf(b.L)
	if err != nil {
		return "", err
	}
	rt, err := t.typeOf(b.R)
	if err != nil {
		return "", err
	}
	if lt == rt {
		return lt, nil
	}
	// matrix * vector -> vector
	if b.Op == "*" {
		if lt == ir.Mat4 && rt == ir.Vec4 {
			return ir.Vec4, nil
		}
		if lt == ir.Mat3 && rt == ir.Vec3 {
			return ir.Vec3, nil
		}
	}
	// scalar broadcasts to the other operand's type
	if lt == ir.Float {
		return rt, nil
	}
	if rt == ir.Float {
		return lt, nil
	}
	return "", diagnostic(CodeTypeMismatch, b.Span, "operator %s is not defined for %s and %s", b.Op, lt, rt)
}

// --- geo usage scan ---

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
	}
}

// --- small helpers ---

func toBindings(ns []bindings.NamedType) []ir.Binding {
	out := make([]ir.Binding, len(ns))
	for i, n := range ns {
		out[i] = ir.Binding{Name: n.Name, Type: n.Type}
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

func isVector(t ir.Type) bool {
	return t == ir.Vec2 || t == ir.Vec3 || t == ir.Vec4
}

func vectorWidth(t ir.Type) int {
	switch t {
	case ir.Vec2:
		return 2
	case ir.Vec3:
		return 3
	case ir.Vec4:
		return 4
	default:
		return 0
	}
}

// swizzleType infers the result type of a component swizzle (.rgb, .xy, .x)
// and validates that the selected components exist on the receiver.
func swizzleType(base ir.Type, field string) (ir.Type, error) {
	width := vectorWidth(base)
	if width == 0 {
		return "", fmt.Errorf("cannot swizzle %s with .%s", base, field)
	}
	var family byte
	for _, c := range field {
		var idx int
		var fam byte
		switch c {
		case 'x', 'y', 'z', 'w':
			fam = 'x'
			idx = strings.IndexRune("xyzw", c)
		case 'r', 'g', 'b', 'a':
			fam = 'r'
			idx = strings.IndexRune("rgba", c)
		default:
			return "", fmt.Errorf("invalid swizzle .%s for %s", field, base)
		}
		if family == 0 {
			family = fam
		} else if family != fam {
			return "", fmt.Errorf("invalid mixed swizzle .%s for %s", field, base)
		}
		if idx >= width {
			return "", fmt.Errorf("invalid swizzle .%s for %s", field, base)
		}
	}
	switch len(field) {
	case 1:
		return ir.Float, nil
	case 2:
		return ir.Vec2, nil
	case 3:
		return ir.Vec3, nil
	case 4:
		return ir.Vec4, nil
	}
	return "", fmt.Errorf("invalid swizzle .%s", field)
}
