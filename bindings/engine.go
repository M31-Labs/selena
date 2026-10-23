package bindings

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
}

var engineInputs = []EngineInput{
	{"cameraPos", "vec3", EngineValue, "world, meters", "Camera position."},
	{"sunDir", "vec3", EngineValue, "world, unit vector", "Direction toward the key directional light."},
	{"sunColor", "vec3", EngineValue, "linear RGB", "Key directional light color times intensity."},
	{"ambient", "vec3", EngineValue, "linear RGB", "Ambient light color times intensity."},
	{"fog", "vec4", EngineValue, "linear RGB, 1/meters", "Fog color in xyz and density per meter in w; density 0 disables fog."},
	{"exposure", "float", EngineValue, "linear scale", "Scene exposure multiplier applied before tone mapping."},
	{"time", "float", EngineValue, "seconds", "Scene clock."},
	{"cameraProximity", "float", EngineValue, "0..1", "How close the camera is to the object, as the renderer defines it."},
	{"environmentInfo", "vec4", EngineValue, "x: image-based light active, y: intensity, z: rotation radians, w: top radiance mip", "Image-based lighting state."},
	{"skyBetaR", "vec4", EngineValue, "physical sky block 0", "Rayleigh coefficients in xyz and sun intensity in w; the first vec4 of the physical sky parameters."},
	{"skyBetaM", "vec4", EngineValue, "physical sky block 1", "Mie coefficients in xyz and sun fade in w."},
	{"skySun", "vec4", EngineValue, "physical sky block 2", "Unit direction toward the sun in xyz and the cosine of the sun disk radius in w (above 1 hides the disk)."},
	{"skyG", "float", EngineValue, "physical sky block 3", "Mie directional factor g."},
	{"planarReflection", "texture2d", EngineTexture, "linear RGB in rgb, coverage in a", "Planar reflection capture."},
	{"opaqueColor", "texture2d", EngineTexture, "linear RGB", "Opaque scene color captured before transparent surfaces."},
	{"opaqueDepth", "textureDepth2d", EngineTexture, "depth 0..1", "Depth of the opaque scene; read with sampleDepth."},
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
