package lower

import (
	"m31labs.dev/selena/bindings"
	"m31labs.dev/selena/hir"
	"m31labs.dev/selena/ir"
)

// meshVertexGeometry lists the raw per-vertex attributes a mesh/general vertex()
// body may read as geo.<field>. Unlike the default mesh surface (which exposes
// derived fields like worldNormal as synthesized varyings), the authored vertex
// stage sees the plain attributes; derived values (e.g. world normal) are the
// author's to compute from geo.normal + normalMatrix. B4.
var meshVertexGeometry = map[string]geometrySpec{
	"position": {typ: ir.Vec3},
	"normal":   {typ: ir.Vec3},
	"uv":       {typ: ir.Vec2},
}

// meshVertexAttrOrder fixes the attribute location order for the authored vertex
// stage so emitted locations are deterministic and position stays at location 0.
var meshVertexAttrOrder = []string{"position", "normal", "uv"}

// rejectMeshStateField errors when a default-transform (no vertex()) mesh
// material declares a statefield. stateAt(uv) is only wired for materials that
// author a vertex() stage (B4); a bare statefield on a default mesh has no read
// path, so it is rejected with a clear diagnostic rather than silently ignored.
func rejectMeshStateField(m hir.Material) error {
	if len(m.States) > 0 {
		return diagnostic(
			CodeUnsupportedFeat, m.States[0].Span,
			"statefield %q requires a vertex() stage; stateAt(uv) is only available in mesh materials that author vertex()",
			m.States[0].Name,
		)
	}
	return nil
}

// lowerMeshWithVertex compiles a mesh/general material that authors its own
// vertex() stage (B4). The author body computes the clip-space position (its
// return) and writes author-declared varyings; the surface() fragment reads
// those varyings as geo.<name>. An optional statefield enables stateAt(uv) reads
// in both stages. The default-transform mesh path (lowerMesh) is untouched, so
// every legacy mesh material still emits byte-identically.
func lowerMeshWithVertex(m hir.Material) (ir.Module, bindings.Layout, error) {
	paramKind, err := validateParams(m.Params, m.Context)
	if err != nil {
		return ir.Module{}, bindings.Layout{}, err
	}
	uniforms, err := buildUniformPlan(m.Params, m.Context)
	if err != nil {
		return ir.Module{}, bindings.Layout{}, err
	}

	// Optional statefield (at most one) enabling stateAt(uv).
	stateName := ""
	if len(m.States) > 1 {
		return ir.Module{}, bindings.Layout{}, diagnostic(
			CodeUnsupportedFeat, m.States[1].Span,
			"material %q declares more than one statefield; exactly one is supported", m.Name,
		)
	}
	if len(m.States) == 1 {
		stateName = m.States[0].Name
		if err := validateAuthorName("statefield", stateName, m.States[0].Span); err != nil {
			return ir.Module{}, bindings.Layout{}, err
		}
	}
	hasState := stateName != ""

	// Author-declared varyings: written by vertex(), read by surface() as geo.<name>.
	varyingOf := map[string]string{}
	varyingType := map[string]ir.Type{}
	var varyings []ir.Binding
	fragGeo := map[string]geometrySpec{} // surface geometry fields = author varyings
	seenVary := map[string]bool{}
	for _, v := range m.Varyings {
		if err := validateAuthorName("varying", v.Name, v.Span); err != nil {
			return ir.Module{}, bindings.Layout{}, err
		}
		if seenVary[v.Name] {
			return ir.Module{}, bindings.Layout{}, diagnostic(CodeDuplicateLocal, v.Span, "duplicate varying %q", v.Name)
		}
		seenVary[v.Name] = true
		vt, ok := hirToIRType(v.Type)
		if !ok {
			return ir.Module{}, bindings.Layout{}, diagnostic(CodeUnsupportedType, v.Span, "varying %q: unsupported type %q", v.Name, v.Type)
		}
		varyingOf[v.Name] = v.Name
		varyingType[v.Name] = vt
		varyings = append(varyings, ir.Binding{Name: v.Name, Type: vt})
		name := v.Name
		fragGeo[v.Name] = geometrySpec{typ: vt, varying: v.Name, build: func() ir.Expr { return ir.Ref{Name: name} }}
	}

	// Which raw attributes does the vertex body read via geo.<field>?
	usedVertexGeo := map[string]bool{}
	if m.Vertex.Geo != "" {
		collectGeo(m.Vertex.Result, m.Vertex.Geo, usedVertexGeo)
		for _, s := range m.Vertex.Body {
			collectGeoStmt(s, m.Vertex.Geo, usedVertexGeo)
		}
	}
	var attributes []ir.Binding
	vVaryingOf := map[string]string{} // geo.<field> -> attribute name (vertex stage)
	for _, name := range meshVertexAttrOrder {
		if !usedVertexGeo[name] {
			continue
		}
		spec := meshVertexGeometry[name]
		attributes = append(attributes, ir.Binding{Name: name, Type: spec.typ})
		vVaryingOf[name] = name
	}
	for f := range usedVertexGeo {
		if _, ok := meshVertexGeometry[f]; !ok {
			return ir.Module{}, bindings.Layout{}, diagnostic(
				CodeInvalidMember, m.Vertex.Span,
				"geo.%s is not a vertex attribute (available: position, normal, uv)", f,
			)
		}
	}

	reserved, err := interfaceNames(uniforms.uniforms, uniforms.textures, attributes, varyings)
	if err != nil {
		return ir.Module{}, bindings.Layout{}, err
	}

	// --- vertex stage ---
	// The vertex typer/resolver also see the engine transform uniforms (mvp,
	// normalMatrix) so the author can transform geometry; they are already in the
	// uniform plan's uniformOf.
	vParamKind := map[string]hir.Type{"mvp": hir.Mat4, "normalMatrix": hir.Mat3}
	for k, v := range paramKind {
		vParamKind[k] = v
	}
	vrs := &resolver{
		paramKind:        vParamKind,
		uniformOf:        uniforms.uniformOf,
		varyingOf:        vVaryingOf,
		geo:              m.Vertex.Geo,
		geoFields:        meshVertexGeometry,
		allowVertexIndex: true,
		allowStateAt:     hasState,
	}
	vtp := &typer{
		paramKind:        vParamKind,
		geo:              m.Vertex.Geo,
		locals:           map[string]ir.Type{},
		mutableLocals:    map[string]bool{},
		geoFields:        meshVertexGeometry,
		allowVertexIndex: true,
		allowStateAt:     hasState,
		paramArrays:      uniforms.paramArrays,
	}
	vrs.tp = vtp
	vertexBody, err := lowerVertexStage(m.Vertex, reserved, vrs, vtp, varyingType)
	if err != nil {
		return ir.Module{}, bindings.Layout{}, err
	}
	vrt, err := vtp.typeOf(m.Vertex.Result)
	if err != nil {
		return ir.Module{}, bindings.Layout{}, diagnostic(CodeTypeMismatch, m.Vertex.Span, "vertex() return: %v", err)
	}
	if vrt != ir.Vec4 {
		return ir.Module{}, bindings.Layout{}, diagnostic(CodeTypeMismatch, m.Vertex.Span, "vertex() must return the clip-space position as vec4, got %s", vrt)
	}
	vertexOut, err := vrs.expr(m.Vertex.Result)
	if err != nil {
		return ir.Module{}, bindings.Layout{}, err
	}

	// --- fragment stage (surface) ---
	frs := &resolver{
		paramKind:    paramKind,
		uniformOf:    uniforms.uniformOf,
		varyingOf:    varyingOf,
		geo:          m.Surface.Geo,
		geoFields:    fragGeo,
		allowStateAt: hasState,
		allowDiscard: true,
	}
	ftp := &typer{
		paramKind:     paramKind,
		geo:           m.Surface.Geo,
		locals:        map[string]ir.Type{},
		mutableLocals: map[string]bool{},
		geoFields:     fragGeo,
		allowStateAt:  hasState,
		paramArrays:   uniforms.paramArrays,
	}
	frs.tp = ftp
	fragBody, fragOut, err := lowerFragment(m.Surface, reserved, frs, ftp)
	if err != nil {
		return ir.Module{}, bindings.Layout{}, err
	}

	irTextures := make([]ir.Texture, len(uniforms.textures))
	for i, t := range uniforms.textures {
		irTextures[i] = ir.Texture{Name: t, Cube: uniforms.cubeTextures[t]}
	}

	mod := ir.Module{
		Name:            m.Name,
		Kind:            ir.KindMesh,
		Uniforms:        toBindings(uniforms.uniforms),
		ArrayUniforms:   toArrayBindings(uniforms.uniforms),
		Attributes:      attributes,
		Varyings:        varyings,
		Textures:        irTextures,
		Vertex:          ir.Stage{Body: vertexBody, Output: vertexOut},
		Fragment:        ir.Stage{Body: fragBody, Output: fragOut},
		StateField:      stateName,
		VertexAuthored:  true,
		UsesVertexIndex: irStageUsesVertexIndex(vertexBody, vertexOut),
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
		Attributes:      toAttrs(attributes),
		Textures:        bindings.ComputeTexturesMixed(uniforms.textures, uniforms.cubeTextures),
		WGSL:            bindings.WGSLBinding{Group: 0, Binding: 0},
		Metal:           bindings.MetalBinding{Buffer: 0},
		Context:         uniforms.contextNames,
	}
	if layout.Textures == nil {
		layout.Textures = []bindings.Texture{}
	}
	layout.UniformBlock.Defaults = uniforms.defaults
	if hasState {
		gridBinding, stateBinding := meshStateBindings(len(uniforms.textures))
		layout.States = []bindings.StateField{{
			Name:  stateName,
			WGSL:  bindings.WGSLStateBinding{Group: 0, InBinding: stateBinding, OutBinding: -1},
			GL:    bindings.GLStateBinding{Uniform: "stateTex", Unit: 0},
			Metal: bindings.MetalStateBinding{InBuffer: 2, OutBuffer: -1},
		}}
		layout.Grid = &bindings.GridBinding{
			WGSL:           bindings.WGSLBinding{Group: 0, Binding: gridBinding},
			Metal:          bindings.MetalBinding{Buffer: 1},
			GLTexelUniform: "",
		}
	}
	return mod, layout, nil
}

// meshStateBindings returns the WGSL @binding slots for the StateGrid uniform and
// the inState storage buffer of an authored-vertex mesh statefield, placed after
// the uniform block (binding 0) and any textures (1+2i / 2+2i). The emitters use
// the same arithmetic. B4.
func meshStateBindings(numTextures int) (gridBinding, stateBinding int) {
	base := 1 + 2*numTextures
	return base, base + 1
}

// lowerVertexStage lowers an authored vertex() body. It routes through the same
// recursive statement lowerer surface()/fragment bodies use (lowerCtx.lowerStmts
// in lower_plan.go), so `let`/`var` bindings, `if`, and `for` are all available
// in vertex() exactly as they are in surface() (B5). The one vertex-specific
// twist is varyingType: an hir.Assign whose name matches a declared varying is
// recognised as a varying write (`name = expr`, CF stays nil, Target is the
// varying name) rather than a reassignment to a mutable local — see the
// hir.Assign case in lowerCtx.lowerStmt. Local arrays (var a : array<T, N>) and
// index assignment fall out for free from the shared path; only `discard`
// remains fragment-only (rs.allowDiscard is false for the vertex resolver).
func lowerVertexStage(vfn *hir.Func, reserved map[string]string, rs *resolver, tp *typer, varyingType map[string]ir.Type) ([]ir.Stmt, error) {
	lc := &lowerCtx{reserved: reserved, rs: rs, tp: tp, localKind: "vertex local", varyingType: varyingType}
	return lc.lowerStmts(vfn.Body)
}

// irStageUsesVertexIndex reports whether the vertex stage references the
// vertexIndex builtin, so the emitters know to wire the per-backend index source.
func irStageUsesVertexIndex(body []ir.Stmt, output ir.Expr) bool {
	for i := range body {
		if irExprUsesVertexIndex(body[i].Value) {
			return true
		}
	}
	return irExprUsesVertexIndex(output)
}

func irExprUsesVertexIndex(e ir.Expr) bool {
	switch x := e.(type) {
	case ir.Ref:
		return x.Name == "vertexIndex"
	case ir.Construct:
		for _, a := range x.Args {
			if irExprUsesVertexIndex(a) {
				return true
			}
		}
	case ir.Call:
		for _, a := range x.Args {
			if irExprUsesVertexIndex(a) {
				return true
			}
		}
	case ir.Binary:
		return irExprUsesVertexIndex(x.L) || irExprUsesVertexIndex(x.R)
	case ir.Unary:
		return irExprUsesVertexIndex(x.E)
	case ir.Swizzle:
		return irExprUsesVertexIndex(x.E)
	case ir.Conditional:
		return irExprUsesVertexIndex(x.Cond) || irExprUsesVertexIndex(x.Then) || irExprUsesVertexIndex(x.Alt)
	case ir.StateSampleUV:
		return irExprUsesVertexIndex(x.UV)
	case ir.Index:
		return irExprUsesVertexIndex(x.Arr) || irExprUsesVertexIndex(x.Idx)
	}
	return false
}
