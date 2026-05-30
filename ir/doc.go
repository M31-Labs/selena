// Package ir defines the neutral, typed Selena shader IR — the heart of the
// project and the single source of truth for a material. Every target emitter
// (wgsl, glsl, metal, gles) consumes this IR, and GoSX's scene.IR custom-shader
// string slots become outputs of these emitters rather than authoring inputs.
//
// Design intent (see README): a small, total typed expression graph — no
// unbounded control flow — so emission to all four backends is deterministic.
package ir
