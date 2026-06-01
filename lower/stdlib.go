package lower

import (
	"m31labs.dev/selena/ir"
)

type stdlibRegistry struct {
	records  map[string]recordSpec
	geometry map[string]geometrySpec
	builtins map[string]builtinSpec
}

type recordSpec struct {
	fields map[string]ir.Type
}

type geometrySpec struct {
	typ     ir.Type
	varying string
	attrs   []string
	build   func() ir.Expr
}

type builtinKind string

const (
	builtinDot          builtinKind = "dot"
	builtinLength       builtinKind = "length"
	builtinDistance     builtinKind = "distance"
	builtinSample       builtinKind = "sample"
	builtinRGB          builtinKind = "rgb"
	builtinUnarySame    builtinKind = "unary_same"
	builtinSameOrScalar builtinKind = "same_or_scalar"
)

type builtinSpec struct {
	kind  builtinKind
	arity int
}

var stdlib = stdlibRegistry{
	records: map[string]recordSpec{
		"Sun": {
			fields: map[string]ir.Type{
				"ambient": ir.Float,
				"dir":     ir.Vec3,
			},
		},
	},
	geometry: map[string]geometrySpec{
		"position": {
			typ:     ir.Vec3,
			varying: "vPosition",
			attrs:   []string{"position"},
			build:   func() ir.Expr { return ir.Ref{Name: "position"} },
		},
		"normal": {
			typ:     ir.Vec3,
			varying: "vNormal",
			attrs:   []string{"normal"},
			build:   func() ir.Expr { return ir.Ref{Name: "normal"} },
		},
		"uv": {
			typ:     ir.Vec2,
			varying: "vUv",
			attrs:   []string{"uv"},
			build:   func() ir.Expr { return ir.Ref{Name: "uv"} },
		},
		"worldNormal": {
			typ:     ir.Vec3,
			varying: "vWorldNormal",
			attrs:   []string{"normal"},
			build: func() ir.Expr {
				return ir.Call{Func: "normalize", Args: []ir.Expr{
					ir.Binary{Op: "*", L: ir.Ref{Name: "normalMatrix"}, R: ir.Ref{Name: "normal"}},
				}}
			},
		},
	},
	builtins: map[string]builtinSpec{
		"dot":       {kind: builtinDot, arity: 2},
		"length":    {kind: builtinLength, arity: 1},
		"distance":  {kind: builtinDistance, arity: 2},
		"sample":    {kind: builtinSample, arity: 2},
		"rgb":       {kind: builtinRGB},
		"normalize": {kind: builtinUnarySame, arity: 1},
		"abs":       {kind: builtinUnarySame, arity: 1},
		"max":       {kind: builtinSameOrScalar, arity: 2},
		"min":       {kind: builtinSameOrScalar, arity: 2},
		"pow":       {kind: builtinSameOrScalar, arity: 2},
		"clamp":     {kind: builtinSameOrScalar, arity: 3},
		"mix":       {kind: builtinSameOrScalar, arity: 3},
	},
}

func (r stdlibRegistry) recordField(record, field string) (ir.Type, bool) {
	spec, ok := r.records[record]
	if !ok {
		return "", false
	}
	t, ok := spec.fields[field]
	return t, ok
}

func (r stdlibRegistry) recordFields(record string) map[string]ir.Type {
	spec, ok := r.records[record]
	if !ok {
		return nil
	}
	return spec.fields
}

func (r stdlibRegistry) geometryField(name string) (geometrySpec, bool) {
	spec, ok := r.geometry[name]
	return spec, ok
}

func (r stdlibRegistry) builtin(name string) (builtinSpec, bool) {
	spec, ok := r.builtins[name]
	return spec, ok
}
