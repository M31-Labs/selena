package lower

import (
	"fmt"
	"sort"
	"strings"

	"m31labs.dev/selena/bindings"
	"m31labs.dev/selena/hir"
	"m31labs.dev/selena/ir"
)

// --- stdlib knowledge (hardcoded for M1; generalized into a real stdlib in M4) ---

// sunFields are the members of the stdlib Sun record.
var sunFields = map[string]ir.Type{"dir": ir.Vec3, "ambient": ir.Float}

// computedGeo describes a stdlib-computed geometry field: the varying it lands
// in, its type, the attributes the vertex stage needs, and how to compute it.
type computedGeo struct {
	varying string
	typ     ir.Type
	attrs   []string
	build   func() ir.Expr
}

var geoComputed = map[string]computedGeo{
	"worldNormal": {
		varying: "vWorldNormal", typ: ir.Vec3, attrs: []string{"normal"},
		build: func() ir.Expr {
			return ir.Call{Func: "normalize", Args: []ir.Expr{
				ir.Binary{Op: "*", L: ir.Ref{Name: "normalMatrix"}, R: ir.Ref{Name: "normal"}},
			}}
		},
	},
}

// geoAttr are geometry fields that map directly to a vertex attribute.
var geoAttr = map[string]ir.Type{"position": ir.Vec3, "normal": ir.Vec3, "uv": ir.Vec2}

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

// Lower compiles a high-level material into the low-level ir.Module plus the
// host binding layout. This is where the pains disappear: bindings are
// allocated, the std140 layout is computed, interpolants are inferred and the
// vertex stage synthesized from the stdlib transform, stdlib records expand into
// uniforms, locals are typed, and a vec3 surface result is wrapped to vec4.
func Lower(m hir.Material) (ir.Module, bindings.Layout, error) {
	fail := func(format string, a ...any) (ir.Module, bindings.Layout, error) {
		return ir.Module{}, bindings.Layout{}, fmt.Errorf(format, a...)
	}

	paramKind := map[string]hir.Type{}
	for _, p := range m.Params {
		paramKind[p.Name] = p.Type
	}

	// --- ordered uniforms: implicit Transform first, then params/records ---
	var uniforms []bindings.NamedType
	uniType := map[string]ir.Type{}
	addUniform := func(name string, t ir.Type) {
		uniforms = append(uniforms, bindings.NamedType{Name: name, Type: t})
		uniType[name] = t
	}
	addUniform("mvp", ir.Mat4)
	addUniform("normalMatrix", ir.Mat3)

	uniformOf := map[string]string{} // "baseColor" or "light.dir" -> uniform name
	for _, p := range m.Params {
		switch p.Type {
		case hir.Sun:
			for _, f := range sortedKeys(sunFields) {
				un := p.Name + "_" + f
				addUniform(un, sunFields[f])
				uniformOf[p.Name+"."+f] = un
			}
		case hir.Texture2D:
			return fail("textures are not supported until M2 (param %q)", p.Name)
		default:
			t, ok := hirToIRType(p.Type)
			if !ok {
				return fail("param %q: unsupported type %q", p.Name, p.Type)
			}
			addUniform(p.Name, t)
			uniformOf[p.Name] = p.Name
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
		if cg, ok := geoComputed[f]; ok {
			for _, a := range cg.attrs {
				attrNeeded[a] = true
			}
			varyingOf[f] = cg.varying
			varyings = append(varyings, ir.Binding{Name: cg.varying, Type: cg.typ})
			vertexBody = append(vertexBody, ir.Stmt{Target: cg.varying, Type: cg.typ, Value: cg.build()})
		} else if at, ok := geoAttr[f]; ok {
			attrNeeded[f] = true
			vn := "v" + title(f)
			varyingOf[f] = vn
			varyings = append(varyings, ir.Binding{Name: vn, Type: at})
			vertexBody = append(vertexBody, ir.Stmt{Target: vn, Type: at, Value: ir.Ref{Name: f}})
		} else {
			return fail("unknown geometry field geo.%s", f)
		}
	}

	// --- attributes: position first (location 0), then the rest, deterministically ---
	attributes := []ir.Binding{{Name: "position", Type: geoAttr["position"]}}
	for _, a := range sortedSet(attrNeeded) {
		if a != "position" {
			attributes = append(attributes, ir.Binding{Name: a, Type: geoAttr[a]})
		}
	}

	rs := &resolver{paramKind: paramKind, uniformOf: uniformOf, varyingOf: varyingOf, geo: m.Surface.Geo}
	tp := &typer{paramKind: paramKind, geo: m.Surface.Geo, locals: map[string]ir.Type{}}

	// --- fragment body: type each local, then lower it ---
	var fragBody []ir.Stmt
	for _, l := range m.Surface.Body {
		t, err := tp.typeOf(l.Value)
		if err != nil {
			return fail("surface local %q: %w", l.Name, err)
		}
		ve, err := rs.expr(l.Value)
		if err != nil {
			return fail("surface local %q: %w", l.Name, err)
		}
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

	mod := ir.Module{
		Name:       m.Name,
		Uniforms:   toBindings(uniforms),
		Attributes: attributes,
		Varyings:   varyings,
		Vertex:     ir.Stage{Body: vertexBody, Output: vertexOut},
		Fragment:   ir.Stage{Body: fragBody, Output: fragOut},
	}
	layout := bindings.Layout{
		Material:     m.Name,
		UniformBlock: bindings.ComputeUniformBlock(uniforms),
		Attributes:   toAttrs(attributes),
		Textures:     []bindings.Texture{},
		WGSL:         bindings.WGSLBinding{Group: 0, Binding: 0},
		Metal:        bindings.MetalBinding{Buffer: 0},
	}
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
		base, ok := x.E.(hir.Ref)
		if !ok {
			return nil, fmt.Errorf("only simple member access (a.b) is supported")
		}
		if base.Name == r.geo {
			vn, ok := r.varyingOf[x.Field]
			if !ok {
				return nil, fmt.Errorf("geo.%s is not available in the surface", x.Field)
			}
			return ir.Ref{Name: vn}, nil
		}
		if r.paramKind[base.Name] == hir.Sun {
			un, ok := r.uniformOf[base.Name+"."+x.Field]
			if !ok {
				return nil, fmt.Errorf("Sun has no field %q", x.Field)
			}
			return ir.Ref{Name: un}, nil
		}
		return nil, fmt.Errorf("cannot access .%s on %q", x.Field, base.Name)
	case hir.Call:
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
			return "", fmt.Errorf("%q has record type %q and must be accessed by field", x.Name, pk)
		}
		return "", fmt.Errorf("unknown name %q", x.Name)
	case hir.Member:
		base, ok := x.E.(hir.Ref)
		if !ok {
			return "", fmt.Errorf("only simple member access is supported")
		}
		if base.Name == t.geo {
			if cg, ok := geoComputed[x.Field]; ok {
				return cg.typ, nil
			}
			if at, ok := geoAttr[x.Field]; ok {
				return at, nil
			}
			return "", fmt.Errorf("unknown geometry field geo.%s", x.Field)
		}
		if t.paramKind[base.Name] == hir.Sun {
			if ft, ok := sunFields[x.Field]; ok {
				return ft, nil
			}
			return "", fmt.Errorf("Sun has no field %q", x.Field)
		}
		return "", fmt.Errorf("cannot access .%s on %q", x.Field, base.Name)
	case hir.Call:
		return t.callType(x)
	case hir.Binary:
		return t.binaryType(x)
	}
	return "", fmt.Errorf("unsupported expression %T", e)
}

func (t *typer) callType(c hir.Call) (ir.Type, error) {
	switch c.Func {
	case "dot", "length", "distance":
		return ir.Float, nil
	case "rgb":
		if len(c.Args) == 4 {
			return ir.Vec4, nil
		}
		return ir.Vec3, nil
	default: // normalize, max, min, clamp, mix, pow, abs, … take the type of arg0
		if len(c.Args) == 0 {
			return "", fmt.Errorf("call %q has no arguments", c.Func)
		}
		return t.typeOf(c.Args[0])
	}
}

func (t *typer) binaryType(b hir.Binary) (ir.Type, error) {
	lt, err := t.typeOf(b.L)
	if err != nil {
		return "", err
	}
	rt, err := t.typeOf(b.R)
	if err != nil {
		return "", err
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
	return lt, nil
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

func title(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
