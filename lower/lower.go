package lower

import (
	"fmt"

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
