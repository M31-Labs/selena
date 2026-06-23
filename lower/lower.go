package lower

import (
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
// host binding layout. Dispatches to the appropriate lowerer based on material kind.
func Lower(m hir.Material) (ir.Module, bindings.Layout, error) {
	switch m.Kind {
	case hir.KindPoints:
		return lowerPoints(m)
	case hir.KindPost:
		return lowerPost(m)
	default:
		return lowerMesh(m)
	}
}

// lowerMesh compiles a mesh material — the original Lower implementation.
// It synthesizes the default vertex transform and the fragment surface.
func lowerMesh(m hir.Material) (ir.Module, bindings.Layout, error) {
	if m.Vertex != nil {
		return ir.Module{}, bindings.Layout{}, diagnostic(
			CodeUnsupportedFeat,
			m.Vertex.Span,
			"vertex hooks are not supported yet; Selena currently emits the default transform",
		)
	}

	paramKind, err := validateParams(m.Params)
	if err != nil {
		return ir.Module{}, bindings.Layout{}, err
	}
	uniforms, err := buildUniformPlan(m.Params)
	if err != nil {
		return ir.Module{}, bindings.Layout{}, err
	}
	interfaces, err := buildInterfacePlan(m.Surface)
	if err != nil {
		return ir.Module{}, bindings.Layout{}, err
	}
	reserved, err := interfaceNames(uniforms.uniforms, uniforms.textures, interfaces.attributes, interfaces.varyings)
	if err != nil {
		return ir.Module{}, bindings.Layout{}, err
	}

	rs := &resolver{paramKind: paramKind, uniformOf: uniforms.uniformOf, varyingOf: interfaces.varyingOf, geo: m.Surface.Geo}
	tp := &typer{paramKind: paramKind, geo: m.Surface.Geo, locals: map[string]ir.Type{}}

	fragBody, fragOut, err := lowerFragment(m.Surface, reserved, rs, tp)
	if err != nil {
		return ir.Module{}, bindings.Layout{}, err
	}

	vertexOut := ir.Binary{Op: "*", L: ir.Ref{Name: "mvp"},
		R: ir.Construct{Type: ir.Vec4, Args: []ir.Expr{ir.Ref{Name: "position"}, ir.Lit{Value: 1.0}}}}

	irTextures := make([]ir.Texture, len(uniforms.textures))
	for i, t := range uniforms.textures {
		irTextures[i] = ir.Texture{Name: t}
	}

	mod := ir.Module{
		Name:       m.Name,
		Kind:       ir.KindMesh,
		Uniforms:   toBindings(uniforms.uniforms),
		Attributes: interfaces.attributes,
		Varyings:   interfaces.varyings,
		Textures:   irTextures,
		Vertex:     ir.Stage{Body: interfaces.vertexBody, Output: vertexOut},
		Fragment:   ir.Stage{Body: fragBody, Output: fragOut},
	}
	layout := bindings.Layout{
		SchemaVersion:   bindings.DescriptorSchemaVersion,
		LanguageVersion: bindings.LanguageVersion,
		Material:        m.Name,
		Kind:            bindings.SurfaceKindMesh,
		EntryPoints:     bindings.EntryPoints{Vertex: "vertexMain", Fragment: "fragmentMain"},
		UniformBlock:    bindings.ComputeUniformBlock(uniforms.uniforms),
		Attributes:      toAttrs(interfaces.attributes),
		Textures:        bindings.ComputeTextures(uniforms.textures),
		WGSL:            bindings.WGSLBinding{Group: 0, Binding: 0},
		Metal:           bindings.MetalBinding{Buffer: 0},
	}
	if layout.Textures == nil {
		layout.Textures = []bindings.Texture{}
	}
	layout.UniformBlock.Defaults = uniforms.defaults
	return mod, layout, nil
}

// lowerPoints compiles a points/particle appearance material.
// The author writes a surface function that receives point geometry inputs
// (pointUV, color, alpha, pointSize, fogFactor) and returns vec4 rgba.
// The emitter generates the full billboard quad scaffold around it.
func lowerPoints(m hir.Material) (ir.Module, bindings.Layout, error) {
	if m.Vertex != nil {
		return ir.Module{}, bindings.Layout{}, diagnostic(
			CodeUnsupportedFeat, m.Vertex.Span,
			"vertex hooks are not supported in points materials",
		)
	}

	// Only scalar/vector/color params are allowed for points (no Sun, no texture2d).
	paramKind := map[string]hir.Type{}
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
		switch p.Type {
		case hir.Texture2D:
			return ir.Module{}, bindings.Layout{}, diagnostic(
				CodeUnsupportedType, p.Span,
				"param %q: texture2d params are not supported in points materials", p.Name,
			)
		case hir.Sun:
			return ir.Module{}, bindings.Layout{}, diagnostic(
				CodeUnsupportedType, p.Span,
				"param %q: Sun record params are not supported in points materials", p.Name,
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

	// Build the varyings resolver from point geometry fields.
	varyingOf := map[string]string{}
	var varyings []ir.Binding
	for _, name := range []string{"pointUV", "color", "alpha", "pointSize", "fogFactor"} {
		spec := pointGeometry[name]
		varyingOf[name] = spec.varying
		varyings = append(varyings, ir.Binding{Name: spec.varying, Type: spec.typ})
	}

	rs := &resolver{
		paramKind:  paramKind,
		uniformOf:  uniformOf,
		varyingOf:  varyingOf,
		geo:        m.Surface.Geo,
		geoFields:  pointGeometry,
	}
	tp := &typer{
		paramKind: paramKind,
		geo:       m.Surface.Geo,
		locals:    map[string]ir.Type{},
		geoFields: pointGeometry,
	}

	uniforms := uniformPlan{uniforms: uniformsList, defaults: defaults, uniformOf: uniformOf}
	reserved, err := interfaceNames(uniforms.uniforms, nil, nil, varyings)
	if err != nil {
		return ir.Module{}, bindings.Layout{}, err
	}

	fragBody, fragOut, err := lowerFragment(m.Surface, reserved, rs, tp)
	if err != nil {
		return ir.Module{}, bindings.Layout{}, err
	}

	mod := ir.Module{
		Name:     m.Name,
		Kind:     ir.KindPoints,
		Uniforms: toBindings(uniformsList),
		Varyings: varyings,
		Fragment: ir.Stage{Body: fragBody, Output: fragOut},
	}

	ub := bindings.ComputeUniformBlock(uniformsList)
	ub.Defaults = defaults
	// Points WGSL emits two vertex entry points:
	//   vertexMain        — reads a_position/a_size/a_color attributes (static layers)
	//   vertexStorageMain — reads from storage buffer particles[] (particle render)
	// GLSL/GLES emit only the attribute form.
	layout := bindings.Layout{
		SchemaVersion: bindings.DescriptorSchemaVersion,
		LanguageVersion: bindings.LanguageVersion,
		Material:        m.Name,
		Kind:            bindings.SurfaceKindPoints,
		EntryPoints: bindings.EntryPoints{
			Vertex:        "vertexMain",
			Fragment:      "fragmentMain",
			VertexStorage: "vertexStorageMain",
		},
		UniformBlock: ub,
		Attributes:   []bindings.Attribute{},
		Textures:     []bindings.Texture{},
		WGSL:         bindings.WGSLBinding{Group: 1, Binding: 0},
		Metal:        bindings.MetalBinding{Buffer: 1},
	}
	return mod, layout, nil
}

// lowerPost compiles a fullscreen post-process pass material.
// The author writes a surface function that receives uv and may call
// sceneColor(uv) / sceneDepth(uv). Returns vec4 rgba.
func lowerPost(m hir.Material) (ir.Module, bindings.Layout, error) {
	if m.Vertex != nil {
		return ir.Module{}, bindings.Layout{}, diagnostic(
			CodeUnsupportedFeat, m.Vertex.Span,
			"vertex hooks are not supported in post materials",
		)
	}

	// Only scalar/vector/color params; no textures, no Sun, in post materials.
	// (User textures are not currently supported; engine provides sceneColor/sceneDepth.)
	paramKind := map[string]hir.Type{}
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
		switch p.Type {
		case hir.Texture2D:
			return ir.Module{}, bindings.Layout{}, diagnostic(
				CodeUnsupportedType, p.Span,
				"param %q: texture2d params are not supported in post materials (use sceneColor(uv) instead)", p.Name,
			)
		case hir.Sun:
			return ir.Module{}, bindings.Layout{}, diagnostic(
				CodeUnsupportedType, p.Span,
				"param %q: Sun record params are not supported in post materials", p.Name,
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

	// Post surface only exposes "uv" as a geo field.
	varyingOf := map[string]string{
		"uv": postGeometry["uv"].varying,
	}
	varyings := []ir.Binding{
		{Name: postGeometry["uv"].varying, Type: postGeometry["uv"].typ},
	}

	rs := &resolver{
		paramKind:       paramKind,
		uniformOf:       uniformOf,
		varyingOf:       varyingOf,
		geo:             m.Surface.Geo,
		geoFields:       postGeometry,
		allowSceneSample: true,
	}
	tp := &typer{
		paramKind:       paramKind,
		geo:             m.Surface.Geo,
		locals:          map[string]ir.Type{},
		geoFields:       postGeometry,
		allowSceneSample: true,
	}

	uniforms := uniformPlan{uniforms: uniformsList, defaults: defaults, uniformOf: uniformOf}
	reserved, err := interfaceNames(uniforms.uniforms, nil, nil, varyings)
	if err != nil {
		return ir.Module{}, bindings.Layout{}, err
	}

	fragBody, fragOut, err := lowerFragment(m.Surface, reserved, rs, tp)
	if err != nil {
		return ir.Module{}, bindings.Layout{}, err
	}

	mod := ir.Module{
		Name:     m.Name,
		Kind:     ir.KindPost,
		Uniforms: toBindings(uniformsList),
		Varyings: varyings,
		Fragment: ir.Stage{Body: fragBody, Output: fragOut},
	}

	// For post passes: engine scene textures live at @group(0) @binding(0/1/2/3).
	// User uniforms start at @group(0) @binding(4) (WGSL) / buffer(1) (Metal).
	ub := bindings.ComputeUniformBlock(uniformsList)
	ub.Defaults = defaults

	// Post user-UBO slot is always @group(0) @binding(4) / Metal buffer(1),
	// regardless of whether the author declared any params. The {0,0} default
	// would collide with the mesh group-0/binding-0 convention and mislead hosts
	// that inspect the Layout without checking Kind. An explicit HasUniforms
	// field is unnecessary because the host can test UniformBlock.Size > 0.
	wgslBinding := bindings.WGSLBinding{Group: 0, Binding: 4}
	metalBinding := bindings.MetalBinding{Buffer: 1}

	layout := bindings.Layout{
		SchemaVersion:   bindings.DescriptorSchemaVersion,
		LanguageVersion: bindings.LanguageVersion,
		Material:        m.Name,
		Kind:            bindings.SurfaceKindPost,
		EntryPoints:     bindings.EntryPoints{Vertex: "vertexMain", Fragment: "fragmentMain"},
		UniformBlock:    ub,
		Attributes:      []bindings.Attribute{},
		Textures:        []bindings.Texture{},
		WGSL:            wgslBinding,
		Metal:           metalBinding,
	}
	return mod, layout, nil
}

