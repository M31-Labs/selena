package lower

import (
	"strings"

	"m31labs.dev/selena/bindings"
	"m31labs.dev/selena/hir"
	"m31labs.dev/selena/ir"
)

// Screen materials (`kind screen`): a fullscreen pass that returns a color with
// alpha, with author textures and arrays, and a render block that declares the
// blend and depth state the host applies. The rules live here so the shared
// lowerer, typer and resolver only carry small hooks.

// CodeInvalidRender reports a bad render block: an unknown key or word, a
// repeated key, or a render block on a kind that has no pipeline state.
const CodeInvalidRender = "SEL1008"

// screenGeometry lists the fields surface(screen) can read. uv is 0..1 across
// the target rectangle with the origin at the top left on every target, so
// load(tex, int(uv.x*w), int(uv.y*h)) reads texture row 0 at the top.
var screenGeometry = map[string]geometrySpec{
	"uv": {
		typ:     ir.Vec2,
		varying: "v_uv",
		build:   func() ir.Expr { return ir.Ref{Name: "v_uv"} },
	},
}

// checkRenderBlock rejects a render block on a material that is not a screen
// material.
func checkRenderBlock(m hir.Material) error {
	if m.Render == nil || m.Kind == hir.KindScreen {
		return nil
	}
	kind := m.Kind
	if kind == "" {
		kind = hir.KindMesh
	}
	return diagnostic(CodeInvalidRender, m.Render.Span,
		"a render block is supported only by screen materials; %s materials take their blend and depth state from the host (write `material %s kind screen` to declare it)", kind, m.Name)
}

// lowerRender validates a screen material's render block and returns the
// pipeline state it declares. Without a block the defaults are blend alpha and
// depth off.
func lowerRender(rb *hir.RenderBlock) (bindings.PipelineState, error) {
	blend, depth := bindings.BlendAlpha, bindings.DepthOff
	if rb == nil {
		return bindings.PipelineFor(blend, depth), nil
	}
	seen := map[string]bool{}
	for _, s := range rb.Settings {
		if seen[s.Key] {
			return bindings.PipelineState{}, diagnostic(CodeInvalidRender, s.KeySpan, "render: %q is set more than once", s.Key)
		}
		seen[s.Key] = true
		switch s.Key {
		case "blend":
			if !contains(bindings.BlendModes, s.Value) {
				return bindings.PipelineState{}, diagnostic(CodeInvalidRender, s.ValueSpan,
					"render: unknown blend %q (expected %s)", s.Value, strings.Join(bindings.BlendModes, ", "))
			}
			blend = s.Value
		case "depth":
			if !contains(bindings.DepthModes, s.Value) {
				return bindings.PipelineState{}, diagnostic(CodeInvalidRender, s.ValueSpan,
					"render: unknown depth %q (expected %s)", s.Value, strings.Join(bindings.DepthModes, ", "))
			}
			depth = s.Value
		default:
			return bindings.PipelineState{}, diagnostic(CodeInvalidRender, s.KeySpan,
				"render: unknown setting %q (expected blend or depth)", s.Key)
		}
	}
	return bindings.PipelineFor(blend, depth), nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// typeScreenSize types screenSize() -> vec2.
func (t *typer) typeScreenSize(c hir.Call) (ir.Type, error) {
	if !t.allowScreenSize {
		return "", diagnostic(CodeInvalidCall, c.Span, "screenSize is only available in screen-kind materials")
	}
	if len(c.Args) != 0 {
		return "", diagnostic(CodeInvalidCall, c.Span, "screenSize() takes no arguments")
	}
	return ir.Vec2, nil
}

// resolveScreenSize lowers screenSize() to a read of the host-written uniform
// bindings.ScreenSizeUniform. lowerScreen declares the uniform when the body
// reads it.
func (r *resolver) resolveScreenSize(x hir.Call) (ir.Expr, error) {
	if !r.allowScreenSize {
		return nil, diagnostic(CodeInvalidCall, x.Span, "screenSize is only available in screen-kind materials")
	}
	if len(x.Args) != 0 {
		return nil, diagnostic(CodeInvalidCall, x.Span, "screenSize() takes no arguments")
	}
	return ir.Ref{Name: bindings.ScreenSizeUniform}, nil
}

// lowerScreen compiles a screen material.
func lowerScreen(m hir.Material) (ir.Module, bindings.Layout, error) {
	fail := func(err error) (ir.Module, bindings.Layout, error) {
		return ir.Module{}, bindings.Layout{}, err
	}
	if m.Vertex != nil {
		return fail(diagnostic(CodeUnsupportedFeat, m.Vertex.Span, "vertex hooks are not supported in screen materials; Selena draws a fullscreen triangle and the host places it with the viewport and scissor"))
	}
	if len(m.Varyings) > 0 {
		return fail(diagnostic(CodeUnsupportedFeat, m.Varyings[0].Span, "varying %q requires a vertex() stage, which screen materials do not have", m.Varyings[0].Name))
	}
	if len(m.States) > 0 {
		return fail(diagnostic(CodeUnsupportedFeat, m.States[0].Span, "statefields are not supported in screen materials"))
	}
	pipeline, err := lowerRender(m.Render)
	if err != nil {
		return fail(err)
	}

	paramKind := map[string]hir.Type{}
	paramArrays := map[string]arrayInfo{}
	uniformOf := map[string]string{}
	var uniformsList []bindings.NamedType
	var defaults []bindings.DefaultValue
	var textures []string
	seen := map[string]bool{}

	for _, p := range m.Params {
		if err := validateAuthorName("param", p.Name, p.Span); err != nil {
			return fail(err)
		}
		if p.Name == bindings.ScreenSizeUniform {
			return fail(diagnostic(CodeReservedName, p.Span, "param %q is reserved for the target size the host writes; read it with screenSize()", p.Name))
		}
		if seen[p.Name] {
			return fail(diagnostic(CodeDuplicateParam, p.Span, "duplicate param %q", p.Name))
		}
		seen[p.Name] = true
		if p.IsArray {
			if p.Default != nil {
				return fail(diagnostic(CodeInvalidDefault, hir.ExprSpan(p.Default), "param %q: defaults for array params are not supported", p.Name))
			}
			elemType, ok := hirToIRType(p.Type)
			if !ok {
				return fail(diagnostic(CodeUnsupportedType, p.Span, "param %q: unsupported array element type %q", p.Name, p.Type))
			}
			if p.ArraySize <= 0 {
				return fail(diagnostic(CodeUnsupportedType, p.Span, "param %q: array size must be positive", p.Name))
			}
			paramArrays[p.Name] = arrayInfo{elemType: elemType, size: p.ArraySize}
			uniformOf[p.Name] = p.Name
			uniformsList = append(uniformsList, bindings.NamedType{Name: p.Name, Type: elemType, Count: p.ArraySize})
			continue
		}
		switch p.Type {
		case hir.Texture2D:
			if p.Default != nil {
				return fail(diagnostic(CodeInvalidDefault, hir.ExprSpan(p.Default), "param %q: defaults for texture2d are not supported", p.Name))
			}
			paramKind[p.Name] = p.Type
			textures = append(textures, p.Name)
		case hir.TextureCube:
			return fail(diagnostic(CodeUnsupportedType, p.Span, "param %q: textureCube params are not supported in screen materials", p.Name))
		case hir.Sun:
			return fail(diagnostic(CodeUnsupportedType, p.Span, "param %q: Sun record params are not supported in screen materials", p.Name))
		default:
			t, ok := hirToIRType(p.Type)
			if !ok {
				return fail(diagnostic(CodeUnsupportedType, p.Span, "param %q: unsupported type %q", p.Name, p.Type))
			}
			paramKind[p.Name] = p.Type
			uniformsList = append(uniformsList, bindings.NamedType{Name: p.Name, Type: t})
			uniformOf[p.Name] = p.Name
			if p.Default != nil {
				vals, err := constDefault(p.Default, p.Type)
				if err != nil {
					return fail(diagnostic(CodeInvalidDefault, hir.ExprSpan(p.Default), "param %q default: %v", p.Name, err))
				}
				defaults = append(defaults, bindings.DefaultValue{Name: p.Name, Type: string(t), Values: vals})
			}
		}
	}

	for _, c := range m.Context {
		if c.Name == bindings.ScreenSizeUniform {
			return fail(diagnostic(CodeReservedName, c.Span, "context %q is reserved for the target size the host writes; read it with screenSize()", c.Name))
		}
	}
	for k := range paramKind {
		seen[k] = true
	}
	for k := range paramArrays {
		seen[k] = true
	}
	contextNames, err := lowerContextFields(m.Context, seen, paramKind, paramArrays, uniformOf, &uniformsList, true)
	if err != nil {
		return fail(err)
	}

	varyings := []ir.Binding{{Name: screenGeometry["uv"].varying, Type: screenGeometry["uv"].typ}}
	rs := &resolver{
		paramKind:       paramKind,
		uniformOf:       uniformOf,
		varyingOf:       map[string]string{"uv": screenGeometry["uv"].varying},
		geo:             m.Surface.Geo,
		geoFields:       screenGeometry,
		allowDiscard:    true,
		allowScreenSize: true,
	}
	tp := &typer{
		paramKind:       paramKind,
		geo:             m.Surface.Geo,
		locals:          map[string]ir.Type{},
		mutableLocals:   map[string]bool{},
		geoFields:       screenGeometry,
		paramArrays:     paramArrays,
		allowScreenSize: true,
	}
	rs.tp = tp

	reserved, err := interfaceNames(uniformsList, textures, nil, varyings)
	if err != nil {
		return fail(err)
	}
	fragBody, fragOut, err := lowerFragment(m.Surface, reserved, rs, tp)
	if err != nil {
		return fail(err)
	}

	// screenSize() reads a host-written uniform, declared only when used.
	frag := ir.Stage{Body: fragBody, Output: fragOut}
	if ir.StageUsesRef(frag, bindings.ScreenSizeUniform) {
		uniformsList = append(uniformsList, bindings.NamedType{Name: bindings.ScreenSizeUniform, Type: ir.Vec2})
		contextNames = append(contextNames, bindings.ScreenSizeUniform)
	}

	plan := uniformPlan{textures: textures}
	mod := ir.Module{
		Name:          m.Name,
		Kind:          ir.KindScreen,
		Uniforms:      toBindings(uniformsList),
		ArrayUniforms: toArrayBindings(uniformsList),
		Varyings:      varyings,
		Textures:      planIRTextures(plan),
		Fragment:      frag,
	}
	ub := bindings.ComputeUniformBlock(uniformsList)
	ub.Defaults = defaults
	stampContextClass(ub.Fields, contextNames)
	layoutTextures := planLayoutTextures(plan)
	if layoutTextures == nil {
		layoutTextures = []bindings.Texture{}
	}
	pipelineCopy := pipeline
	layout := bindings.Layout{
		SchemaVersion:   bindings.DescriptorSchemaVersion,
		LanguageVersion: bindings.LanguageVersion,
		Material:        m.Name,
		Kind:            bindings.SurfaceKindScreen,
		EntryPoints:     bindings.EntryPoints{Vertex: "vertexMain", Fragment: "fragmentMain"},
		UniformBlock:    ub,
		Attributes:      []bindings.Attribute{},
		Textures:        layoutTextures,
		WGSL:            bindings.WGSLBinding{Group: 0, Binding: 0},
		Metal:           bindings.MetalBinding{Buffer: 0},
		Context:         contextNames,
		Pipeline:        &pipelineCopy,
	}
	return mod, layout, nil
}
