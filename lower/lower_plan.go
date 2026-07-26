package lower

import (
	"fmt"

	"m31labs.dev/selena/bindings"
	"m31labs.dev/selena/hir"
	"m31labs.dev/selena/ir"
)

type uniformPlan struct {
	uniforms    []bindings.NamedType
	defaults    []bindings.DefaultValue
	uniformOf   map[string]string
	textures    []string
	// cubeTextures records which texture params are cube-map types (textureCube).
	// A name present in cubeTextures with value true emits texture_cube<f32>
	// (WGSL), samplerCube (GLSL/GLES), or texturecube<float> (Metal) and is
	// sampled with sampleCube(tex, dir) → ir.SampleCube. Absent or false = 2D.
	cubeTextures map[string]bool
	// paramArrays maps array-param names to their element type + count (B3.2).
	// These are uniform arrays declared as `param name : array<T, N>` and are
	// excluded from the scalar uniforms list. The typer uses this to allow
	// indexing expressions like drops[i] while rejecting bare refs to drops.
	paramArrays map[string]arrayInfo
	// contextNames lists the declared engine-injected uniform names (a
	// material's `context { ... }` block), in declaration order. Every name
	// here is also present in uniforms/paramArrays like an ordinary param; this
	// list is consulted after ComputeUniformBlock to stamp Class:"context" on
	// the matching bindings.Field and to fill bindings.Layout.Context (§3.2 of
	// the context-uniform design).
	contextNames []string
}

type interfacePlan struct {
	attributes []ir.Binding
	varyings   []ir.Binding
	varyingOf  map[string]string
	vertexBody []ir.Stmt
}

// validateParams validates param names (and, when given, context-field names)
// and returns the combined paramKind map the typer/resolver use to resolve a
// bare Ref by name. A context field's type is recorded exactly like a
// non-array param's — see the "Resolver detail" note in the context-uniform
// design §3.3: a body reference to a context field resolves identically to a
// param reference; only the compiled Layout tags its provenance.
func validateParams(params []hir.Param, context []hir.ContextField) (map[string]hir.Type, error) {
	paramKind := map[string]hir.Type{}
	seen := map[string]bool{}
	for _, p := range params {
		if err := validateAuthorName("param", p.Name, p.Span); err != nil {
			return nil, err
		}
		if seen[p.Name] {
			return nil, diagnostic(CodeDuplicateParam, p.Span, "duplicate param %q", p.Name)
		}
		seen[p.Name] = true
		if !p.IsArray {
			paramKind[p.Name] = p.Type
		}
		// Array params are not added to paramKind; the typer tracks them in
		// paramArrays so bare refs to the array are rejected and [i] indexing works.
	}
	for _, c := range context {
		if err := validateAuthorName("context field", c.Name, c.Span); err != nil {
			return nil, err
		}
		if seen[c.Name] {
			return nil, diagnostic(CodeDuplicateParam, c.Span, "context %q: name already used by a param or another context field", c.Name)
		}
		seen[c.Name] = true
		if !c.IsArray {
			paramKind[c.Name] = c.Type
		}
		// Array context fields are not added to paramKind for the same reason as
		// array params above; buildUniformPlan tracks them in paramArrays.
	}
	return paramKind, nil
}

func buildUniformPlan(params []hir.Param, context []hir.ContextField) (uniformPlan, error) {
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
		// Array params (B3.2): added to paramArrays and the array section of the
		// uniform block, but NOT to the scalar uniforms list or paramKind.
		if p.IsArray {
			if p.Default != nil {
				return uniformPlan{}, diagnostic(CodeInvalidDefault, hir.ExprSpan(p.Default), "param %q: defaults for array params are not supported", p.Name)
			}
			elemType, ok := hirToIRType(p.Type)
			if !ok {
				return uniformPlan{}, diagnostic(CodeUnsupportedType, p.Span, "param %q: unsupported array element type %q", p.Name, p.Type)
			}
			if p.ArraySize <= 0 {
				return uniformPlan{}, diagnostic(CodeUnsupportedType, p.Span, "param %q: array size must be positive", p.Name)
			}
			if plan.paramArrays == nil {
				plan.paramArrays = map[string]arrayInfo{}
			}
			plan.paramArrays[p.Name] = arrayInfo{elemType: elemType, size: p.ArraySize}
			plan.uniformOf[p.Name] = p.Name
			// Add to the uniforms list with Count > 1 so ComputeUniformBlock
			// picks up the std140 array layout (stride 16 per element).
			if _, ok := uniType[p.Name]; ok {
				return uniformPlan{}, fmt.Errorf("param %q: uniform %q is declared more than once", p.Name, p.Name)
			}
			plan.uniforms = append(plan.uniforms, bindings.NamedType{Name: p.Name, Type: elemType, Count: p.ArraySize})
			uniType[p.Name] = elemType
			continue
		}
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
		case hir.TextureCube:
			if p.Default != nil {
				return uniformPlan{}, diagnostic(CodeInvalidDefault, hir.ExprSpan(p.Default), "param %q: defaults for textureCube are not supported", p.Name)
			}
			plan.textures = append(plan.textures, p.Name)
			if plan.cubeTextures == nil {
				plan.cubeTextures = map[string]bool{}
			}
			plan.cubeTextures[p.Name] = true
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

	// Context fields (the "context uniform" concept, §3.3 C5): appended AFTER
	// every author param, in declaration order, into the SAME
	// uniforms/paramArrays/uniformOf plumbing a param uses — addUniform's
	// uniType map already rejects a name collision with an existing param or
	// auto-uniform (mvp/normalMatrix). No Sun records and no textures: those are
	// separate binding categories, not uniform fields (§3.1). A context field's
	// Default is intentionally NOT added to plan.defaults — it must never
	// surface as a customUniforms default (C9); it exists in the HIR purely as
	// a native/test-oracle fallback for a later host-side path.
	for _, c := range context {
		if c.IsArray {
			elemType, ok := hirToIRType(c.Type)
			if !ok {
				return uniformPlan{}, diagnostic(CodeUnsupportedType, c.Span, "context %q: unsupported array element type %q", c.Name, c.Type)
			}
			if c.ArraySize <= 0 {
				return uniformPlan{}, diagnostic(CodeUnsupportedType, c.Span, "context %q: array size must be positive", c.Name)
			}
			if plan.paramArrays == nil {
				plan.paramArrays = map[string]arrayInfo{}
			}
			if _, ok := uniType[c.Name]; ok {
				return uniformPlan{}, fmt.Errorf("context %q: uniform %q is declared more than once", c.Name, c.Name)
			}
			plan.paramArrays[c.Name] = arrayInfo{elemType: elemType, size: c.ArraySize}
			plan.uniformOf[c.Name] = c.Name
			plan.uniforms = append(plan.uniforms, bindings.NamedType{Name: c.Name, Type: elemType, Count: c.ArraySize})
			uniType[c.Name] = elemType
			plan.contextNames = append(plan.contextNames, c.Name)
			continue
		}
		switch c.Type {
		case hir.Sun, hir.Texture2D, hir.TextureCube:
			return uniformPlan{}, diagnostic(CodeUnsupportedType, c.Span, "context %q: type %q is not supported in a context block (no records or textures)", c.Name, c.Type)
		default:
			t, ok := hirToIRType(c.Type)
			if !ok {
				return uniformPlan{}, diagnostic(CodeUnsupportedType, c.Span, "context %q: unsupported type %q", c.Name, c.Type)
			}
			if err := addUniform(c.Name, t); err != nil {
				return uniformPlan{}, fmt.Errorf("context %q: %w", c.Name, err)
			}
			plan.uniformOf[c.Name] = c.Name
			plan.contextNames = append(plan.contextNames, c.Name)
		}
	}
	return plan, nil
}

// lowerContextFields validates and appends context into the accumulators an
// inline per-kind param loop already maintains (lowerPoints/lowerPost/
// lowerFeedback each build their own paramKind/uniformsList/uniformOf/
// paramArrays instead of going through buildUniformPlan) — the array/post/
// feedback counterpart of the context-append loop buildUniformPlan runs for
// mesh materials (§3.3 C5/C6). It returns the ordered context-name list for
// stampContextClass + bindings.Layout.Context.
//
// seen is the existing param-name set (paramKind ∪ paramArrays keys); it is
// extended in place so two context fields (or a context field and a param)
// cannot collide. allowArrays gates array-typed context fields off for kinds
// whose ir.Module doesn't wire ArrayUniforms yet (points, which also rejects
// array params outright); paramArrays may be nil when allowArrays is false.
func lowerContextFields(
	context []hir.ContextField,
	seen map[string]bool,
	paramKind map[string]hir.Type,
	paramArrays map[string]arrayInfo,
	uniformOf map[string]string,
	uniformsList *[]bindings.NamedType,
	allowArrays bool,
) ([]string, error) {
	var names []string
	for _, c := range context {
		if err := validateAuthorName("context field", c.Name, c.Span); err != nil {
			return nil, err
		}
		if seen[c.Name] {
			return nil, diagnostic(CodeDuplicateParam, c.Span, "context %q: name already used by a param or another context field", c.Name)
		}
		seen[c.Name] = true
		if c.IsArray {
			if !allowArrays {
				return nil, diagnostic(CodeUnsupportedType, c.Span, "context %q: array context fields are not yet supported in this material kind", c.Name)
			}
			elemType, ok := hirToIRType(c.Type)
			if !ok {
				return nil, diagnostic(CodeUnsupportedType, c.Span, "context %q: unsupported array element type %q", c.Name, c.Type)
			}
			if c.ArraySize <= 0 {
				return nil, diagnostic(CodeUnsupportedType, c.Span, "context %q: array size must be positive", c.Name)
			}
			paramArrays[c.Name] = arrayInfo{elemType: elemType, size: c.ArraySize}
			*uniformsList = append(*uniformsList, bindings.NamedType{Name: c.Name, Type: elemType, Count: c.ArraySize})
			names = append(names, c.Name)
			continue
		}
		switch c.Type {
		case hir.Sun, hir.Texture2D, hir.TextureCube:
			return nil, diagnostic(CodeUnsupportedType, c.Span, "context %q: type %q is not supported in a context block (no records or textures)", c.Name, c.Type)
		default:
			t, ok := hirToIRType(c.Type)
			if !ok {
				return nil, diagnostic(CodeUnsupportedType, c.Span, "context %q: unsupported type %q", c.Name, c.Type)
			}
			paramKind[c.Name] = c.Type
			*uniformsList = append(*uniformsList, bindings.NamedType{Name: c.Name, Type: t})
			uniformOf[c.Name] = c.Name
			names = append(names, c.Name)
		}
	}
	return names, nil
}

// stampContextClass marks Class:"context" on every field in fields whose name
// is in names (mutating in place — fields is a slice, so this is visible to
// the caller without needing a pointer). Used after ComputeUniformBlock by
// every kind lowerer that supports context fields (§3.2/§3.3 C6/C7).
func stampContextClass(fields []bindings.Field, names []string) {
	if len(names) == 0 {
		return
	}
	set := make(map[string]bool, len(names))
	for _, n := range names {
		set[n] = true
	}
	for i := range fields {
		if set[fields[i].Name] {
			fields[i].Class = "context"
		}
	}
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
	for _, s := range surface.Body {
		collectGeoStmt(s, surface.Geo, usedGeo)
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
	lc := &lowerCtx{reserved: reserved, rs: rs, tp: tp, localKind: "surface local"}
	fragBody, err := lc.lowerStmts(surface.Body)
	if err != nil {
		return nil, nil, err
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

// lowerCtx carries the lowering state through recursive statement lowering.
type lowerCtx struct {
	reserved   map[string]string
	rs         *resolver
	tp         *typer
	seenLocals map[string]bool // top-level flat scope
	// localKind names the kind of binding in diagnostics ("surface local" for
	// fragment/surface bodies, "vertex local" for authored vertex() bodies).
	localKind string
	// varyingType is non-nil only when lowering an authored vertex() body
	// (B4). It maps each author-declared varying name to its type so an
	// hir.Assign to that name is recognised as a varying write (emitted as a
	// Target-only ir.Stmt, matched by the emitters' Varyings set) rather than
	// a reassignment to a mutable local. Fragment/surface lowering leaves this
	// nil: surface() never writes varyings, so the Assign case always falls
	// through to the ordinary mutable-local path.
	varyingType map[string]ir.Type
}

// checkNotVarying rejects a local declaration whose name collides with an
// author-declared varying (vertex() bodies only; varyingType is nil for every
// other lowering context, so this is a no-op there).
func (lc *lowerCtx) checkNotVarying(name string, span hir.Span) error {
	if lc.varyingType == nil {
		return nil
	}
	if _, isVary := lc.varyingType[name]; isVary {
		return diagnostic(CodeDuplicateLocal, span, "%s %q conflicts with varying %q", lc.localKind, name, name)
	}
	return nil
}

func (lc *lowerCtx) ensureSeenLocals() {
	if lc.seenLocals == nil {
		lc.seenLocals = map[string]bool{}
	}
}

func (lc *lowerCtx) lowerStmts(stmts []hir.Stmt) ([]ir.Stmt, error) {
	var out []ir.Stmt
	for _, s := range stmts {
		irStmt, err := lc.lowerStmt(s)
		if err != nil {
			return nil, err
		}
		out = append(out, irStmt)
	}
	return out, nil
}

func (lc *lowerCtx) lowerStmt(s hir.Stmt) (ir.Stmt, error) {
	lc.ensureSeenLocals()
	switch x := s.(type) {
	case hir.Let:
		if err := validateAuthorName(lc.localKind, x.Name, x.Span); err != nil {
			return ir.Stmt{}, err
		}
		if err := lc.checkNotVarying(x.Name, x.Span); err != nil {
			return ir.Stmt{}, err
		}
		if kind, ok := lc.reserved[x.Name]; ok {
			return ir.Stmt{}, diagnostic(CodeDuplicateLocal, x.Span, "%s %q conflicts with %s", lc.localKind, x.Name, kind)
		}
		if lc.seenLocals[x.Name] {
			return ir.Stmt{}, diagnostic(CodeDuplicateLocal, x.Span, "duplicate %s %q", lc.localKind, x.Name)
		}
		t, err := lc.tp.typeOf(x.Value)
		if err != nil {
			return ir.Stmt{}, fmt.Errorf("%s %q: %w", lc.localKind, x.Name, err)
		}
		ve, err := lc.rs.expr(x.Value)
		if err != nil {
			return ir.Stmt{}, fmt.Errorf("%s %q: %w", lc.localKind, x.Name, err)
		}
		lc.seenLocals[x.Name] = true
		lc.tp.locals[x.Name] = t
		return ir.Stmt{Target: x.Name, Type: t, Value: ve}, nil

	case hir.VarDecl:
		if err := validateAuthorName(lc.localKind, x.Name, x.Span); err != nil {
			return ir.Stmt{}, err
		}
		if err := lc.checkNotVarying(x.Name, x.Span); err != nil {
			return ir.Stmt{}, err
		}
		if kind, ok := lc.reserved[x.Name]; ok {
			return ir.Stmt{}, diagnostic(CodeDuplicateLocal, x.Span, "%s %q conflicts with %s", lc.localKind, x.Name, kind)
		}
		if lc.seenLocals[x.Name] {
			return ir.Stmt{}, diagnostic(CodeDuplicateLocal, x.Span, "duplicate %s %q", lc.localKind, x.Name)
		}
		t, err := lc.tp.typeOf(x.Value)
		if err != nil {
			return ir.Stmt{}, fmt.Errorf("%s %q: %w", lc.localKind, x.Name, err)
		}
		ve, err := lc.rs.expr(x.Value)
		if err != nil {
			return ir.Stmt{}, fmt.Errorf("%s %q: %w", lc.localKind, x.Name, err)
		}
		lc.seenLocals[x.Name] = true
		lc.tp.locals[x.Name] = t
		lc.tp.mutableLocals[x.Name] = true
		return ir.Stmt{Target: x.Name, Type: t, Value: ve, Mutable: true}, nil

	case hir.Assign:
		if err := validateAuthorName(lc.localKind, x.Name, x.Span); err != nil {
			return ir.Stmt{}, err
		}
		if lc.varyingType != nil {
			if vt, isVary := lc.varyingType[x.Name]; isVary {
				t, err := lc.tp.typeOf(x.Value)
				if err != nil {
					return ir.Stmt{}, fmt.Errorf("varying %q: %w", x.Name, err)
				}
				if t != vt {
					return ir.Stmt{}, diagnostic(CodeTypeMismatch, x.Span, "varying %q is %s but assigned %s", x.Name, vt, t)
				}
				ve, err := lc.rs.expr(x.Value)
				if err != nil {
					return ir.Stmt{}, fmt.Errorf("varying %q: %w", x.Name, err)
				}
				// Emitted as a varying write (out.<name> = ...), matched by the
				// emitters' Varyings set — CF stays nil so this renders through
				// the same "declaration vs. output write" branch as a flat
				// (non-CF) authored-vertex body did before B4's CF support.
				return ir.Stmt{Target: x.Name, Value: ve}, nil
			}
		}
		existing, ok := lc.tp.locals[x.Name]
		if !ok {
			return ir.Stmt{}, diagnostic(CodeUnknownName, x.Span, "assignment to undeclared local %q", x.Name)
		}
		if !lc.tp.mutableLocals[x.Name] {
			return ir.Stmt{}, diagnostic(CodeDuplicateLocal, x.Span, "cannot assign to immutable local %q (use var instead of let)", x.Name)
		}
		t, err := lc.tp.typeOf(x.Value)
		if err != nil {
			return ir.Stmt{}, fmt.Errorf("assignment to %q: %w", x.Name, err)
		}
		if t != existing {
			return ir.Stmt{}, diagnostic(CodeTypeMismatch, x.Span, "assignment to %q: got %s, want %s", x.Name, t, existing)
		}
		ve, err := lc.rs.expr(x.Value)
		if err != nil {
			return ir.Stmt{}, fmt.Errorf("assignment to %q: %w", x.Name, err)
		}
		return ir.Stmt{CF: ir.AssignCF{Target: x.Name, Value: ve}}, nil

	case hir.If:
		condT, err := lc.tp.typeOf(x.Cond)
		if err != nil {
			return ir.Stmt{}, fmt.Errorf("if condition: %w", err)
		}
		if condT != ir.Bool {
			return ir.Stmt{}, diagnostic(CodeTypeMismatch, x.Span, "if condition must be bool, got %s", condT)
		}
		cond, err := lc.rs.expr(x.Cond)
		if err != nil {
			return ir.Stmt{}, fmt.Errorf("if condition: %w", err)
		}
		then, err := lc.lowerStmts(x.Then)
		if err != nil {
			return ir.Stmt{}, err
		}
		var els []ir.Stmt
		if len(x.Else) > 0 {
			els, err = lc.lowerStmts(x.Else)
			if err != nil {
				return ir.Stmt{}, err
			}
		}
		return ir.Stmt{CF: ir.IfCF{Cond: cond, Then: then, Else: els}}, nil

	case hir.VarArrayDecl:
		if err := validateAuthorName(lc.localKind, x.Name, x.Span); err != nil {
			return ir.Stmt{}, err
		}
		if err := lc.checkNotVarying(x.Name, x.Span); err != nil {
			return ir.Stmt{}, err
		}
		if kind, ok := lc.reserved[x.Name]; ok {
			return ir.Stmt{}, diagnostic(CodeDuplicateLocal, x.Span, "%s %q conflicts with %s", lc.localKind, x.Name, kind)
		}
		if lc.seenLocals[x.Name] {
			return ir.Stmt{}, diagnostic(CodeDuplicateLocal, x.Span, "duplicate %s %q", lc.localKind, x.Name)
		}
		elemType, ok := hirToIRType(x.ElemType)
		if !ok {
			return ir.Stmt{}, diagnostic(CodeUnsupportedType, x.Span, "array element type %q is not supported", x.ElemType)
		}
		lc.seenLocals[x.Name] = true
		// Register in arrayLocals (not in tp.locals — whole-array refs are invalid).
		if lc.tp.arrayLocals == nil {
			lc.tp.arrayLocals = map[string]arrayInfo{}
		}
		lc.tp.arrayLocals[x.Name] = arrayInfo{elemType: elemType, size: x.Size}
		return ir.Stmt{CF: ir.VarArrayCF{Target: x.Name, ElemType: elemType, Size: x.Size}}, nil

	case hir.IndexAssign:
		if err := validateAuthorName(lc.localKind, x.Name, x.Span); err != nil {
			return ir.Stmt{}, err
		}
		if lc.tp.arrayLocals == nil {
			return ir.Stmt{}, diagnostic(CodeUnknownName, x.Span, "unknown array %q (declare with var a : array<T, N> first)", x.Name)
		}
		info, ok := lc.tp.arrayLocals[x.Name]
		if !ok {
			return ir.Stmt{}, diagnostic(CodeUnknownName, x.Span, "%q is not a local array", x.Name)
		}
		idxT, err := lc.tp.typeOf(x.Index)
		if err != nil {
			return ir.Stmt{}, fmt.Errorf("index of %q: %w", x.Name, err)
		}
		if idxT != ir.Int && idxT != ir.Uint {
			return ir.Stmt{}, diagnostic(CodeTypeMismatch, x.Span, "%s", arrayIndexTypeMessage(idxT, x.Index))
		}
		valT, err := lc.tp.typeOf(x.Value)
		if err != nil {
			return ir.Stmt{}, fmt.Errorf("value assigned to %q[i]: %w", x.Name, err)
		}
		if valT != info.elemType {
			return ir.Stmt{}, diagnostic(CodeTypeMismatch, x.Span, "%q element type is %s but assigned %s", x.Name, info.elemType, valT)
		}
		idxE, err := lc.rs.expr(x.Index)
		if err != nil {
			return ir.Stmt{}, fmt.Errorf("index of %q: %w", x.Name, err)
		}
		valE, err := lc.rs.expr(x.Value)
		if err != nil {
			return ir.Stmt{}, fmt.Errorf("value assigned to %q[i]: %w", x.Name, err)
		}
		return ir.Stmt{CF: ir.IndexAssignCF{Target: x.Name, Index: idxE, Value: valE}}, nil

	case hir.Discard:
		if !lc.rs.allowDiscard {
			return ir.Stmt{}, diagnostic(
				CodeUnsupportedFeat, x.Span,
				"discard is only available in the fragment stage of mesh and post materials",
			)
		}
		return ir.Stmt{CF: ir.DiscardCF{}}, nil

	case hir.Break:
		if lc.rs.loopDepth <= 0 {
			return ir.Stmt{}, diagnostic(
				CodeUnsupportedFeat, x.Span,
				"break is only valid inside a for loop",
			)
		}
		return ir.Stmt{CF: ir.BreakCF{}}, nil

	case hir.For:
		// Lower init value and declare the loop variable as mutable.
		if err := validateAuthorName("for loop variable", x.InitName, x.Span); err != nil {
			return ir.Stmt{}, err
		}
		initT, err := lc.tp.typeOf(x.InitValue)
		if err != nil {
			return ir.Stmt{}, fmt.Errorf("for init %q: %w", x.InitName, err)
		}
		initV, err := lc.rs.expr(x.InitValue)
		if err != nil {
			return ir.Stmt{}, fmt.Errorf("for init %q: %w", x.InitName, err)
		}
		// Register loop variable in scope.
		lc.seenLocals[x.InitName] = true
		lc.tp.locals[x.InitName] = initT
		lc.tp.mutableLocals[x.InitName] = true

		// Condition must be bool.
		condT, err := lc.tp.typeOf(x.Cond)
		if err != nil {
			return ir.Stmt{}, fmt.Errorf("for condition: %w", err)
		}
		if condT != ir.Bool {
			return ir.Stmt{}, diagnostic(CodeTypeMismatch, x.Span, "for condition must be bool, got %s", condT)
		}
		cond, err := lc.rs.expr(x.Cond)
		if err != nil {
			return ir.Stmt{}, fmt.Errorf("for condition: %w", err)
		}

		// Post-update must be an assignment to the same variable.
		postT, err := lc.tp.typeOf(x.PostValue)
		if err != nil {
			return ir.Stmt{}, fmt.Errorf("for post %q: %w", x.PostName, err)
		}
		if postT != initT {
			return ir.Stmt{}, diagnostic(CodeTypeMismatch, x.Span, "for post %q: got %s, want %s", x.PostName, postT, initT)
		}
		postV, err := lc.rs.expr(x.PostValue)
		if err != nil {
			return ir.Stmt{}, fmt.Errorf("for post %q: %w", x.PostName, err)
		}

		lc.rs.loopDepth++
		body, err := lc.lowerStmts(x.Body)
		lc.rs.loopDepth--
		if err != nil {
			return ir.Stmt{}, err
		}

		return ir.Stmt{CF: ir.ForCF{
			InitTarget: x.InitName,
			InitType:   initT,
			InitValue:  initV,
			Cond:       cond,
			PostTarget: x.PostName,
			PostValue:  postV,
			Body:       body,
		}}, nil
	}
	return ir.Stmt{}, fmt.Errorf("unsupported HIR statement type %T in lower", s)
}
