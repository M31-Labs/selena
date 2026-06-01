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

// CompileOptions controls material selection and shader emission.
//
// If Material is empty, Selena compiles the last material in the program. This
// matches the file-authoring convention where a derived material follows its
// base material. If Targets is nil, Compile emits every supported target. If
// Targets is an empty non-nil slice, Compile parses and lowers without emitting
// shader source.
type CompileOptions struct {
	Material string
	Targets  []Target
}

// Artifact is the emitted shader source for one target. WGSL and Metal use
// Source. GLSL and GLES use Vertex and Fragment.
type Artifact struct {
	Target   Target
	Source   string
	Vertex   string
	Fragment string
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
	artifacts, err := emitArtifacts(mod, opts.Targets)
	if err != nil {
		return Result{}, compileError(err)
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
