// Package internal holds helpers shared by the four backend shader emitters
// (emit/wgsl, emit/glsl, emit/metal, emit/gles). It is importable only by
// packages under emit/.
//
// The emitters differ in stage scaffolding and surface spelling, but the
// IR->gputype mapping, the binding-name set, and the ir.Dialect Ref/TypeName/
// Call/Sample plumbing are identical (or cleanly parameterizable) across them.
// This package is where that shared machinery lives so each emitter does not
// carry its own copy.
package internal

import (
	"m31labs.dev/prism/dialect"
	"m31labs.dev/prism/gputype"
	"m31labs.dev/selena/ir"
)

// TypeToGPU maps a Selena ir.Type to a prism gputype.Type.
func TypeToGPU(t ir.Type) gputype.Type {
	switch t {
	case ir.Float:
		return gputype.F32
	case ir.Vec2:
		return gputype.Vec{N: 2, Elem: gputype.F32}
	case ir.Vec3:
		return gputype.Vec{N: 3, Elem: gputype.F32}
	case ir.Vec4:
		return gputype.Vec{N: 4, Elem: gputype.F32}
	case ir.Mat3:
		return gputype.Mat{Cols: 3, Rows: 3, Elem: gputype.F32}
	case ir.Mat4:
		return gputype.Mat{Cols: 4, Rows: 4, Elem: gputype.F32}
	default:
		return gputype.F32
	}
}

// NameSet collects the names of bs into a lookup set.
func NameSet(bs []ir.Binding) map[string]bool {
	m := make(map[string]bool, len(bs))
	for _, b := range bs {
		m[b.Name] = true
	}
	return m
}

// Resolver implements ir.Dialect for one emitter stage by delegating the
// surface spelling (TypeName/Call/Sample) to a prism dialect.Dialect and
// resolving references per the backend's reference model:
//
//   - Qualified backends (WGSL, Metal) address a uniform as u.x and a stage
//     input as in.x, falling back to a bare name for stage-locals.
//   - Bare backends (GLSL, GLES) address everything through bare globals, so
//     Ref returns the name unchanged.
//
// The name sets and Fragment flag drive the qualified case and are ignored when
// Qualified is false.
type Resolver struct {
	Dialect       dialect.Dialect
	Uniforms      map[string]bool
	Attributes    map[string]bool
	Varyings      map[string]bool
	Fragment      bool
	Qualified     bool
	// SceneSampleFn renders a SceneSample expression. When nil, SceneSample
	// falls back to a dummy expression. Each post-emitter sets this.
	SceneSampleFn func(name, uv string) string
}

// NewQualified builds a struct-qualified resolver (WGSL/Metal) for one stage.
func NewQualified(d dialect.Dialect, m ir.Module, fragment bool) Resolver {
	return Resolver{
		Dialect:    d,
		Uniforms:   NameSet(m.Uniforms),
		Attributes: NameSet(m.Attributes),
		Varyings:   NameSet(m.Varyings),
		Fragment:   fragment,
		Qualified:  true,
	}
}

// NewBare builds a bare-global resolver (GLSL/GLES).
func NewBare(d dialect.Dialect) Resolver {
	return Resolver{Dialect: d}
}

// TypeName spells an ir.Type for the backend.
func (r Resolver) TypeName(t ir.Type) string { return r.Dialect.TypeName(TypeToGPU(t)) }

// Call renders a builtin/function call.
func (r Resolver) Call(name string, args []string) string { return r.Dialect.Builtin(name, args) }

// Sample renders a texture sample.
func (r Resolver) Sample(tex, uv string) string { return r.Dialect.Sample(tex, uv) }

// SceneSample renders a post-pass engine scene texture sample.
func (r Resolver) SceneSample(name, uv string) string {
	if r.SceneSampleFn != nil {
		return r.SceneSampleFn(name, uv)
	}
	return "vec4<f32>(0.0)" // unreachable in valid programs; guard only
}

// Ref resolves a reference in the current stage scope.
func (r Resolver) Ref(name string) string {
	if !r.Qualified {
		return name
	}
	switch {
	case r.Uniforms[name]:
		return "u." + name
	case !r.Fragment && r.Attributes[name]:
		return "in." + name
	case r.Fragment && r.Varyings[name]:
		return "in." + name
	default:
		return name // stage-local
	}
}
