package lower

import "m31labs.dev/selena/hir"

func validateAuthorName(kind, name string, span hir.Span) error {
	if reason, ok := reservedBackendName(name); ok {
		return diagnostic(CodeReservedName, span, "%s %q is reserved for %s", kind, name, reason)
	}
	return nil
}

func reservedBackendName(name string) (string, bool) {
	if reason, ok := generatedNames[name]; ok {
		return reason, true
	}
	if reason, ok := shaderReservedNames[name]; ok {
		return reason, true
	}
	if _, ok := stdlib.builtin(name); ok {
		return "Selena stdlib builtin", true
	}
	switch name {
	case "rgb", "sample", "state", "stateAt":
		return "Selena stdlib builtin", true
	case "vertexIndex":
		return "Selena vertex-index builtin", true
	}
	return "", false
}

var generatedNames = map[string]string{
	"Uniforms":     "generated uniform struct",
	"VertexIn":     "generated Metal vertex input struct",
	"VertexInput":  "generated WGSL vertex input struct",
	"VertexOut":    "generated Metal vertex output struct",
	"VertexOutput": "generated WGSL vertex output struct",
	"fragColor":    "generated GLES fragment output",
	"fragmentMain": "generated shader entry point",
	"gl_FragColor": "generated GLSL fragment output",
	"gl_Position":  "generated vertex position output",
	"in":           "generated stage input binding",
	"main":         "generated GLSL entry point",
	"mvp":          "generated transform uniform",
	"normalMatrix": "generated transform uniform",
	"out":          "generated stage output binding",
	"position":     "generated vertex position attribute",
	"u":            "generated uniform binding",
	"vertexMain":   "generated shader entry point",
	// Feedback-kind injected names.
	"computeMain": "generated compute entry point",
	"inState":     "generated statefield input buffer",
	"outState":    "generated statefield output buffer",
	"stateTex":    "generated statefield sampler",
	"cellIndex":   "generated cell index",
	"gridWidth":   "generated grid uniform",
	"gridLen":     "generated grid uniform",
	"texelSize":   "generated grid uniform",
	"vUV":         "generated fragment uv varying",
	"_grid":       "generated grid uniform block",
	// B4 mesh/general statefield-read (stateAt) + authored-vertex injected names.
	"_inState":      "generated statefield input buffer",
	"_stateGrid":    "generated state-grid uniform block",
	"StateGrid":     "generated state-grid uniform struct",
	"gridHeight":    "generated grid uniform",
	"a_vertexIndex": "generated GLSL vertex-index attribute",
	"vertexIndex":   "generated vertex-index builtin",
}

var shaderReservedNames = map[string]string{
	"alias":       "WGSL keyword",
	"array":       "WGSL keyword",
	"attribute":   "GLSL keyword",
	"bool":        "shader type keyword",
	"case":        "shader control-flow keyword",
	"const":       "shader keyword",
	"constant":    "Metal address-space keyword",
	"continue":    "shader control-flow keyword",
	"default":     "shader control-flow keyword",
	"device":      "Metal address-space keyword",
	"discard":     "shader control-flow keyword",
	"else":        "shader control-flow keyword",
	"f32":         "WGSL scalar type",
	"false":       "shader literal",
	"float":       "shader scalar type",
	"float2":      "Metal vector type",
	"float3":      "Metal vector type",
	"float4":      "Metal vector type",
	"for":         "shader control-flow keyword",
	"fragment":    "Metal/WGSL stage keyword",
	"fn":          "WGSL function keyword",
	"highp":       "GLSL precision keyword",
	"i32":         "WGSL scalar type",
	"if":          "shader control-flow keyword",
	"inout":       "shader parameter keyword",
	"int":         "shader scalar type",
	"kernel":      "Metal stage keyword",
	"let":         "WGSL binding keyword",
	"loop":        "WGSL control-flow keyword",
	"lowp":        "GLSL precision keyword",
	"mat3":        "shader matrix type",
	"mat3x3":      "WGSL matrix type",
	"mat4":        "shader matrix type",
	"mat4x4":      "WGSL matrix type",
	"mediump":     "GLSL precision keyword",
	"metal":       "Metal namespace",
	"namespace":   "Metal keyword",
	"override":    "WGSL keyword",
	"precision":   "GLSL keyword",
	"return":      "shader control-flow keyword",
	"sampler":     "shader sampler type",
	"sampler2D":   "GLSL sampler type",
	"struct":      "shader declaration keyword",
	"switch":      "shader control-flow keyword",
	"texture":     "GLES builtin function",
	"texture2D":   "GLSL builtin function",
	"texture_2d":  "WGSL texture type",
	"thread":      "Metal address-space keyword",
	"threadgroup": "Metal address-space keyword",
	"true":        "shader literal",
	"type":        "WGSL keyword",
	"u32":         "WGSL scalar type",
	"uniform":     "GLSL keyword",
	"using":       "Metal keyword",
	"var":         "WGSL binding keyword",
	"varying":     "GLSL keyword",
	"vec2":        "shader vector type",
	"vec3":        "shader vector type",
	"vec4":        "shader vector type",
	"vertex":      "Metal/WGSL stage keyword",
	"void":        "shader return type",
	"while":       "shader control-flow keyword",
}
