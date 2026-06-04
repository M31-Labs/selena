package lower

import (
	"fmt"

	"m31labs.dev/selena/bindings"
	"m31labs.dev/selena/hir"
	"m31labs.dev/selena/ir"
)

type uniformPlan struct {
	uniforms  []bindings.NamedType
	defaults  []bindings.DefaultValue
	uniformOf map[string]string
	textures  []string
}

type interfacePlan struct {
	attributes []ir.Binding
	varyings   []ir.Binding
	varyingOf  map[string]string
	vertexBody []ir.Stmt
}

func validateParams(params []hir.Param) (map[string]hir.Type, error) {
	paramKind := map[string]hir.Type{}
	for _, p := range params {
		if err := validateAuthorName("param", p.Name, p.Span); err != nil {
			return nil, err
		}
		if _, ok := paramKind[p.Name]; ok {
			return nil, diagnostic(CodeDuplicateParam, p.Span, "duplicate param %q", p.Name)
		}
		paramKind[p.Name] = p.Type
	}
	return paramKind, nil
}

func buildUniformPlan(params []hir.Param) (uniformPlan, error) {
	plan := uniformPlan{uniformOf: map[string]string{}}
	uniType := map[string]ir.Type{}
	addUniform := func(name string, t ir.Type) error {
		if _, ok := uniType[name]; ok {
			return fmt.Errorf("uniform %q is declared more than once", name)
		}
		plan.uniforms = append(plan.uniforms, bindings.NamedType{Name: name, Type: t})
		uniType[name] = t
		return nil
	}
	if err := addUniform("mvp", ir.Mat4); err != nil {
		return uniformPlan{}, err
	}
	if err := addUniform("normalMatrix", ir.Mat3); err != nil {
		return uniformPlan{}, err
	}
	for _, p := range params {
		switch p.Type {
		case hir.Sun:
			sunFields, err := sunDefaultFields(p)
			if err != nil {
				return uniformPlan{}, err
			}
			for _, f := range sortedKeys(stdlib.recordFields(string(hir.Sun))) {
				un := p.Name + "_" + f
				ft, _ := stdlib.recordField(string(hir.Sun), f)
				if err := addUniform(un, ft); err != nil {
					return uniformPlan{}, fmt.Errorf("param %q: %w", p.Name, err)
				}
				plan.uniformOf[p.Name+"."+f] = un
				if sunFields != nil {
					plan.defaults = append(plan.defaults, bindings.DefaultValue{Name: un, Type: string(ft), Values: sunFields[f]})
				}
			}
		case hir.Texture2D:
			if p.Default != nil {
				return uniformPlan{}, diagnostic(CodeInvalidDefault, hir.ExprSpan(p.Default), "param %q: defaults for texture2d are not supported", p.Name)
			}
			plan.textures = append(plan.textures, p.Name)
		default:
			t, ok := hirToIRType(p.Type)
			if !ok {
				return uniformPlan{}, diagnostic(CodeUnsupportedType, p.Span, "param %q: unsupported type %q", p.Name, p.Type)
			}
			if err := addUniform(p.Name, t); err != nil {
				return uniformPlan{}, fmt.Errorf("param %q: %w", p.Name, err)
			}
			plan.uniformOf[p.Name] = p.Name
			if p.Default != nil {
				vals, err := constDefault(p.Default, p.Type)
				if err != nil {
					return uniformPlan{}, diagnostic(CodeInvalidDefault, hir.ExprSpan(p.Default), "param %q default: %v", p.Name, err)
				}
				plan.defaults = append(plan.defaults, bindings.DefaultValue{Name: p.Name, Type: string(t), Values: vals})
			}
		}
	}
	return plan, nil
}

func sunDefaultFields(p hir.Param) (map[string][]float32, error) {
	if p.Default == nil {
		return nil, nil
	}
	fields, err := sunDefault(p.Default)
	if err != nil {
		return nil, diagnostic(CodeInvalidDefault, hir.ExprSpan(p.Default), "param %q default: %v", p.Name, err)
	}
	return fields, nil
}

func buildInterfacePlan(surface hir.Func) (interfacePlan, error) {
	usedGeo := map[string]bool{}
	collectGeo(surface.Result, surface.Geo, usedGeo)
	for _, l := range surface.Body {
		collectGeo(l.Value, surface.Geo, usedGeo)
	}

	attrNeeded := map[string]bool{"position": true}
	plan := interfacePlan{varyingOf: map[string]string{}}
	for _, f := range sortedSet(usedGeo) {
		gf, ok := stdlib.geometryField(f)
		if !ok {
			return interfacePlan{}, fmt.Errorf("unknown geometry field geo.%s", f)
		}
		for _, a := range gf.attrs {
			attrNeeded[a] = true
		}
		plan.varyingOf[f] = gf.varying
		plan.varyings = append(plan.varyings, ir.Binding{Name: gf.varying, Type: gf.typ})
		plan.vertexBody = append(plan.vertexBody, ir.Stmt{Target: gf.varying, Type: gf.typ, Value: gf.build()})
	}

	position, _ := stdlib.geometryField("position")
	plan.attributes = []ir.Binding{{Name: "position", Type: position.typ}}
	for _, a := range sortedSet(attrNeeded) {
		if a == "position" {
			continue
		}
		gf, ok := stdlib.geometryField(a)
		if !ok {
			return interfacePlan{}, fmt.Errorf("attribute %q is not registered", a)
		}
		plan.attributes = append(plan.attributes, ir.Binding{Name: a, Type: gf.typ})
	}
	return plan, nil
}

func lowerFragment(surface hir.Func, reserved map[string]string, rs *resolver, tp *typer) ([]ir.Stmt, ir.Expr, error) {
	var fragBody []ir.Stmt
	seenLocals := map[string]bool{}
	for _, l := range surface.Body {
		if err := validateAuthorName("surface local", l.Name, l.Span); err != nil {
			return nil, nil, err
		}
		if kind, ok := reserved[l.Name]; ok {
			return nil, nil, diagnostic(CodeDuplicateLocal, l.Span, "surface local %q conflicts with %s", l.Name, kind)
		}
		if seenLocals[l.Name] {
			return nil, nil, diagnostic(CodeDuplicateLocal, l.Span, "duplicate surface local %q", l.Name)
		}
		t, err := tp.typeOf(l.Value)
		if err != nil {
			return nil, nil, fmt.Errorf("surface local %q: %w", l.Name, err)
		}
		ve, err := rs.expr(l.Value)
		if err != nil {
			return nil, nil, fmt.Errorf("surface local %q: %w", l.Name, err)
		}
		seenLocals[l.Name] = true
		tp.locals[l.Name] = t
		fragBody = append(fragBody, ir.Stmt{Target: l.Name, Type: t, Value: ve})
	}

	rt, err := tp.typeOf(surface.Result)
	if err != nil {
		return nil, nil, fmt.Errorf("surface result: %w", err)
	}
	re, err := rs.expr(surface.Result)
	if err != nil {
		return nil, nil, fmt.Errorf("surface result: %w", err)
	}
	switch rt {
	case ir.Vec4:
		return fragBody, re, nil
	case ir.Vec3:
		return fragBody, ir.Construct{Type: ir.Vec4, Args: []ir.Expr{re, ir.Lit{Value: 1.0}}}, nil
	default:
		return nil, nil, fmt.Errorf("surface must return color/vec3/vec4, got %s", rt)
	}
}
