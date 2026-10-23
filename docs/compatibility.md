# Selena Compatibility Notes

Selena's compatibility contract is the generated shader source plus the binding
descriptor returned by `selena.Compile`. Hosts should treat the descriptor as
the source of truth for uniform packing, attributes, texture units, and backend
binding coordinates.

This document describes the current public surface of the compiler. It is not a
browser, OS, or GPU support matrix.

`sampleCubeLevel(texture, direction, lod)` samples a `textureCube` parameter at
an explicit floating-point mip level. Use it for roughness-prefiltered environment
maps; it preserves the authored level on WGSL, GLES, GLSL and Metal. Like other
texture sampling calls, it is currently supported in fragment surfaces only.
WebGL 1 requires `EXT_shader_texture_lod`, declared both in the emitted shader
and the host descriptor. Hosts provide the cubemap mip chain; this operation
does not request scene-color/backdrop mipmaps. Library functions may accept
texture handles without copying them into numeric local bindings.

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
- `requires` lists what the host must arrange beyond uniforms, attributes and
  textures. It is absent for a material that needs nothing extra. See
  [Host Requirements](#host-requirements).

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

GLSL ES 1.00 has a smaller core library than the other three targets. When the
material needs something outside it, this emitter writes the matching
`#extension` directive ahead of the `precision` line AND the descriptor lists
the extension in `requires.glExtensions`, because WebGL ignores the directive
unless the host has already called `gl.getExtension` for it. See
[Host Requirements](#host-requirements).

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

## Host Requirements

Some shader features need the host to do something the shader source cannot
express. A host that skips one of these does not get a loud failure — the pass
either fails to compile in the browser only, or renders with the effect missing.
`layout.requires` names each of them so the host can act, or refuse the shader,
before a page is loaded.

```json
"requires": {
  "glExtensions": ["OES_standard_derivatives", "EXT_shader_texture_lod"],
  "sceneColorMips": true,
  "glSceneSizeUniform": "_sceneSize",
  "glVertexTextureUnits": 1
}
```

### requires.glExtensions

The WebGL extensions the host must request with `gl.getExtension(name)` BEFORE
compiling the `glsl` (WebGL 1 / GLSL ES 1.00) artifact.

A shader `#extension` directive is not enough on its own. WebGL only honours the
directive when the matching extension object has already been requested;
otherwise the driver reports `'GL_OES_standard_derivatives' : extension is not
supported` and then `'fwidth' : no matching overloaded function found`, and the
whole pass is dropped.

| Extension | Needed by |
|---|---|
| `OES_standard_derivatives` | `fwidth`, `dpdx`, `dpdy` |
| `EXT_shader_texture_lod` | `sceneColorLevel` (fragment-stage explicit LOD) |
| `WEBGL_depth_texture` | a `textureDepth2d` engine texture such as `opaqueDepth` |

The `gles` (GLSL ES 3.00), `wgsl` and `metal` artifacts have all of this in core
and need nothing. A host running a WebGL 2 context should prefer the `gles`
artifact: `OES_standard_derivatives` does not exist in WebGL 2, so a GLSL ES
1.00 shader that needs derivatives cannot be made to work there.

### requires.sceneColorMips

True when the material calls `sceneColorLevel(uv, lod)`. The host must:

1. Render the post source into a texture that HAS a mip chain.
2. Regenerate the mips each frame before the pass.
3. Bind a sampler whose minification filter walks mips
   (`LINEAR_MIPMAP_LINEAR`, or `GPUFilterMode` `linear` with `mipmapFilter`
   `linear`).

Without a mip chain every backend clamps to level 0 and the pass renders
unblurred rather than failing.

### requires.glSceneSizeUniform

Non-empty when the material calls `sceneSize()`. GLSL ES 1.00 has no
`textureSize`, so the `glsl` artifact declares a `uniform vec2 _sceneSize` the
host must set to the scene-color target size in pixels, and reset on resize.
`wgsl`, `gles` and `metal` read the size off the bound texture.

### requires.glVertexTextureUnits

The number of vertex-stage texture units the `glsl` (WebGL 1 / GLSL ES 1.00)
artifact needs. It is non-zero when an authored `vertex()` calls
`sampleLevel(tex, uv, lod)`; it equals the number of textures whose descriptor
entry has `vertexStage: true`.

GLSL ES 1.00 lets `MAX_VERTEX_TEXTURE_IMAGE_UNITS` be 0, and some WebGL 1
devices report 0. Selena cannot see the device, so it emits the shader and
declares the need. The host should compare the value with
`gl.getParameter(gl.MAX_VERTEX_TEXTURE_IMAGE_UNITS)` and, when the device has
fewer units, use the `gles` artifact on a WebGL 2 context (at least 16 vertex
units) or pick another material. Otherwise the program fails to link in the
browser only. `wgsl`, `gles` and `metal` need nothing for this.

### textures[].engine and fields[].source

`textures[].engine` names the engine input bound to a texture declared in
`context`, and `uniformBlock.fields[].source: "engine"` marks a registered
engine value. `engineVersion` on the descriptor names the registry version the
material was compiled against. `textures[].dimension` is `"depth2d"` for a
depth texture. See [engine-inputs.md](engine-inputs.md).

### textures[].vertexStage

True for a texture the authored `vertex()` samples. Binding coordinates are the
same in both stages. The host must also make the binding visible to the vertex
stage:

- WebGPU: bind-group-layout visibility `GPUShaderStage.VERTEX |
  GPUShaderStage.FRAGMENT` for the texture and its sampler.
- Metal: `setVertexTexture` and `setVertexSamplerState` at `textures[].metal`.
- WebGL: bind the texture to `textures[].gl.unit` as usual.

Only `sampleLevel` (2D, explicit LOD) is available in `vertex()`. `sample` and
`sampleCube` need implicit derivatives, which a vertex shader lacks.
`sampleCubeLevel` is not wired into the vertex stage yet. A vertex-stage
sampler2D is declared `highp` on the GLSL targets, because the vertex-stage
default is `lowp`.

## Current Gaps

The current compiler is useful, but the compatibility story is still growing.

- Browser pixel conformance is not in CI yet.
- Native/mobile CI is not in this repository yet.
- Vertex hooks are described in HIR but not implemented in the parser/lowering
  path.
- Booleans and integer uniforms are not part of the v1 descriptor. Fixed-size
  arrays are: a `param x : array<T, N>` field carries `count` and `stride`.
- Standard material / PBR interop is not designed yet.
- Golden output conformance is still lightweight; the checked-in corpus is the
  starting point rather than the finish line.

## Optional linear specular output (MRT)

Mesh materials may author a companion entry:

```selena
material Polished {
    param highlight : color
    surface(geo) -> vec4 {
        return vec4f(0.4, 0.3, 0.2, 0.8)
    }
    specular(geo) -> color {
        return highlight
    }
}
```

`specular` returns linear RGB lighting, not total surface color or a luminance
threshold. Its geometry binding and locals are independent of `surface`; params,
context, textures and varyings are shared. User functions work in both entries.
An omitted companion inherits from the parent material; declaring one overrides
it. A companion must return `color`/`vec3` in every return and cannot discard.
Straight-line immutable calculations shared by both entries are fused once in
the MRT shader; stages with control flow or mutation keep separate evaluation
so optimization cannot move work across coverage or assignment boundaries.
Only `surface` controls coverage: its discard terminates both outputs and its
alpha is copied to the specular attachment, including after early returns.

Request variants with `selena.CompileOptions{SpecularMRT: true}`. Existing
`Artifact.Source`, `Vertex`, `Fragment` and normal entry names remain unchanged.
Each mesh artifact gains optional `Artifact.SpecularMRT`: WGSL/Metal `Source`
contains a full module with entry `fragmentMainSpecular`; GLSL/GLES `Fragment`
reuses the normal vertex source and enters through `main`. Non-mesh materials
without a companion ignore the option; authoring a companion on them is an error.

The additive v1 descriptor fields are `entryPoints.fragmentSpecular` and
`specularOutput`, with semantic `specular`, location `1`, type `vec4`, color space
`linear`, and an `authored` boolean. Color remains at location 0. When no companion
is authored, the variant writes exactly zero RGB with the surface alpha; hosts
must not infer specular from luminance. The variant requires two compatible
render attachments and matching blend/depth/coverage configuration. GLSL ES 1.00
also requires `specularOutput.glExtensions: ["WEBGL_draw_buffers"]`; request that
extension before compiling. GLES3, WGSL and Metal express MRT in their core APIs.
A host that reserves attachment 1 (for example, a native picking target) must
explicitly reconcile that layout before using these artifacts. This compiler
contract alone does not add custom-shader transport to a native renderer.

Validation includes Naga for WGSL, glslangValidator for GLES3, and an opt-in real
Chromium WebGL1 compile/link/readback test (`SELENA_CHROME=/path/to/chrome go test
./validate -run Specular`). glslang does not implement `GL_EXT_draw_buffers`;
its GLSL1 check covers the shared vertex source, while Chromium checks the MRT
fragment and both attachments. MSL source contracts are tested here; actual
Metal compilation still requires Apple's toolchain.
