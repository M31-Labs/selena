package lsp

// builtinDoc documents one Selena builtin. T stands for float or a float
// vector; a "T or float" argument may also be a scalar.
type builtinDoc struct {
	Signature string
	Doc       string
}

func unary(name, what string) builtinDoc {
	return builtinDoc{name + "(x: T) -> T", what + ", per component."}
}

// builtinDocs covers every name lower.Builtins returns; a test keeps them equal.
var builtinDocs = map[string]builtinDoc{
	"abs":             unary("abs", "Absolute value"),
	"acos":            unary("acos", "Arc cosine in radians"),
	"asin":            unary("asin", "Arc sine in radians"),
	"atan":            unary("atan", "Arc tangent in radians"),
	"atan2":           {"atan2(y: T, x: T) -> T", "Arc tangent of y/x in radians, using both signs to pick the quadrant."},
	"ceil":            unary("ceil", "Smallest integer value not less than x"),
	"clamp":           {"clamp(x: T, lo: T or float, hi: T or float) -> T", "Limits x to the range lo..hi."},
	"cos":             unary("cos", "Cosine of an angle in radians"),
	"cross":           {"cross(a: vec3, b: vec3) -> vec3", "Cross product."},
	"distance":        {"distance(a: T, b: T) -> float", "Distance between two points."},
	"dot":             {"dot(a: T, b: T) -> float", "Dot product."},
	"dpdx":            unary("dpdx", "Screen-space derivative along x (surface() only)"),
	"dpdy":            unary("dpdy", "Screen-space derivative along y (surface() only)"),
	"exp":             unary("exp", "Natural exponent e^x"),
	"exp2":            unary("exp2", "Base-2 exponent 2^x"),
	"float":           {"float(x: int or uint) -> float", "Converts an integer to float."},
	"floor":           unary("floor", "Largest integer value not greater than x"),
	"fract":           unary("fract", "Fractional part x - floor(x)"),
	"fwidth":          unary("fwidth", "abs(dpdx(x)) + abs(dpdy(x)), the per-pixel change (surface() only)"),
	"int":             {"int(x: float or uint) -> int", "Converts to a signed integer, truncating toward zero."},
	"length":          {"length(v: T) -> float", "Length of a vector."},
	"load":            {"load(tex: texture2d, ix: int, iy: int) -> vec4", "Reads texel (ix, iy) of the base level exactly, with no filtering. One-channel formats return (r, 0, 0, 1). Clamp the coordinates: out-of-range reads differ per target."},
	"log":             unary("log", "Natural logarithm"),
	"log2":            unary("log2", "Base-2 logarithm"),
	"max":             {"max(a: T, b: T or float) -> T", "Larger of two values, per component."},
	"min":             {"min(a: T, b: T or float) -> T", "Smaller of two values, per component."},
	"mix":             {"mix(a: T, b: T, t: T or float) -> T", "Linear blend a*(1-t) + b*t."},
	"mod":             {"mod(x: T, y: T or float) -> T", "x - y*floor(x/y), the same on every target."},
	"normalize":       {"normalize(v: T) -> T", "Unit vector in the direction of v."},
	"pow":             {"pow(x: T, y: T or float) -> T", "x raised to y; undefined for x < 0."},
	"reflect":         {"reflect(i: T, n: T) -> T", "Reflects incident direction i about normal n."},
	"refract":         {"refract(i: T, n: T, eta: float) -> T", "Refracts incident direction i through normal n with index ratio eta."},
	"rgb":             {"rgb(r: float, g: float, b: float) -> color", "Builds a linear color. rgb(r, g, b, a) builds a vec4 with alpha, as post materials return."},
	"round":           unary("round", "Nearest integer value"),
	"sample":          {"sample(tex: texture2d, uv: vec2) -> vec4", "Filtered texture read with automatic mip selection. surface() only; inside branches and in vertex() use sampleLevel."},
	"sampleCube":      {"sampleCube(tex: textureCube, dir: vec3) -> vec4", "Samples a cube map along a direction."},
	"sampleCubeLevel": {"sampleCubeLevel(tex: textureCube, dir: vec3, lod: float) -> vec4", "Samples a cube map at an explicit mip level, for roughness-filtered environments."},
	"sampleDepth":     {"sampleDepth(tex: textureDepth2d, uv: vec2) -> float", "Reads depth 0..1 from an engine depth texture at its base level. sampleDepth(tex, uv, layer) reads one cascade of a textureDepth2dArray shadow map. Works in vertex() and inside branches."},
	"sampleLevel":     {"sampleLevel(tex: texture2d, uv: vec2, lod: float) -> vec4", "Filtered texture read at an explicit mip level. Allowed in vertex() and inside branches."},
	"sceneColor":      {"sceneColor(uv: vec2) -> vec4", "Backdrop color at a screen uv. Post materials only."},
	"sceneColorLevel": {"sceneColorLevel(uv: vec2, lod: float) -> vec4", "Backdrop color at a mip level, one pre-filtered tap instead of a blur kernel. Post materials only; sets requires.sceneColorMips."},
	"sceneDepth":      {"sceneDepth(uv: vec2) -> float", "Backdrop depth at a screen uv. Post materials only."},
	"sceneSize":       {"sceneSize() -> vec2", "Backdrop size in pixels. Post materials only."},
	"screenSize":      {"screenSize() -> vec2", "Size in pixels of the target rectangle a screen pass draws into. Screen materials only; the host writes it every draw (class context in the descriptor)."},
	"sign":            unary("sign", "-1, 0 or 1 by the sign of x"),
	"sin":             unary("sin", "Sine of an angle in radians"),
	"size":            {"size(tex: texture2d) -> vec2", "Width and height of the base level in texels."},
	"smoothstep":      {"smoothstep(e0: T or float, e1: T or float, x: T) -> T", "Hermite step from 0 at e0 to 1 at e1."},
	"sqrt":            unary("sqrt", "Square root"),
	"state":           {"state(dx: int, dy: int) -> vec4", "Previous step's statefield value at the cell offset (dx, dy). Feedback materials only."},
	"stateAt":         {"stateAt(uv: vec2) -> vec4", "Live statefield value at a uv, in mesh materials that declare a statefield and author vertex()."},
	"step":            {"step(edge: T or float, x: T) -> T", "0 where x < edge, else 1."},
	"tan":             unary("tan", "Tangent of an angle in radians"),
	"uint":            {"uint(x: float or int) -> uint", "Converts to an unsigned integer."},
	"vec2f":           {"vec2f(x: float, y: float) -> vec2", "Builds a vec2."},
	"vec3f":           {"vec3f(x, y, z) -> vec3", "Builds a vec3 from three floats, or from a vec2 and a float."},
	"vec4f":           {"vec4f(x, y, z, w) -> vec4", "Builds a vec4 from components that add up to four: floats, vec2 and floats, or a vec3 and a float."},
}

// typeDocs documents the type names a declaration can use.
var typeDocs = map[string]string{
	"float":               "32-bit float. Literals without a suffix are floats: `1`, `0.5`.",
	"int":                 "Signed integer. Literals take an `i` suffix: `0i`, `8i`. Array indices and loop counters use it.",
	"uint":                "Unsigned integer. Literals take a `u` suffix: `0u`.",
	"vec2":                "Two-component float vector.",
	"vec3":                "Three-component float vector.",
	"vec4":                "Four-component float vector.",
	"mat3":                "3x3 float matrix, column-major.",
	"mat4":                "4x4 float matrix, column-major.",
	"color":               "Linear RGB color, lowered as vec3. Write defaults with `rgb(r, g, b)`.",
	"Sun":                 "Directional light record: `dir` (vec3, toward the light) and `ambient` (float). Expands into the uniforms light_dir and light_ambient.",
	"texture2d":           "2D texture with a sampler. Read with sample, sampleLevel, or load and size for data textures. `format` sets the storage format.",
	"textureCube":         "Cube-map texture. Read with sampleCube or sampleCubeLevel.",
	"textureDepth2d":      "Engine depth texture (context only), read with sampleDepth.",
	"textureDepth2dArray": "Layered engine depth texture (context only), one layer per shadow cascade, read with sampleDepth(tex, uv, layer).",
	"array":               "Fixed-size array: `array<T, N>`. Params and context fields pack each element at a 16-byte stride. Index with `[i]` and an int.",
}

var keywordList = []string{
	"material", "extends", "kind", "param", "context", "varying", "state",
	"surface", "vertex", "specular", "feedback", "fn", "let", "var", "if",
	"else", "for", "return", "break", "discard", "format", "super",
	"render",
}

var kindDocs = map[string]string{
	"mesh":     "Triangle mesh material: vertex and fragment stages. The default kind.",
	"points":   "Billboard point and particle surface.",
	"post":     "Fullscreen post pass over the rendered scene; reads sceneColor and sceneDepth.",
	"screen":   "Fullscreen pass that draws a color with alpha into a rectangle the host chooses. Takes texture2d and array params, reads screen.uv and screenSize(), and declares blend and depth state in a render block.",
	"feedback": "Per-cell simulation step over a statefield grid; reads state(dx, dy).",
}

var textureFormats = []string{"rgba8unorm", "rgba16float", "rgba32float", "r16float", "r32float"}
