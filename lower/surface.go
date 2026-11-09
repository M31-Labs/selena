package lower

import (
	"fmt"
	"strconv"

	"m31labs.dev/selena/bindings"
	"m31labs.dev/selena/hir"
	"m31labs.dev/selena/ir"
)

// Surface-kind lowering.
//
// A surface material returns the typed Surface record instead of a color:
//
//	surface(geo) -> Surface {
//	    let s = pbr(albedo, 0.8, 0.0)
//	    return withNormal(s, n)
//	}
//
// The record never reaches the IR. desugarSurface rewrites every Surface value
// into one local per ir.SurfaceFields entry (s_baseColor, s_roughness, ...), so
// the rest of lowering, the typer and every IR walker see ordinary scalar and
// vector locals. The fragment body then ends with one selenaOut_<field> local
// per field, named in ir.Module.SurfaceOutputs, which the emitters return as
// the SelenaSurface record.

// surfaceGeoVaryingPrefix names the IR varying that carries one
// SelenaSurfaceGeo field. Emitters spell it as a member of the hook's geo
// parameter (selenaGeo.<field>).
const surfaceGeoVaryingPrefix = "selenaGeo_"

// surfaceOutputPrefix names the trailing fragment local of each Surface field.
const surfaceOutputPrefix = "selenaOut_"

// surfaceGLUniformPrefix is prepended to param uniform names in the GLES hook.
const surfaceGLUniformPrefix = "selena_"

var surfaceGeometry = func() map[string]geometrySpec {
	out := make(map[string]geometrySpec, len(ir.SurfaceGeoFields))
	for _, f := range ir.SurfaceGeoFields {
		out[f.Name] = geometrySpec{typ: f.Type, varying: surfaceGeoVaryingPrefix + f.Name}
	}
	return out
}()

// surfaceBuiltins are the calls that produce a Surface value.
var surfaceBuiltins = map[string]int{
	"pbr":              3, // pbr(baseColor, roughness, metallic)
	"mixSurface":       3, // mixSurface(a, b, t)
	"withBaseColor":    2,
	"withAlpha":        2,
	"withNormal":       2,
	"withRoughness":    2,
	"withMetallic":     2,
	"withOcclusion":    2,
	"withEmissive":     2,
	"withClearcoat":    3, // withClearcoat(s, amount, roughness)
	"withSheen":        2,
	"withTransmission": 2,
}

// surfaceWithField maps a single-value with* builtin to the field it sets.
var surfaceWithField = map[string]string{
	"withBaseColor":    "baseColor",
	"withAlpha":        "alpha",
	"withNormal":       "normal",
	"withRoughness":    "roughness",
	"withMetallic":     "metallic",
	"withOcclusion":    "occlusion",
	"withEmissive":     "emissive",
	"withSheen":        "sheen",
	"withTransmission": "transmission",
}

func surfaceFieldIndex(name string) (int, bool) {
	for i, f := range ir.SurfaceFields {
		if f.Name == name {
			return i, true
		}
	}
	return 0, false
}

func surfaceFieldNames() string {
	s := ""
	for i, f := range ir.SurfaceFields {
		if i > 0 {
			s += ", "
		}
		s += f.Name
	}
	return s
}

type surfaceLocal struct {
	fields  []string // local name per ir.SurfaceFields entry
	mutable bool
}

type surfaceDesugar struct {
	geo     string
	names   map[string]bool
	scopes  []map[string]*surfaceLocal
	pending []hir.Stmt
	serial  int
}

// desugarSurface rewrites the surface body so no Surface-typed value remains
// and returns the body plus one expression per ir.SurfaceFields entry.
func desugarSurface(m hir.Material) ([]hir.Stmt, []hir.Expr, error) {
	d := &surfaceDesugar{geo: m.Surface.Geo, names: materialNames(m), scopes: []map[string]*surfaceLocal{{}}}
	body, err := d.stmts(m.Surface.Body)
	if err != nil {
		return nil, nil, err
	}
	if m.Surface.Result == nil {
		return nil, nil, diagnostic(CodeUnsupportedFeat, m.Surface.Span, "surface must return a Surface")
	}
	if !d.isSurface(m.Surface.Result) {
		return nil, nil, diagnostic(CodeTypeMismatch, hir.ExprSpan(m.Surface.Result),
			"surface(geo) -> Surface must return a Surface value, for example pbr(baseColor, roughness, metallic)")
	}
	outs, err := d.fields(m.Surface.Result)
	if err != nil {
		return nil, nil, err
	}
	body = append(body, d.pending...)
	d.pending = nil
	return body, outs, nil
}

func (d *surfaceDesugar) lookup(name string) *surfaceLocal {
	for i := len(d.scopes) - 1; i >= 0; i-- {
		if l, ok := d.scopes[i][name]; ok {
			return l
		}
	}
	return nil
}

func (d *surfaceDesugar) fresh(base string) string {
	name := base
	for n := 1; d.names[name]; n++ {
		name = base + strconv.Itoa(n)
	}
	d.names[name] = true
	return name
}

func (d *surfaceDesugar) isSurface(e hir.Expr) bool {
	switch x := e.(type) {
	case hir.Ref:
		return d.lookup(x.Name) != nil
	case hir.Call:
		_, ok := surfaceBuiltins[x.Func]
		return ok
	case hir.Conditional:
		return d.isSurface(x.Then) || d.isSurface(x.Alt)
	}
	return false
}

// trivial reports whether e is cheap and side-effect free to repeat in every
// field expression: a name, a literal, or a member of a name.
func trivial(e hir.Expr) bool {
	switch x := e.(type) {
	case hir.Ref, hir.Lit, hir.IntLit, hir.UintLit:
		return true
	case hir.Member:
		_, ok := x.E.(hir.Ref)
		return ok
	}
	return false
}

// bind stores a non-trivial expression in a fresh let so field expressions can
// share it without repeating its work.
func (d *surfaceDesugar) bind(e hir.Expr, span hir.Span) hir.Expr {
	if trivial(e) {
		return e
	}
	name := d.fresh("selenaSurf" + strconv.Itoa(d.serial))
	d.serial++
	d.pending = append(d.pending, hir.Let{Name: name, Value: e, Span: span})
	return hir.Ref{Name: name, Span: span}
}

func (d *surfaceDesugar) defaults(baseColor, roughness, metallic hir.Expr, span hir.Span) []hir.Expr {
	zero3 := hir.Call{Func: "vec3f", Args: []hir.Expr{hir.Lit{Value: 0}, hir.Lit{Value: 0}, hir.Lit{Value: 0}}, Span: span}
	normal := hir.Call{Func: "normalize", Args: []hir.Expr{hir.Member{E: hir.Ref{Name: d.geo, Span: span}, Field: "worldNormal", Span: span}}, Span: span}
	values := map[string]hir.Expr{
		"baseColor":          baseColor,
		"alpha":              hir.Lit{Value: 1, Span: span},
		"normal":             normal,
		"roughness":          roughness,
		"metallic":           metallic,
		"occlusion":          hir.Lit{Value: 1, Span: span},
		"emissive":           zero3,
		"clearcoat":          hir.Lit{Value: 0, Span: span},
		"clearcoatRoughness": hir.Lit{Value: 0.03, Span: span},
		"sheen":              hir.Lit{Value: 0, Span: span},
		"transmission":       hir.Lit{Value: 0, Span: span},
	}
	out := make([]hir.Expr, len(ir.SurfaceFields))
	for i, f := range ir.SurfaceFields {
		out[i] = values[f.Name]
	}
	return out
}

// fields resolves a Surface-valued expression to one expression per field.
func (d *surfaceDesugar) fields(e hir.Expr) ([]hir.Expr, error) {
	switch x := e.(type) {
	case hir.Ref:
		l := d.lookup(x.Name)
		if l == nil {
			return nil, diagnostic(CodeTypeMismatch, x.Span, "%q is not a Surface", x.Name)
		}
		out := make([]hir.Expr, len(l.fields))
		for i, name := range l.fields {
			out[i] = hir.Ref{Name: name, Span: x.Span}
		}
		return out, nil
	case hir.Conditional:
		if !d.isSurface(x.Then) || !d.isSurface(x.Alt) {
			return nil, diagnostic(CodeTypeMismatch, x.Span, "both branches of a Surface conditional must be Surface values")
		}
		cond, err := d.expr(x.Cond)
		if err != nil {
			return nil, err
		}
		cond = d.bind(cond, x.Span)
		a, err := d.sharedFields(x.Then, x.Span)
		if err != nil {
			return nil, err
		}
		b, err := d.sharedFields(x.Alt, x.Span)
		if err != nil {
			return nil, err
		}
		out := make([]hir.Expr, len(a))
		for i := range a {
			out[i] = hir.Conditional{Cond: cond, Then: a[i], Alt: b[i], Span: x.Span}
		}
		return out, nil
	case hir.Call:
		arity, ok := surfaceBuiltins[x.Func]
		if !ok {
			break
		}
		if len(x.Args) != arity {
			return nil, diagnostic(CodeInvalidCall, x.Span, "%s expects %d arguments, got %d", x.Func, arity, len(x.Args))
		}
		switch x.Func {
		case "pbr":
			args := make([]hir.Expr, 3)
			for i, a := range x.Args {
				if d.isSurface(a) {
					return nil, diagnostic(CodeInvalidCall, x.Span, "pbr(baseColor, roughness, metallic) takes values, not a Surface")
				}
				v, err := d.expr(a)
				if err != nil {
					return nil, err
				}
				args[i] = v
			}
			return d.defaults(args[0], args[1], args[2], x.Span), nil
		case "mixSurface":
			if !d.isSurface(x.Args[0]) || !d.isSurface(x.Args[1]) {
				return nil, diagnostic(CodeInvalidCall, x.Span, "mixSurface(a, b, t) needs two Surface values")
			}
			a, err := d.sharedFields(x.Args[0], x.Span)
			if err != nil {
				return nil, err
			}
			b, err := d.sharedFields(x.Args[1], x.Span)
			if err != nil {
				return nil, err
			}
			t, err := d.expr(x.Args[2])
			if err != nil {
				return nil, err
			}
			t = d.bind(t, x.Span)
			out := make([]hir.Expr, len(a))
			for i, f := range ir.SurfaceFields {
				mixed := hir.Expr(hir.Call{Func: "mix", Args: []hir.Expr{a[i], b[i], t}, Span: x.Span})
				if f.Name == "normal" {
					mixed = hir.Call{Func: "normalize", Args: []hir.Expr{mixed}, Span: x.Span}
				}
				out[i] = mixed
			}
			return out, nil
		default:
			if !d.isSurface(x.Args[0]) {
				return nil, diagnostic(CodeInvalidCall, x.Span, "%s(s, ...) needs a Surface as its first argument", x.Func)
			}
			out, err := d.fields(x.Args[0])
			if err != nil {
				return nil, err
			}
			values := make([]hir.Expr, len(x.Args)-1)
			for i, a := range x.Args[1:] {
				if d.isSurface(a) {
					return nil, diagnostic(CodeInvalidCall, x.Span, "%s takes a value after the Surface, not another Surface", x.Func)
				}
				v, err := d.expr(a)
				if err != nil {
					return nil, err
				}
				values[i] = v
			}
			if x.Func == "withClearcoat" {
				ci, _ := surfaceFieldIndex("clearcoat")
				ri, _ := surfaceFieldIndex("clearcoatRoughness")
				out[ci], out[ri] = values[0], values[1]
				return out, nil
			}
			idx, _ := surfaceFieldIndex(surfaceWithField[x.Func])
			out[idx] = values[0]
			return out, nil
		}
	}
	return nil, diagnostic(CodeTypeMismatch, hir.ExprSpan(e), "expected a Surface value")
}

// sharedFields resolves e and binds every non-trivial field so a caller can
// read each field more than once without repeating its work.
func (d *surfaceDesugar) sharedFields(e hir.Expr, span hir.Span) ([]hir.Expr, error) {
	fs, err := d.fields(e)
	if err != nil {
		return nil, err
	}
	for i := range fs {
		fs[i] = d.bind(fs[i], span)
	}
	return fs, nil
}

// expr rewrites a non-Surface expression: s.field reads become the field
// local, and a Surface value in any other position is an error.
func (d *surfaceDesugar) expr(e hir.Expr) (hir.Expr, error) {
	switch x := e.(type) {
	case hir.Ref:
		if d.lookup(x.Name) != nil {
			return nil, diagnostic(CodeTypeMismatch, x.Span, "Surface %q used as a value; read a field such as %s.roughness", x.Name, x.Name)
		}
		return x, nil
	case hir.Member:
		if d.isSurface(x.E) {
			idx, ok := surfaceFieldIndex(x.Field)
			if !ok {
				return nil, diagnostic(CodeInvalidMember, x.Span, "Surface has no field %q (fields: %s)", x.Field, surfaceFieldNames())
			}
			fs, err := d.fields(x.E)
			if err != nil {
				return nil, err
			}
			return fs[idx], nil
		}
		inner, err := d.expr(x.E)
		if err != nil {
			return nil, err
		}
		x.E = inner
		return x, nil
	case hir.Call:
		if _, ok := surfaceBuiltins[x.Func]; ok {
			return nil, diagnostic(CodeTypeMismatch, x.Span, "%s returns a Surface; bind it with let or read a field", x.Func)
		}
		args := make([]hir.Expr, len(x.Args))
		for i, a := range x.Args {
			v, err := d.expr(a)
			if err != nil {
				return nil, err
			}
			args[i] = v
		}
		x.Args = args
		return x, nil
	case hir.Binary:
		l, err := d.expr(x.L)
		if err != nil {
			return nil, err
		}
		r, err := d.expr(x.R)
		if err != nil {
			return nil, err
		}
		x.L, x.R = l, r
		return x, nil
	case hir.Unary:
		inner, err := d.expr(x.E)
		if err != nil {
			return nil, err
		}
		x.E = inner
		return x, nil
	case hir.Conditional:
		if d.isSurface(x.Then) || d.isSurface(x.Alt) {
			return nil, diagnostic(CodeTypeMismatch, x.Span, "a Surface conditional can only be bound with let or var")
		}
		c, err := d.expr(x.Cond)
		if err != nil {
			return nil, err
		}
		a, err := d.expr(x.Then)
		if err != nil {
			return nil, err
		}
		b, err := d.expr(x.Alt)
		if err != nil {
			return nil, err
		}
		x.Cond, x.Then, x.Alt = c, a, b
		return x, nil
	case hir.IndexExpr:
		arr, err := d.expr(x.Arr)
		if err != nil {
			return nil, err
		}
		idx, err := d.expr(x.Index)
		if err != nil {
			return nil, err
		}
		x.Arr, x.Index = arr, idx
		return x, nil
	}
	return e, nil
}

func (d *surfaceDesugar) block(ss []hir.Stmt) ([]hir.Stmt, error) {
	saved := d.pending
	d.pending = nil
	d.scopes = append(d.scopes, map[string]*surfaceLocal{})
	out, err := d.stmts(ss)
	d.scopes = d.scopes[:len(d.scopes)-1]
	d.pending = saved
	return out, err
}

func (d *surfaceDesugar) stmts(ss []hir.Stmt) ([]hir.Stmt, error) {
	var out []hir.Stmt
	for _, s := range ss {
		expanded, err := d.stmt(s)
		if err != nil {
			return nil, err
		}
		out = append(out, d.pending...)
		d.pending = nil
		out = append(out, expanded...)
	}
	return out, nil
}

func (d *surfaceDesugar) declare(name string, value hir.Expr, mutable bool, span hir.Span) ([]hir.Stmt, error) {
	fs, err := d.fields(value)
	if err != nil {
		return nil, err
	}
	l := &surfaceLocal{fields: make([]string, len(fs)), mutable: mutable}
	out := make([]hir.Stmt, len(fs))
	for i, f := range ir.SurfaceFields {
		l.fields[i] = d.fresh(name + "_" + f.Name)
		if mutable {
			out[i] = hir.VarDecl{Name: l.fields[i], Value: fs[i], Span: span}
		} else {
			out[i] = hir.Let{Name: l.fields[i], Value: fs[i], Span: span}
		}
	}
	d.scopes[len(d.scopes)-1][name] = l
	return out, nil
}

func (d *surfaceDesugar) stmt(s hir.Stmt) ([]hir.Stmt, error) {
	switch x := s.(type) {
	case hir.Let:
		if x.Expected == hir.SurfaceT || d.isSurface(x.Value) {
			if !d.isSurface(x.Value) {
				return nil, diagnostic(CodeInvalidCall, x.Span, "library binding expects Surface, got a non-Surface value")
			}
			return d.declare(x.Name, x.Value, false, x.Span)
		}
		v, err := d.expr(x.Value)
		if err != nil {
			return nil, err
		}
		x.Value = v
		return []hir.Stmt{x}, nil
	case hir.VarDecl:
		if d.isSurface(x.Value) {
			return d.declare(x.Name, x.Value, true, x.Span)
		}
		v, err := d.expr(x.Value)
		if err != nil {
			return nil, err
		}
		x.Value = v
		return []hir.Stmt{x}, nil
	case hir.Assign:
		if l := d.lookup(x.Name); l != nil {
			if !l.mutable {
				return nil, diagnostic(CodeDuplicateLocal, x.Span, "cannot assign to immutable Surface %q (use var instead of let)", x.Name)
			}
			if !d.isSurface(x.Value) {
				return nil, diagnostic(CodeTypeMismatch, x.Span, "assignment to Surface %q needs a Surface value", x.Name)
			}
			fs, err := d.fields(x.Value)
			if err != nil {
				return nil, err
			}
			// Evaluate every new field before writing any, so a value that
			// reads another field of the same Surface sees the old record.
			var out []hir.Stmt
			for i := range fs {
				if r, ok := fs[i].(hir.Ref); ok && r.Name == l.fields[i] {
					fs[i] = nil
					continue
				}
				if !trivial(fs[i]) || refersToAny(fs[i], l.fields) {
					fs[i] = d.bind(fs[i], x.Span)
				}
			}
			for i := range fs {
				if fs[i] != nil {
					out = append(out, hir.Assign{Name: l.fields[i], Value: fs[i], Span: x.Span})
				}
			}
			return out, nil
		}
		v, err := d.expr(x.Value)
		if err != nil {
			return nil, err
		}
		x.Value = v
		return []hir.Stmt{x}, nil
	case hir.If:
		c, err := d.expr(x.Cond)
		if err != nil {
			return nil, err
		}
		then, err := d.block(x.Then)
		if err != nil {
			return nil, err
		}
		var els []hir.Stmt
		if len(x.Else) > 0 {
			if els, err = d.block(x.Else); err != nil {
				return nil, err
			}
		}
		return []hir.Stmt{hir.If{Cond: c, Then: then, Else: els, Span: x.Span}}, nil
	case hir.For:
		iv, err := d.expr(x.InitValue)
		if err != nil {
			return nil, err
		}
		c, err := d.expr(x.Cond)
		if err != nil {
			return nil, err
		}
		pv, err := d.expr(x.PostValue)
		if err != nil {
			return nil, err
		}
		if len(d.pending) > 0 {
			return nil, diagnostic(CodeUnsupportedFeat, x.Span, "a for loop header cannot read a Surface field; bind it with let first")
		}
		body, err := d.block(x.Body)
		if err != nil {
			return nil, err
		}
		x.InitValue, x.Cond, x.PostValue, x.Body = iv, c, pv, body
		return []hir.Stmt{x}, nil
	case hir.IndexAssign:
		idx, err := d.expr(x.Index)
		if err != nil {
			return nil, err
		}
		v, err := d.expr(x.Value)
		if err != nil {
			return nil, err
		}
		x.Index, x.Value = idx, v
		return []hir.Stmt{x}, nil
	case hir.Return:
		return nil, diagnostic(CodeUnsupportedFeat, x.Span,
			"an early return is not supported in a surface material; assign a var Surface and return it at the end")
	}
	return []hir.Stmt{s}, nil
}

// refersToAny reports whether e references one of names.
func refersToAny(e hir.Expr, names []string) bool {
	found := false
	var walk func(hir.Expr)
	walk = func(e hir.Expr) {
		if found || e == nil {
			return
		}
		switch x := e.(type) {
		case hir.Ref:
			for _, n := range names {
				if x.Name == n {
					found = true
				}
			}
		case hir.Member:
			walk(x.E)
		case hir.Call:
			for _, a := range x.Args {
				walk(a)
			}
		case hir.Binary:
			walk(x.L)
			walk(x.R)
		case hir.Unary:
			walk(x.E)
		case hir.Conditional:
			walk(x.Cond)
			walk(x.Then)
			walk(x.Alt)
		case hir.IndexExpr:
			walk(x.Arr)
			walk(x.Index)
		}
	}
	walk(e)
	return found
}

// lowerSurface compiles a surface-kind material into a fragment hook module.
func lowerSurface(m hir.Material) (ir.Module, bindings.Layout, error) {
	switch {
	case m.Vertex != nil:
		return ir.Module{}, bindings.Layout{}, diagnostic(CodeUnsupportedFeat, m.Vertex.Span,
			"surface materials use the host's standard vertex stage; vertex() is not supported")
	case len(m.Varyings) > 0:
		return ir.Module{}, bindings.Layout{}, diagnostic(CodeUnsupportedFeat, m.Varyings[0].Span, "surface materials cannot declare varyings")
	case len(m.States) > 0:
		return ir.Module{}, bindings.Layout{}, diagnostic(CodeUnsupportedFeat, m.States[0].Span, "surface materials cannot declare a statefield")
	}
	for _, p := range m.Params {
		if p.Type == hir.TextureCube {
			return ir.Module{}, bindings.Layout{}, diagnostic(CodeUnsupportedType, p.Span,
				"param %q: surface materials sample texture2d params only; the host supplies image-based lighting", p.Name)
		}
	}
	for _, c := range m.Context {
		switch c.Type {
		case hir.Texture2D, hir.TextureCube, hir.TextureDepth2D, hir.TextureDepth2DArray:
			return ir.Module{}, bindings.Layout{}, diagnostic(CodeUnsupportedType, c.Span,
				"context %q: surface materials cannot read engine textures; the host's standard lighting does shadows and reflections", c.Name)
		}
	}
	paramKind, err := validateParams(m.Params, m.Context)
	if err != nil {
		return ir.Module{}, bindings.Layout{}, err
	}
	plan, err := buildUniformPlan(m.Params, m.Context)
	if err != nil {
		return ir.Module{}, bindings.Layout{}, err
	}
	// The host owns the transforms; drop the mesh auto-uniforms.
	kept := plan.uniforms[:0]
	for _, u := range plan.uniforms {
		if u.Name != "mvp" && u.Name != "normalMatrix" {
			kept = append(kept, u)
		}
	}
	plan.uniforms = kept
	if len(plan.textures) > bindings.SurfaceMaxTextures {
		return ir.Module{}, bindings.Layout{}, diagnostic(CodeUnsupportedFeat, m.Span,
			"surface material %q samples %d textures; the host binds at most %d", m.Name, len(plan.textures), bindings.SurfaceMaxTextures)
	}

	body, outs, err := desugarSurface(m)
	if err != nil {
		return ir.Module{}, bindings.Layout{}, err
	}

	varyings := make([]ir.Binding, len(ir.SurfaceGeoFields))
	varyingOf := map[string]string{}
	for i, f := range ir.SurfaceGeoFields {
		varyings[i] = ir.Binding{Name: surfaceGeoVaryingPrefix + f.Name, Type: f.Type}
		varyingOf[f.Name] = varyings[i].Name
	}
	reserved, err := interfaceNames(plan.uniforms, plan.textures, nil, varyings)
	if err != nil {
		return ir.Module{}, bindings.Layout{}, err
	}
	rs := &resolver{paramKind: paramKind, uniformOf: plan.uniformOf, varyingOf: varyingOf, geo: m.Surface.Geo, geoFields: surfaceGeometry, allowDiscard: true}
	tp := &typer{paramKind: paramKind, geo: m.Surface.Geo, locals: map[string]ir.Type{}, mutableLocals: map[string]bool{}, paramArrays: plan.paramArrays, geoFields: surfaceGeometry}
	rs.tp = tp
	lc := &lowerCtx{reserved: reserved, rs: rs, tp: tp, localKind: "surface local"}
	irBody, err := lc.lowerStmts(body)
	if err != nil {
		return ir.Module{}, bindings.Layout{}, err
	}
	outNames := make([]string, len(ir.SurfaceFields))
	for i, f := range ir.SurfaceFields {
		t, err := tp.typeOf(outs[i])
		if err != nil {
			return ir.Module{}, bindings.Layout{}, fmt.Errorf("Surface.%s: %w", f.Name, err)
		}
		if t != f.Type {
			return ir.Module{}, bindings.Layout{}, diagnostic(CodeTypeMismatch, hir.ExprSpan(outs[i]), "Surface.%s must be %s, got %s", f.Name, f.Type, t)
		}
		v, err := rs.expr(outs[i])
		if err != nil {
			return ir.Module{}, bindings.Layout{}, fmt.Errorf("Surface.%s: %w", f.Name, err)
		}
		outNames[i] = surfaceOutputPrefix + f.Name
		irBody = append(irBody, ir.Stmt{Target: outNames[i], Type: t, Value: v})
	}

	mod := ir.Module{
		Name:           m.Name,
		Kind:           ir.KindSurface,
		Uniforms:       toBindings(plan.uniforms),
		ArrayUniforms:  toArrayBindings(plan.uniforms),
		Varyings:       varyings,
		Textures:       planIRTextures(plan),
		Fragment:       ir.Stage{Body: irBody, Output: ir.Lit{Value: 0}},
		SurfaceOutputs: outNames,
	}
	ub := bindings.ComputeUniformBlock(plan.uniforms)
	stampContextClass(ub.Fields, plan.contextNames)
	ub.Defaults = plan.defaults
	textures := planLayoutTextures(plan)
	names := make([]string, len(textures))
	for i := range textures {
		names[i] = ir.SurfaceTextureName(i)
		textures[i].WGSL = bindings.WGSLTexBinding{}
	}
	if textures == nil {
		textures = []bindings.Texture{}
	}
	layout := bindings.Layout{
		SchemaVersion:   bindings.DescriptorSchemaVersion,
		LanguageVersion: bindings.LanguageVersion,
		Material:        m.Name,
		Kind:            bindings.SurfaceKindSurface,
		EntryPoints:     bindings.EntryPoints{Fragment: "selenaSurface"},
		UniformBlock:    ub,
		Attributes:      []bindings.Attribute{},
		Textures:        textures,
		WGSL:            bindings.WGSLBinding{Group: 2, Binding: 0},
		Context:         plan.contextNames,
		Surface: &bindings.SurfaceHook{
			ABI:             bindings.SurfaceABI,
			Function:        "selenaSurface",
			TextureNames:    names,
			GLUniformPrefix: surfaceGLUniformPrefix,
		},
	}
	return mod, layout, nil
}
