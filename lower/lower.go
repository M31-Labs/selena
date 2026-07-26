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
	case hir.Int:
		return ir.Int, true
	case hir.Uint:
		return ir.Uint, true
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
// host binding layout. Dispatches to the appropriate lowerer based on material
// kind, then stamps the host requirements the emitted shaders imply.
func Lower(m hir.Material) (ir.Module, bindings.Layout, error) {
	mod, layout, err := lowerByKind(m)
	if err != nil {
		return ir.Module{}, bindings.Layout{}, err
	}
	layout.Requires = hostRequirements(mod)
	return mod, layout, nil
}

func lowerByKind(m hir.Material) (ir.Module, bindings.Layout, error) {
	switch m.Kind {
	case hir.KindPoints:
		return lowerPoints(m)
	case hir.KindPost:
		return lowerPost(m)
	case hir.KindFeedback:
		return lowerFeedback(m)
	default:
		return lowerMesh(m)
	}
}

// hostRequirements reports what the host must arrange for mod's emitted shaders
// to behave as authored. Selena emits the shader call; only the host can enable
// a WebGL extension, build a mip chain, or measure a render target — and a host
// that does none of it loses the effect silently in the browser rather than
// failing at compile time. See bindings.Requirements.
func hostRequirements(mod ir.Module) bindings.Requirements {
	var req bindings.Requirements
	if ir.UsesDerivatives(mod) {
		req.GLExtensions = append(req.GLExtensions, "OES_standard_derivatives")
	}
	if ir.UsesSceneSampleLevel(mod) {
		req.GLExtensions = append(req.GLExtensions, "EXT_shader_texture_lod")
		req.SceneColorMips = true
	}
	if ir.UsesSceneSize(mod) {
		req.GLSceneSizeUniform = "_sceneSize"
	}
	return req
}

// lowerMesh compiles a mesh material — the original Lower implementation.
// It synthesizes the default vertex transform and the fragment surface. When the
// material authors its own vertex() stage it dispatches to lowerMeshWithVertex
// (B4); the default-transform path below is unchanged so legacy mesh materials
// emit byte-identically.
func lowerMesh(m hir.Material) (ir.Module, bindings.Layout, error) {
	if m.Vertex != nil {
		return lowerMeshWithVertex(m)
	}
	if len(m.Varyings) > 0 {
		return ir.Module{}, bindings.Layout{}, diagnostic(
			CodeUnsupportedFeat, m.Varyings[0].Span,
			"varying %q requires a vertex() stage to write it", m.Varyings[0].Name,
		)
	}
	if err := rejectMeshStateField(m); err != nil {
		return ir.Module{}, bindings.Layout{}, err
	}

	paramKind, err := validateParams(m.Params, m.Context)
	if err != nil {
		return ir.Module{}, bindings.Layout{}, err
	}
	uniforms, err := buildUniformPlan(m.Params, m.Context)
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

	rs := &resolver{paramKind: paramKind, uniformOf: uniforms.uniformOf, varyingOf: interfaces.varyingOf, geo: m.Surface.Geo, allowDiscard: true}
	tp := &typer{paramKind: paramKind, geo: m.Surface.Geo, locals: map[string]ir.Type{}, mutableLocals: map[string]bool{}, paramArrays: uniforms.paramArrays}
	rs.tp = tp

	fragBody, fragOut, err := lowerFragment(m.Surface, reserved, rs, tp)
	if err != nil {
		return ir.Module{}, bindings.Layout{}, err
	}

	vertexOut := ir.Binary{Op: "*", L: ir.Ref{Name: "mvp"},
		R: ir.Construct{Type: ir.Vec4, Args: []ir.Expr{ir.Ref{Name: "position"}, ir.Lit{Value: 1.0}}}}

	irTextures := make([]ir.Texture, len(uniforms.textures))
	for i, t := range uniforms.textures {
		irTextures[i] = ir.Texture{Name: t, Cube: uniforms.cubeTextures[t]}
	}

	mod := ir.Module{
		Name:          m.Name,
		Kind:          ir.KindMesh,
		Uniforms:      toBindings(uniforms.uniforms),
		ArrayUniforms: toArrayBindings(uniforms.uniforms),
		Attributes:    interfaces.attributes,
		Varyings:      interfaces.varyings,
		Textures:      irTextures,
		Vertex:        ir.Stage{Body: interfaces.vertexBody, Output: vertexOut},
		Fragment:      ir.Stage{Body: fragBody, Output: fragOut},
	}
	ub := bindings.ComputeUniformBlock(uniforms.uniforms)
	stampContextClass(ub.Fields, uniforms.contextNames)
	layout := bindings.Layout{
		SchemaVersion:   bindings.DescriptorSchemaVersion,
		LanguageVersion: bindings.LanguageVersion,
		Material:        m.Name,
		Kind:            bindings.SurfaceKindMesh,
		EntryPoints:     bindings.EntryPoints{Vertex: "vertexMain", Fragment: "fragmentMain"},
		UniformBlock:    ub,
		Attributes:      toAttrs(interfaces.attributes),
		Textures:        bindings.ComputeTexturesMixed(uniforms.textures, uniforms.cubeTextures),
		WGSL:            bindings.WGSLBinding{Group: 0, Binding: 0},
		Metal:           bindings.MetalBinding{Buffer: 0},
		Context:         uniforms.contextNames,
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
		if p.IsArray {
			return ir.Module{}, bindings.Layout{}, diagnostic(
				CodeUnsupportedType, p.Span,
				"param %q: array params are not yet supported in points materials", p.Name,
			)
		}
		switch p.Type {
		case hir.Texture2D:
			return ir.Module{}, bindings.Layout{}, diagnostic(
				CodeUnsupportedType, p.Span,
				"param %q: texture2d params are not supported in points materials", p.Name,
			)
		case hir.TextureCube:
			return ir.Module{}, bindings.Layout{}, diagnostic(
				CodeUnsupportedType, p.Span,
				"param %q: textureCube params are not supported in points materials", p.Name,
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

	// Context fields (§3.3 C6): points has no array-uniform wiring yet (array
	// params are rejected above), so allowArrays is false here.
	seen := map[string]bool{}
	for k := range paramKind {
		seen[k] = true
	}
	contextNames, err := lowerContextFields(m.Context, seen, paramKind, nil, uniformOf, &uniformsList, false)
	if err != nil {
		return ir.Module{}, bindings.Layout{}, err
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
		paramKind: paramKind,
		uniformOf: uniformOf,
		varyingOf: varyingOf,
		geo:       m.Surface.Geo,
		geoFields: pointGeometry,
	}
	tp := &typer{
		paramKind:     paramKind,
		geo:           m.Surface.Geo,
		locals:        map[string]ir.Type{},
		mutableLocals: map[string]bool{},
		geoFields:     pointGeometry,
	}
	rs.tp = tp

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
	stampContextClass(ub.Fields, contextNames)
	// Points WGSL emits two vertex entry points:
	//   vertexMain        — reads a_position/a_size/a_color attributes (static layers)
	//   vertexStorageMain — reads from storage buffer particles[] (particle render)
	// GLSL/GLES emit only the attribute form.
	layout := bindings.Layout{
		SchemaVersion:   bindings.DescriptorSchemaVersion,
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
		Context:      contextNames,
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

	// Only scalar/vector/color params and fixed-size array params; no textures, no
	// Sun, in post materials. (User textures are not currently supported; engine
	// provides sceneColor/sceneDepth.) Array params let a fullscreen pass loop over
	// a uniform array (mesh/feedback already support them).
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
				"param %q: texture2d params are not supported in post materials (use sceneColor(uv) instead)", p.Name,
			)
		case hir.TextureCube:
			return ir.Module{}, bindings.Layout{}, diagnostic(
				CodeUnsupportedType, p.Span,
				"param %q: textureCube params are not supported in post materials", p.Name,
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

	// Context fields (§3.3 C6): post already wires ArrayUniforms for array
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

	// Post surface only exposes "uv" as a geo field.
	varyingOf := map[string]string{
		"uv": postGeometry["uv"].varying,
	}
	varyings := []ir.Binding{
		{Name: postGeometry["uv"].varying, Type: postGeometry["uv"].typ},
	}

	rs := &resolver{
		paramKind:        paramKind,
		uniformOf:        uniformOf,
		varyingOf:        varyingOf,
		geo:              m.Surface.Geo,
		geoFields:        postGeometry,
		allowSceneSample: true,
		allowDiscard:     true,
	}
	tp := &typer{
		paramKind:        paramKind,
		geo:              m.Surface.Geo,
		locals:           map[string]ir.Type{},
		mutableLocals:    map[string]bool{},
		geoFields:        postGeometry,
		allowSceneSample: true,
		paramArrays:      paramArrays,
	}
	rs.tp = tp

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
		Name:          m.Name,
		Kind:          ir.KindPost,
		Uniforms:      toBindings(uniformsList),
		ArrayUniforms: toArrayBindings(uniformsList),
		Varyings:      varyings,
		Fragment:      ir.Stage{Body: fragBody, Output: fragOut},
	}

	// For post passes: engine scene textures live at @group(0) @binding(0/1/2/3).
	// User uniforms start at @group(0) @binding(4) (WGSL) / buffer(1) (Metal).
	ub := bindings.ComputeUniformBlock(uniformsList)
	ub.Defaults = defaults
	stampContextClass(ub.Fields, contextNames)

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
		Context:         contextNames,
	}
	return mod, layout, nil
}
