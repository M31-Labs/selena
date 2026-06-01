# Selena Compatibility Notes

Selena's compatibility contract is the generated shader source plus the binding
descriptor returned by `selena.Compile`. Hosts should treat the descriptor as
the source of truth for uniform packing, attributes, texture units, and backend
binding coordinates.

This document describes the current public surface of the compiler. It is not a
browser, OS, or GPU support matrix.

## Targets

| Selena target | Renderer family | Shader shape |
|---|---|---|
| `wgsl` | WebGPU in browser and Chromium desktop shells | Single WGSL module with `vertexMain` and `fragmentMain` |
| `glsl` | WebGL 1 style runtime | Split vertex and fragment GLSL using `attribute`, `varying`, `texture2D`, and `gl_FragColor` |
| `gles` | WebGL 2 / Android GLES 3 style runtime | Split `#version 300 es` vertex and fragment GLSL using `in`, `out`, `texture`, and `fragColor` |
| `metal` | Metal / SceneKit style runtime | Single MSL source with `vertexMain` and `fragmentMain` |

## Descriptor Contract

Every compile result carries a `bindings.Layout` descriptor.

- `schemaVersion` is currently `selena.descriptor.v1`.
- `languageVersion` is currently `selena.lang.v1`.
- `uniformBlock` uses std140-compatible offsets and sizes in declaration order.
- `uniformBlock.defaults` carries host-packable float component arrays for
  `.sel` defaults.
- `attributes` lists the vertex inputs the emitted shader expects.
- `textures` lists each sampled texture and its per-backend binding coordinates.

The host is responsible for providing the standard transform uniforms:

- `mvp : mat4`
- `normalMatrix : mat3`

Material parameters are appended after those standard uniforms in the same
uniform block. `bindings.PackUniforms` and
`bindings.PackUniformsWithDefaults` are the supported Go-side packing APIs.

## WGSL / WebGPU

The WGSL emitter produces one module with `vertexMain` and `fragmentMain`.

- The uniform block is `@group(0) @binding(0)`.
- Texture `i` is `@group(0) @binding(1 + 2*i)`.
- Sampler `i` is `@group(0) @binding(2 + 2*i)`.
- Vertex attributes use the locations listed in the descriptor.
- `texture2d` params lower to separate `texture_2d<f32>` and `sampler`
  bindings.

WGSL is the primary path for WebGPU-capable browser and desktop GoSX surfaces.

## GLSL / WebGL 1

The GLSL emitter produces split vertex and fragment sources compatible with the
WebGL 1 style shader model used by the current web runtime.

- Vertex inputs use `attribute`.
- Interpolants use `varying`.
- Material uniforms are emitted as bare GLSL uniforms.
- Sampled textures are `sampler2D` uniforms.
- Texture sampling uses `texture2D`.
- The fragment output uses `gl_FragColor`.

The descriptor's `textures[].gl.unit` value tells the host which texture unit to
bind for each sampler uniform.

## GLES / WebGL 2 / Android GLES 3

The GLES emitter produces split `#version 300 es` vertex and fragment sources.

- Vertex inputs use `in`.
- Interpolants use `out` from the vertex shader and `in` in the fragment shader.
- Material uniforms are emitted as bare GLSL ES uniforms.
- Sampled textures are `sampler2D` uniforms.
- Texture sampling uses `texture`.
- The fragment output is `fragColor`.

This target is intended for Android GLES 3 style renderers and WebGL 2 style
hosts that prefer GLSL ES 3 syntax.

## Metal / SceneKit

The Metal emitter produces one MSL source with `vertexMain` and `fragmentMain`.

- The uniform block is passed as `constant Uniforms& u [[buffer(0)]]`.
- Texture params become `texture2d<float>` arguments.
- Samplers are separate `sampler` arguments.
- Texture and sampler indices match `textures[].metal`.

SceneKit binding details are intentionally host-owned. Selena's contract is the
MSL source plus the descriptor coordinates needed to bind matching buffers,
textures, and samplers.

## Current Gaps

The current compiler is useful, but the compatibility story is still growing.

- Browser pixel conformance is not in CI yet.
- Native/mobile CI is not in this repository yet.
- Vertex hooks are described in HIR but not implemented in the parser/lowering
  path.
- Arrays, booleans, and integer uniforms are not part of the v1 descriptor.
- Standard material / PBR interop is not designed yet.
- Golden output conformance is still lightweight; the checked-in corpus is the
  starting point rather than the finish line.
