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
	// builtinVec3: vec3f(x, y, z) or vec3f(vec2, z) constructs a vec3.
	// Args must contribute exactly 3 float components.
	builtinVec3 builtinKind = "vec3f"
	// builtinVec4: vec4f(x, y, z, w) / vec4f(vec3, w) / vec4f(vec2, z, w)
	// constructs a vec4. Args must contribute exactly 4 float components.
	builtinVec4 builtinKind = "vec4f"
	// builtinSceneSample: post-pass engine samplers sceneColor(uv) and
	// sceneDepth(uv). These sample engine-provided textures bound at fixed
	// group/binding slots; the author calls them as regular functions.
	builtinSceneSample builtinKind = "scene_sample"
	// builtinCastFloat / builtinCastInt / builtinCastUint: scalar type
	// conversions float(x) / int(x) / uint(x). They lower to an ir.Construct of
	// the target scalar type (WGSL f32()/i32()/u32(), GLSL/GLES/Metal
	// float()/int()/uint()). Needed to build float positions from the uint
	// vertexIndex builtin. B4.
	builtinCastFloat builtinKind = "cast_float"
	builtinCastInt   builtinKind = "cast_int"
	builtinCastUint  builtinKind = "cast_uint"
	// builtinSampleCube: sampleCube(tex, dir) samples a textureCube param by a
	// vec3 direction vector, returning vec4. The first arg must be a textureCube
	// param and the second must be vec3.
	builtinSampleCube builtinKind = "sample_cube"
	// builtinSampleLevel: sampleLevel(tex, uv, lod) samples a texture2d param
	// at an explicit LOD (mip level), returning vec4. The first arg must be a
	// texture2d param, the second vec2 uv, the third a float lod. Unlike
	// sample(), the emitted WGSL (textureSampleLevel) does not require uniform
	// control flow, so authors use it for sample() calls that must live inside
	// an if/for.
	builtinSampleLevel builtinKind = "sample_level"
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
		// worldPos is the world-space surface position. Under the scene3d
		// mesh convention, position attributes arrive pre-baked to world
		// space (mvp = view×proj, no model re-multiply; skinned hosts
		// rebind the attribute to the skinned world-space result), so the
		// varying passes the attribute through unchanged — the same value
		// geo.position carries, named for what it IS on this pipeline so
		// surfaces reading it (fresnel terms against a cameraPos context
		// field, height fades, etc.) say what they mean.
		"worldPos": {
			typ:     ir.Vec3,
			varying: "vWorldPos",
			attrs:   []string{"position"},
			build:   func() ir.Expr { return ir.Ref{Name: "position"} },
		},
	},
	builtins: map[string]builtinSpec{
		"dot":         {kind: builtinDot, arity: 2},
		"length":      {kind: builtinLength, arity: 1},
		"distance":    {kind: builtinDistance, arity: 2},
		"sample":      {kind: builtinSample, arity: 2},
		"sampleLevel": {kind: builtinSampleLevel, arity: 3},
		"sampleCube":  {kind: builtinSampleCube, arity: 2},
		"rgb":         {kind: builtinRGB},
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

		// --- Extended math / vector builtins ---

		// refract(I, N, eta): result = I's type (vecN), eta is float.
		// builtinSameOrScalar: base=I's type; N must match base; eta must be float.
		"refract": {kind: builtinSameOrScalar, arity: 3},

		// mod(x, y): component-wise modulo; y may be scalar.
		// WGSL emits as (x - y * floor(x / y)); GLSL/GLES mod(); Metal fmod().
		"mod": {kind: builtinSameOrScalar, arity: 2},

		// Unary component-wise: float -> float, vecN -> vecN.
		"round": {kind: builtinUnarySame, arity: 1},
		"atan":  {kind: builtinUnarySame, arity: 1},
		"asin":  {kind: builtinUnarySame, arity: 1},
		"acos":  {kind: builtinUnarySame, arity: 1},
		// Screen-space derivative functions (fragment only in practice).
		"dpdx":   {kind: builtinUnarySame, arity: 1},
		"dpdy":   {kind: builtinUnarySame, arity: 1},
		"fwidth": {kind: builtinUnarySame, arity: 1},

		// atan2(y, x): 2-argument arc-tangent; result = y's type.
		// WGSL: atan2(y,x); GLSL/GLES: atan(y,x); Metal: atan2(y,x).
		"atan2": {kind: builtinSameOrScalar, arity: 2},

		// vec3f / vec4f constructors. Args must sum to the right component count.
		// vec3f: accepts (float,float,float) or (vec2,float).
		// vec4f: accepts (float,float,float,float), (vec3,float), (vec2,float,float).
		"vec3f": {kind: builtinVec3, arity: 0}, // variable arity; checked by component count
		"vec4f": {kind: builtinVec4, arity: 0},

		// Scalar type conversions (B4). float(x)/int(x)/uint(x) take one numeric
		// scalar and convert it. Lower to ir.Construct of the target scalar type.
		"float": {kind: builtinCastFloat, arity: 1},
		"int":   {kind: builtinCastInt, arity: 1},
		"uint":  {kind: builtinCastUint, arity: 1},
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
