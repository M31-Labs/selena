package lower

import (
	"m31labs.dev/selena/bindings"
	"m31labs.dev/selena/hir"
	"m31labs.dev/selena/ir"
)

// lowerFeedback compiles a feedback-kind simulation step. The author writes a
// single feedback(cell) -> vec4 entry that returns the next state for the
// current cell, reading the previous state via state(dx, dy). Because the body
// is a pure per-output-cell GATHER, it lowers to one ir.Module that BOTH the
// WGSL/Metal compute emitters (writing outState[cellIndex]) and the GLSL/GLES
// fullscreen-fragment emitters (writing the stage output, sampling stateTex)
// can target.
//
// Binding model (kind-injected; see bindings.GridBinding/StateField):
//   - WGSL:  @group(0) @binding(0) grid {gridWidth,gridLen}; @binding(1) inState
//            (storage,read); @binding(2) outState (storage,read_write);
//            @binding(3) user uniforms.
//   - Metal: inState [[buffer(0)]], outState [[buffer(1)]], grid [[buffer(2)]],
//            user uniforms [[buffer(3)]].
//   - GL:    uniform sampler2D stateTex (unit 0); uniform vec2 texelSize.
func lowerFeedback(m hir.Material) (ir.Module, bindings.Layout, error) {
	if m.Vertex != nil {
		return ir.Module{}, bindings.Layout{}, diagnostic(
			CodeUnsupportedFeat, m.Vertex.Span,
			"vertex hooks are not supported in feedback materials",
		)
	}

	// Exactly one statefield: the grid the state(dx, dy) read addresses.
	if len(m.States) == 0 {
		return ir.Module{}, bindings.Layout{}, diagnostic(
			CodeUnsupportedFeat, m.Span,
			"feedback material %q declares no statefield (add `state <name>`)", m.Name,
		)
	}
	if len(m.States) > 1 {
		return ir.Module{}, bindings.Layout{}, diagnostic(
			CodeUnsupportedFeat, m.States[1].Span,
			"feedback material %q declares more than one statefield; exactly one is supported", m.Name,
		)
	}
	stateName := m.States[0].Name
	if err := validateAuthorName("statefield", stateName, m.States[0].Span); err != nil {
		return ir.Module{}, bindings.Layout{}, err
	}

	// Params: scalar/vector/color and fixed-size array params are allowed; no Sun
	// records, no texture2d — the compute/fragment dispatch only carries a small
	// user uniform block.
	paramKind := map[string]hir.Type{}
	paramArrays := map[string]arrayInfo{}
	var uniformsList []bindings.NamedType
	var defaults []bindings.DefaultValue
	uniformOf := map[string]string{}

	for _, p := range m.Params {
		if err := validateAuthorName("param", p.Name, p.Span); err != nil {
			return ir.Module{}, bindings.Layout{}, err
		}
		if _, dup := paramKind[p.Name]; dup {
			return ir.Module{}, bindings.Layout{}, diagnostic(CodeDuplicateParam, p.Span, "duplicate param %q", p.Name)
		}
		if _, dup := paramArrays[p.Name]; dup {
			return ir.Module{}, bindings.Layout{}, diagnostic(CodeDuplicateParam, p.Span, "duplicate param %q", p.Name)
		}
		if p.IsArray {
			if p.Default != nil {
				return ir.Module{}, bindings.Layout{}, diagnostic(CodeInvalidDefault, hir.ExprSpan(p.Default), "param %q: defaults for array params are not supported", p.Name)
			}
			elemType, ok := hirToIRType(p.Type)
			if !ok {
				return ir.Module{}, bindings.Layout{}, diagnostic(CodeUnsupportedType, p.Span, "param %q: unsupported array element type %q", p.Name, p.Type)
			}
			if p.ArraySize <= 0 {
				return ir.Module{}, bindings.Layout{}, diagnostic(CodeUnsupportedType, p.Span, "param %q: array size must be positive", p.Name)
			}
			paramArrays[p.Name] = arrayInfo{elemType: elemType, size: p.ArraySize}
			uniformsList = append(uniformsList, bindings.NamedType{Name: p.Name, Type: elemType, Count: p.ArraySize})
			continue
		}
		switch p.Type {
		case hir.Texture2D:
			return ir.Module{}, bindings.Layout{}, diagnostic(
				CodeUnsupportedType, p.Span,
				"param %q: texture2d params are not supported in feedback materials", p.Name,
			)
		case hir.Sun:
			return ir.Module{}, bindings.Layout{}, diagnostic(
				CodeUnsupportedType, p.Span,
				"param %q: Sun record params are not supported in feedback materials", p.Name,
			)
		default:
			t, ok := hirToIRType(p.Type)
			if !ok {
				return ir.Module{}, bindings.Layout{}, diagnostic(CodeUnsupportedType, p.Span, "param %q: unsupported type %q", p.Name, p.Type)
			}
			paramKind[p.Name] = p.Type
			uniformsList = append(uniformsList, bindings.NamedType{Name: p.Name, Type: t})
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

	// Context fields (§3.3 C6): feedback already wires ArrayUniforms for array
	// params, so array context fields are allowed here too.
	seen := map[string]bool{}
	for k := range paramKind {
		seen[k] = true
	}
	for k := range paramArrays {
		seen[k] = true
	}
	contextNames, err := lowerContextFields(m.Context, seen, paramKind, paramArrays, uniformOf, &uniformsList, true)
	if err != nil {
		return ir.Module{}, bindings.Layout{}, err
	}

	// The feedback body exposes cell.uv (the cell's normalized [0,1]×[0,1] grid
	// coordinate) via ir.CellUV{}, which each backend renders differently.
	// All other cell.* accesses are rejected (geoFields has only "uv").
	rs := &resolver{
		paramKind:  paramKind,
		uniformOf:  uniformOf,
		varyingOf:  map[string]string{},
		geo:        m.Surface.Geo,
		geoFields:  map[string]geometrySpec{"uv": {typ: ir.Vec2}},
		geoBuild:   map[string]func() ir.Expr{"uv": func() ir.Expr { return ir.CellUV{} }},
		allowState: true,
	}
	tp := &typer{
		paramKind:     paramKind,
		geo:           m.Surface.Geo,
		locals:        map[string]ir.Type{},
		mutableLocals: map[string]bool{},
		geoFields:     map[string]geometrySpec{"uv": {typ: ir.Vec2}},
		paramArrays:   paramArrays,
		allowState:    true,
	}
	rs.tp = tp

	reserved, err := interfaceNames(uniformsList, nil, nil, nil)
	if err != nil {
		return ir.Module{}, bindings.Layout{}, err
	}

	fragBody, fragOut, err := lowerFragment(m.Surface, reserved, rs, tp)
	if err != nil {
		return ir.Module{}, bindings.Layout{}, err
	}

	mod := ir.Module{
		Name:          m.Name,
		Kind:          ir.KindFeedback,
		Uniforms:      toBindings(uniformsList),
		ArrayUniforms: toArrayBindings(uniformsList),
		StateField:    stateName,
		Fragment:      ir.Stage{Body: fragBody, Output: fragOut},
	}

	ub := bindings.ComputeUniformBlock(uniformsList)
	ub.Defaults = defaults
	stampContextClass(ub.Fields, contextNames)

	layout := bindings.Layout{
		SchemaVersion:   bindings.DescriptorSchemaVersion,
		LanguageVersion: bindings.LanguageVersion,
		Material:        m.Name,
		Kind:            bindings.SurfaceKindFeedback,
		EntryPoints: bindings.EntryPoints{
			Vertex:   "vertexMain",
			Fragment: "fragmentMain",
			Compute:  "computeMain",
		},
		UniformBlock: ub,
		Attributes:   []bindings.Attribute{},
		Textures:     []bindings.Texture{},
		// User uniform block: WGSL @group(0)@binding(3); Metal [[buffer(3)]].
		WGSL:  bindings.WGSLBinding{Group: 0, Binding: 3},
		Metal: bindings.MetalBinding{Buffer: 3},
		States: []bindings.StateField{{
			Name:  stateName,
			WGSL:  bindings.WGSLStateBinding{Group: 0, InBinding: 1, OutBinding: 2, InKind: "storage"},
			GL:    bindings.GLStateBinding{Uniform: "stateTex", Unit: 0},
			Metal: bindings.MetalStateBinding{InBuffer: 0, OutBuffer: 1},
		}},
		Grid: &bindings.GridBinding{
			WGSL:           bindings.WGSLBinding{Group: 0, Binding: 0},
			Metal:          bindings.MetalBinding{Buffer: 2},
			GLTexelUniform: "texelSize",
		},
		Context: contextNames,
	}
	return mod, layout, nil
}
