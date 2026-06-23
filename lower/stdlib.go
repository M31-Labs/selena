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
	// builtinVec2: vec2f(x, y) constructs a vec2 from two floats.
	builtinVec2 builtinKind = "vec2f"
	// builtinSceneSample: post-pass engine samplers sceneColor(uv) and
	// sceneDepth(uv). These sample engine-provided textures bound at fixed
	// group/binding slots; the author calls them as regular functions.
	builtinSceneSample builtinKind = "scene_sample"
)

type builtinSpec struct {
	kind  builtinKind
	arity int
}

// pointGeometry lists the geometry field names available inside a points surface.
// These map to the values plumbed through by the engine billboard quad emitter.
var pointGeometry = map[string]geometrySpec{
	// pointUV: vec2 in [0,1]x[0,1] across the billboard quad face.
	"pointUV": {
		typ:     ir.Vec2,
		varying: "v_pointCoord",
		attrs:   nil,
		build:   func() ir.Expr { return ir.Ref{Name: "v_pointCoord"} },
	},
	// color: rgb base color passed from the point buffer.
	"color": {
		typ:     ir.Vec3,
		varying: "v_color",
		attrs:   nil,
		build:   func() ir.Expr { return ir.Ref{Name: "v_color"} },
	},
	// alpha: per-point alpha, pre-multiplied by material opacity.
	"alpha": {
		typ:     ir.Float,
		varying: "v_alpha",
		attrs:   nil,
		build:   func() ir.Expr { return ir.Ref{Name: "v_alpha"} },
	},
	// pointSize: computed pixel size of this billboard.
	"pointSize": {
		typ:     ir.Float,
		varying: "v_pointSize",
		attrs:   nil,
		build:   func() ir.Expr { return ir.Ref{Name: "v_pointSize"} },
	},
	// fogFactor: precomputed exponential fog factor in [0,1].
	"fogFactor": {
		typ:     ir.Float,
		varying: "v_fogFactor",
		attrs:   nil,
		build:   func() ir.Expr { return ir.Ref{Name: "v_fogFactor"} },
	},
}

// postGeometry lists the geometry field names available inside a post surface.
// These are the screen-space inputs a post-process author can read.
var postGeometry = map[string]geometrySpec{
	// uv: vec2 screen UV [0,1]x[0,1].
	"uv": {
		typ:     ir.Vec2,
		varying: "v_uv",
		attrs:   nil,
		build:   func() ir.Expr { return ir.Ref{Name: "v_uv"} },
	},
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
		// vec2(x, y) constructs a vec2 from two floats.
		"vec2f":     {kind: builtinVec2, arity: 2},
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
		// Engine-provided post-pass samplers. sceneColor(uv)->vec4,
		// sceneDepth(uv)->float. Only valid inside a post-kind surface.
		"sceneColor": {kind: builtinSceneSample, arity: 1},
		"sceneDepth": {kind: builtinSceneSample, arity: 1},
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
