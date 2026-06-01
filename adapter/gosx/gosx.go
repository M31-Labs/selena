// Package gosx adapts a Selena material into a GoSX scene.IRMaterial, filling
// the custom-shader slots for every backend (GLSL for WebGL, WGSL for WebGPU).
// Because both language pairs are populated from one source, the GoSX WebGPU
// honesty gate serves the material on all backends instead of degrading it
// (see scene.customShaderUnservedBackends). It also returns the binding layout
// the host uses to wire uniforms without hand-packing.
//
// This is the only Selena package that depends on GoSX; the core
// (ir/lower/emit/bindings/hir/parse) stays free of it.
package gosx

import (
	gscene "m31labs.dev/gosx/scene"

	"m31labs.dev/selena/bindings"
	"m31labs.dev/selena/emit/glsl"
	"m31labs.dev/selena/emit/wgsl"
	"m31labs.dev/selena/hir"
	"m31labs.dev/selena/lower"
)

// Material lowers m (with optional reusable functions) and returns a GoSX
// IRMaterial with all custom-shader slots filled, plus the binding layout.
func Material(m hir.Material, funcs []hir.FuncDecl) (gscene.IRMaterial, bindings.Layout, error) {
	mod, layout, err := lower.LowerWith(m, funcs)
	if err != nil {
		return gscene.IRMaterial{}, bindings.Layout{}, err
	}
	wgslSrc, err := wgsl.Emit(mod)
	if err != nil {
		return gscene.IRMaterial{}, bindings.Layout{}, err
	}
	glslVert, glslFrag, err := glsl.Emit(mod)
	if err != nil {
		return gscene.IRMaterial{}, bindings.Layout{}, err
	}
	return gscene.IRMaterial{
		Name:               mod.Name,
		Kind:               "custom",
		CustomVertex:       glslVert,
		CustomFragment:     glslFrag,
		CustomVertexWGSL:   wgslSrc, // one WGSL module exposes vertexMain + fragmentMain
		CustomFragmentWGSL: wgslSrc,
		CustomUniforms:     DefaultUniforms(layout),
	}, layout, nil
}

// DefaultUniforms returns descriptor defaults in GoSX's CustomUniforms shape.
func DefaultUniforms(layout bindings.Layout) map[string]any {
	if len(layout.UniformBlock.Defaults) == 0 {
		return nil
	}
	out := make(map[string]any, len(layout.UniformBlock.Defaults))
	for _, d := range layout.UniformBlock.Defaults {
		if len(d.Values) == 1 {
			out[d.Name] = d.Values[0]
			continue
		}
		out[d.Name] = append([]float32(nil), d.Values...)
	}
	return out
}
