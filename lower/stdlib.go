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
	builtinCross        builtinKind = "cross"
	builtinStep         builtinKind = "step"
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
		// Component-wise unary math: float->float, vecN->vecN.
		"sin":   {kind: builtinUnarySame, arity: 1},
		"cos":   {kind: builtinUnarySame, arity: 1},
		"tan":   {kind: builtinUnarySame, arity: 1},
		"sqrt":  {kind: builtinUnarySame, arity: 1},
		"floor": {kind: builtinUnarySame, arity: 1},
		"ceil":  {kind: builtinUnarySame, arity: 1},
		"fract": {kind: builtinUnarySame, arity: 1},
		"sign":  {kind: builtinUnarySame, arity: 1},
		"exp":   {kind: builtinUnarySame, arity: 1},
		"log":   {kind: builtinUnarySame, arity: 1},
		"exp2":  {kind: builtinUnarySame, arity: 1},
		"log2":  {kind: builtinUnarySame, arity: 1},
		// reflect(I, N) -> same vector as I.
		"reflect": {kind: builtinSameOrScalar, arity: 2},
		// cross(a, b): vec3 x vec3 -> vec3.
		"cross": {kind: builtinCross, arity: 2},
		// step(edge, x) / smoothstep(e0, e1, x): result is the last argument's type.
		"step":       {kind: builtinStep, arity: 2},
		"smoothstep": {kind: builtinStep, arity: 3},
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
