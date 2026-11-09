package result

import (
	"m31labs.dev/selena/bindings"
	"m31labs.dev/selena/hir"
	"m31labs.dev/selena/ir"
)

// Target is one shader backend Selena can emit.
type Target string

const (
	TargetWGSL  Target = "wgsl"
	TargetGLSL  Target = "glsl"
	TargetMetal Target = "metal"
	TargetGLES  Target = "gles"
)

var defaultTargets = []Target{TargetWGSL, TargetGLSL, TargetMetal, TargetGLES}

// AllTargets returns Selena's default backend set in deterministic order.
func AllTargets() []Target {
	out := make([]Target, len(defaultTargets))
	copy(out, defaultTargets)
	return out
}

// CompileOptions controls material selection and shader emission for the
// compile functions in package selena (Compile, CompileMaterial and
// CompileProgram).
//
// If Material is empty, Selena compiles the last material in the program. This
// matches the file-authoring convention where a derived material follows its
// base material. If Targets is nil, the compile functions emit every supported
// target (see AllTargets). If Targets is an empty non-nil slice, they parse and
// lower without emitting shader source.
type CompileOptions struct {
	// SpecularMRT emits an additional color0/specular1 mesh fragment variant.
	// It does not replace normal artifacts. Without authored specular(), that
	// variant writes zero RGB with the normal surface's coverage alpha.
	// Non-mesh materials without a companion ignore this option.
	SpecularMRT bool
	Material    string
	Targets     []Target
	// FirstInstanceLocation is the shader location of the first `instance`
	// attribute. Zero means directly after the vertex attributes. A host that
	// binds its own per-instance data at fixed locations sets it above them;
	// WebGPU needs the shader location to equal the buffer layout's
	// shaderLocation, so the host cannot remap after compile. It must be at
	// least the number of vertex attributes, and the last instance location
	// must stay below 16. Materials without instance attributes ignore it.
	FirstInstanceLocation int
}

// FragmentArtifact is an additive fragment variant. WGSL/Metal carry the full
// module in Source; GLSL/GLES carry Fragment and reuse Artifact.Vertex.
type FragmentArtifact struct {
	Source   string
	Fragment string
}

// Artifact is the emitted shader source for one target. WGSL and Metal use
// Source. GLSL and GLES use Vertex and Fragment.
type Artifact struct {
	SpecularMRT *FragmentArtifact `json:",omitempty"`
	Target      Target
	Source      string
	Vertex      string
	Fragment    string
}

// Result contains the useful artifacts from a Selena compile: parsed HIR,
// selected material, lowered IR, host binding layout, and emitted shader source.
type Result struct {
	Program   hir.Program
	Material  hir.Material
	Module    ir.Module
	Layout    bindings.Layout
	Artifacts []Artifact
}

// Artifact returns the emitted artifact for target.
func (r Result) Artifact(target Target) (Artifact, bool) {
	for _, a := range r.Artifacts {
		if a.Target == target {
			return a, true
		}
	}
	return Artifact{}, false
}
