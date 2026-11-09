package bindings

import "strconv"

// EngineVersion identifies the engine input registry. A descriptor carries it
// (Layout.EngineVersion) when the material declares an engine input, so a host
// can refuse a material that needs a newer registry than it implements. Adding
// an entry keeps the version; changing a type, unit or meaning of an entry, or
// removing one, bumps it.
const EngineVersion = "selena.engine.v1"

// Engine input kinds.
const (
	EngineValue   = "value"   // a uniform-block field the host writes every frame
	EngineTexture = "texture" // a texture the host binds; no author-supplied source
)

// EngineInput is one named input the renderer supplies to a material that
// declares it in a `context { ... }` block. Declaring a registered name with a
// different type is a compile error (SEL1007); a name that is not registered
// stays a host-custom context field as before.
type EngineInput struct {
	Name string
	// Type is the Selena type the declaration must use: a value type
	// ("vec3", "float", ...) or a texture type ("texture2d", "textureDepth2d").
	Type string
	Kind string
	// Space names the coordinate space or encoding of the value.
	Space string
	Doc   string
	// Count is the element count of an array input, declared as
	// array<Type, Count>. Zero for a single value or a texture.
	Count int
}

// The scene lighting block. A renderer packs up to SceneLightCapacity lights
// into the `lights` engine input, SceneLightVec4s vec4 values per light, and up
// to ShadowCascadeCapacity cascades into each of the ShadowSlots shadow slots.
// See docs/engine-inputs.md#scene-lighting for the packing.
const (
	SceneLightCapacity    = 8
	SceneLightVec4s       = 4
	ShadowSlots           = 2
	ShadowCascadeCapacity = 4
)

// Scene light kinds, stored in lights[4*i].w.
const (
	LightAmbient     = 0
	LightDirectional = 1
	LightPoint       = 2
	LightSpot        = 3
	LightHemisphere  = 4
)

var engineInputs = []EngineInput{
	{"cameraPos", "vec3", EngineValue, "world, meters", "Camera position.", 0},
	{"sunDir", "vec3", EngineValue, "world, unit vector", "Direction toward the key directional light.", 0},
	{"sunColor", "vec3", EngineValue, "linear RGB", "Key directional light color times intensity.", 0},
	{"ambient", "vec3", EngineValue, "linear RGB", "Ambient light color times intensity.", 0},
	{"fog", "vec4", EngineValue, "linear RGB, 1/meters", "Fog color in xyz and density per meter in w; density 0 disables fog.", 0},
	{"exposure", "float", EngineValue, "linear scale", "Scene exposure multiplier applied before tone mapping.", 0},
	{"time", "float", EngineValue, "seconds", "Scene clock.", 0},
	{"cameraProximity", "float", EngineValue, "0..1", "How close the camera is to the object, as the renderer defines it.", 0},
	{"environmentInfo", "vec4", EngineValue, "x: image-based light active, y: intensity, z: rotation radians, w: top radiance mip", "Image-based lighting state.", 0},
	{"skyBetaR", "vec4", EngineValue, "physical sky block 0", "Rayleigh coefficients in xyz and sun intensity in w; the first vec4 of the physical sky parameters.", 0},
	{"skyBetaM", "vec4", EngineValue, "physical sky block 1", "Mie coefficients in xyz and sun fade in w.", 0},
	{"skySun", "vec4", EngineValue, "physical sky block 2", "Unit direction toward the sun in xyz and the cosine of the sun disk radius in w (above 1 hides the disk).", 0},
	{"skyG", "float", EngineValue, "physical sky block 3", "Mie directional factor g.", 0},
	{"planarReflection", "texture2d", EngineTexture, "linear RGB in rgb, coverage in a", "Planar reflection capture.", 0},
	{"opaqueColor", "texture2d", EngineTexture, "linear RGB", "Opaque scene color captured before transparent surfaces.", 0},
	{"opaqueDepth", "textureDepth2d", EngineTexture, "depth 0..1", "Depth of the opaque scene; read with sampleDepth.", 0},

	// Scene lighting (docs/engine-inputs.md#scene-lighting).
	{Name: "cameraForward", Type: "vec3", Kind: EngineValue, Space: "world, unit vector", Doc: "Direction the camera looks; view depth is dot(p - cameraPos, cameraForward)."},
	{Name: "lightCount", Type: "float", Kind: EngineValue, Space: "0..8", Doc: "Number of lights packed into lights."},
	{Name: "lights", Type: "vec4", Kind: EngineValue, Count: SceneLightCapacity * SceneLightVec4s, Space: "4 vec4 per light", Doc: "Scene lights: [4i] position xyz, kind w; [4i+1] direction the light travels xyz, range w (0 = unlimited); [4i+2] linear color times intensity xyz, decay w; [4i+3] cos outer cone x and cos inner cone y for a spot light, or ground color times intensity xyz for a hemisphere light, and shadow slot w (0 or 1, -1 none)."},
	{Name: "shadow0", Type: "vec4", Kind: EngineValue, Space: "x cascades, y bias, z filter radius (uv), w light index", Doc: "Shadow slot 0: cascade count (0 = no shadow), depth bias, PCF radius in shadow-map uv, and the index into lights of the casting light (-1 none)."},
	{Name: "shadow1", Type: "vec4", Kind: EngineValue, Space: "x cascades, y bias, z filter radius (uv), w light index", Doc: "Shadow slot 1, as shadow0."},
	{Name: "shadowSplits0", Type: "vec4", Kind: EngineValue, Space: "view depth, meters", Doc: "Far view depth of cascades 0..3 of slot 0."},
	{Name: "shadowSplits1", Type: "vec4", Kind: EngineValue, Space: "view depth, meters", Doc: "Far view depth of cascades 0..3 of slot 1."},
	{Name: "shadowMatrices0", Type: "vec4", Kind: EngineValue, Count: ShadowCascadeCapacity * 4, Space: "world to shadow uvz, column-major", Doc: "Slot 0 matrices, four columns per cascade (cascade c is [4c..4c+3]); they map a world position to shadow-map uv in xy and comparable depth 0..1 in z, after dividing by w."},
	{Name: "shadowMatrices1", Type: "vec4", Kind: EngineValue, Count: ShadowCascadeCapacity * 4, Space: "world to shadow uvz, column-major", Doc: "Slot 1 matrices, as shadowMatrices0."},
	{Name: "shadowMap0", Type: "textureDepth2dArray", Kind: EngineTexture, Space: "depth 0..1, one layer per cascade", Doc: "Slot 0 shadow depth; read with sampleDepth(shadowMap0, uv, cascade)."},
	{Name: "shadowMap1", Type: "textureDepth2dArray", Kind: EngineTexture, Space: "depth 0..1, one layer per cascade", Doc: "Slot 1 shadow depth; read with sampleDepth(shadowMap1, uv, cascade)."},

	// Wind (GoSX scene.Wind; the materiallib game module's mlWindSway reads it).
	{Name: "wind", Type: "vec4", Kind: EngineValue, Space: "xy unit XZ direction, z m/s, w seconds", Doc: "Scene wind: the direction it blows toward in xy (world x and z), speed in z and the wind clock in w; all zero when the scene has no wind."},
	{Name: "windGust", Type: "vec4", Kind: EngineValue, Space: "x 0..1, y meters, z m/s, w Hz", Doc: "Wind gusts: strength (speed swings by +-x), distance between gust fronts, speed of the fronts and flutter frequency."},
}

// Declaration returns the type a context block declares for in:
// "array<vec4, 32>" for an array input, otherwise Type.
func (in EngineInput) Declaration() string {
	if in.Count > 0 {
		return "array<" + in.Type + ", " + strconv.Itoa(in.Count) + ">"
	}
	return in.Type
}

// EngineInputs returns a copy of the registry in declaration order.
func EngineInputs() []EngineInput {
	return append([]EngineInput(nil), engineInputs...)
}

// LookupEngineInput returns the registry entry for name.
func LookupEngineInput(name string) (EngineInput, bool) {
	for _, in := range engineInputs {
		if in.Name == name {
			return in, true
		}
	}
	return EngineInput{}, false
}
