package selena

import (
	"fmt"

	"m31labs.dev/selena/bindings"
	"m31labs.dev/selena/emit/gles"
	"m31labs.dev/selena/emit/glsl"
	"m31labs.dev/selena/emit/metal"
	"m31labs.dev/selena/emit/wgsl"
	"m31labs.dev/selena/hir"
	"m31labs.dev/selena/ir"
	"m31labs.dev/selena/lower"
	"m31labs.dev/selena/parse"
	"m31labs.dev/selena/result"
)

// The compile-result types are defined in package result, which does not
// import the compiler, so hosts that only read compile output can avoid
// linking it. The names below are aliases, constants and a forwarding function:
// existing code that uses them compiles unchanged, and values pass between
// the two packages without conversion.

// Target is one shader backend Selena can emit. It is an alias of
// result.Target.
type Target = result.Target

const (
	TargetWGSL  = result.TargetWGSL
	TargetGLSL  = result.TargetGLSL
	TargetMetal = result.TargetMetal
	TargetGLES  = result.TargetGLES
)

// AllTargets returns Selena's default backend set in deterministic order. It
// forwards to result.AllTargets.
func AllTargets() []Target { return result.AllTargets() }

// CompileOptions controls material selection and shader emission: Material
// names the material to compile (the last one when empty), Targets lists the
// backends to emit (all of them when nil, none when empty), and SpecularMRT adds
// the optional color/specular fragment variant. It is an alias of
// result.CompileOptions, which documents each field.
type CompileOptions = result.CompileOptions

// FragmentArtifact is an additive fragment variant. WGSL/Metal carry the full
// module in Source; GLSL/GLES carry Fragment and reuse Artifact.Vertex. It is an
// alias of result.FragmentArtifact.
type FragmentArtifact = result.FragmentArtifact

// Artifact is the emitted shader source for one target. WGSL and Metal use
// Source. GLSL and GLES use Vertex and Fragment. It is an alias of
// result.Artifact.
type Artifact = result.Artifact

// Result contains the useful artifacts from a Selena compile: parsed HIR,
// selected material, lowered IR, host binding layout, and emitted shader source.
// It is an alias of result.Result, which also defines the Artifact method that
// finds the artifact for one target.
type Result = result.Result

// Compile parses source, selects a material, lowers it, and emits requested
// shader targets.
func Compile(source []byte, opts CompileOptions) (Result, error) {
	p, err := parse.Program(source)
	if err != nil {
		return Result{}, compileError(err)
	}
	return CompileProgram(p, opts)
}

// CompileMaterial compiles a named material from an already parsed program.
func CompileMaterial(p hir.Program, name string, opts CompileOptions) (Result, error) {
	if name == "" {
		return Result{}, fmt.Errorf("material name is required")
	}
	opts.Material = name
	return CompileProgram(p, opts)
}

// CompileProgram selects, lowers, and emits one material from p.
func CompileProgram(p hir.Program, opts CompileOptions) (Result, error) {
	idx, err := selectMaterial(p, opts.Material)
	if err != nil {
		return Result{}, compileError(err)
	}
	mod, layout, err := lower.LowerProgram(p, idx)
	if err != nil {
		return Result{}, compileError(fmt.Errorf("lower %s: %w", p.Materials[idx].Name, err))
	}
	meshMRT := opts.SpecularMRT && (mod.Kind == "" || mod.Kind == ir.KindMesh)
	if meshMRT {
		layout.EntryPoints.FragmentSpecular = "fragmentMainSpecular"
		layout.SpecularOutput = &bindings.SpecularOutput{Semantic: "specular", Location: 1, Type: "vec4", ColorSpace: "linear", Authored: mod.Specular != nil, GLExtensions: []string{"WEBGL_draw_buffers"}}
	}
	artifacts, err := emitArtifacts(mod, opts.Targets)
	if err != nil {
		return Result{}, compileError(err)
	}
	if meshMRT {
		variant := mod
		variant.SpecularMRT = true
		for i := range artifacts {
			a, err := emitArtifact(variant, artifacts[i].Target)
			if err != nil {
				return Result{}, compileError(err)
			}
			artifacts[i].SpecularMRT = &FragmentArtifact{Source: a.Source, Fragment: a.Fragment}
		}
	}
	return Result{
		Program:   p,
		Material:  p.Materials[idx],
		Module:    mod,
		Layout:    layout,
		Artifacts: artifacts,
	}, nil
}

func selectMaterial(p hir.Program, name string) (int, error) {
	if len(p.Materials) == 0 {
		return 0, fmt.Errorf("no material declared")
	}
	if name == "" {
		return len(p.Materials) - 1, nil
	}
	found := -1
	for i, m := range p.Materials {
		if m.Name != name {
			continue
		}
		if found >= 0 {
			return 0, fmt.Errorf("material %q is declared more than once", name)
		}
		found = i
	}
	if found < 0 {
		return 0, fmt.Errorf("material %q not found", name)
	}
	return found, nil
}

func compileTargets(targets []Target) []Target {
	if targets != nil {
		return targets
	}
	return AllTargets()
}

func emitArtifacts(mod ir.Module, targets []Target) ([]Artifact, error) {
	targets = compileTargets(targets)
	artifacts := make([]Artifact, 0, len(targets))
	seen := map[Target]bool{}
	for _, target := range targets {
		if seen[target] {
			return nil, fmt.Errorf("target %q requested more than once", target)
		}
		seen[target] = true
		artifact, err := emitArtifact(mod, target)
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, artifact)
	}
	return artifacts, nil
}

func emitArtifact(mod ir.Module, target Target) (Artifact, error) {
	switch target {
	case TargetWGSL:
		src, err := wgsl.Emit(mod)
		if err != nil {
			return Artifact{}, fmt.Errorf("emit wgsl: %w", err)
		}
		return Artifact{Target: target, Source: src}, nil
	case TargetMetal:
		src, err := metal.Emit(mod)
		if err != nil {
			return Artifact{}, fmt.Errorf("emit metal: %w", err)
		}
		return Artifact{Target: target, Source: src}, nil
	case TargetGLSL:
		vertex, fragment, err := glsl.Emit(mod)
		if err != nil {
			return Artifact{}, fmt.Errorf("emit glsl: %w", err)
		}
		return Artifact{Target: target, Vertex: vertex, Fragment: fragment}, nil
	case TargetGLES:
		vertex, fragment, err := gles.Emit(mod)
		if err != nil {
			return Artifact{}, fmt.Errorf("emit gles: %w", err)
		}
		return Artifact{Target: target, Vertex: vertex, Fragment: fragment}, nil
	default:
		return Artifact{}, fmt.Errorf("unknown target %q", target)
	}
}
