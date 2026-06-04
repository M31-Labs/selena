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
// host binding layout. This is where the pains disappear: bindings are
// allocated, the std140 layout is computed, interpolants are inferred and the
// vertex stage synthesized from the stdlib transform, stdlib records expand into
// uniforms, locals are typed, and a vec3 surface result is wrapped to vec4.
func Lower(m hir.Material) (ir.Module, bindings.Layout, error) {
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
