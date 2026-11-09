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
  A texture declared with a `format`, or read with `load`, also carries
  `format` and `sampleMode` (see [Data Textures](#data-textures)).
- `instanceAttributes` and `instanceStride` are present only for mesh materials
  that declare `instance` attributes. They give the location, type and byte
  offset of each per-instance attribute, and the size of one packed record. See
  [Instancing](#instancing).
- `pipeline` is present only for `kind: "screen"` materials. It carries the
  blend and depth state the material declared. See [Screen
  Materials](#screen-materials).
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

### Validating Metal output

On a Mac with Xcode's Metal toolchain, Apple's offline compiler checks the
emitted MSL. The check is optional: hosts without the toolchain (Linux, or a Mac
with only the Command Line Tools) skip it.

```sh
selena doctor                                   # prints "metal: <path>" or "metal: unavailable"
selena shaders --target metal --validate --out out examples/textured.sel
go test ./validate/ -run TestMetalCompilesWithXcrun -v
```

The test compiles every material in `testdata/conformance` and `examples` with
`xcrun --sdk macosx metal -c`; on 2026-11-09 all 64 compiled with Xcode 15.4
(Apple metal 32023.155). If `xcode-select` points at the Command Line Tools,
the check retries with `DEVELOPER_DIR=/Applications/Xcode.app/Contents/Developer`;
set `DEVELOPER_DIR` to choose another Xcode. Package `mslcheck` exposes the same
check to Go code.

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
| `WEBGL_depth_texture` | a `textureDepth2d` or `textureDepth2dArray` engine texture such as `opaqueDepth` or `shadowMap0` |
| `ANGLE_instanced_arrays` | any `instance` attribute of a material that has a GLSL ES 1.00 form (float attributes only, no bit operators, no `instanceIndex`): call `vertexAttribDivisorANGLE(location, 1)` and draw with `drawArraysInstancedANGLE` |

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

True for a texture the authored `vertex()` samples (`sampleLevel`) or reads
with `load` or `size`. Binding coordinates are the
same in both stages. The host must also make the binding visible to the vertex
stage:

- WebGPU: bind-group-layout visibility `GPUShaderStage.VERTEX |
  GPUShaderStage.FRAGMENT` for the texture and its sampler.
- Metal: `setVertexTexture` and `setVertexSamplerState` at `textures[].metal`.
- WebGL: bind the texture to `textures[].gl.unit` as usual.

Only `sampleLevel` (2D, explicit LOD), `load` and `size` are available in
`vertex()`. `sample` and
`sampleCube` need implicit derivatives, which a vertex shader lacks.
`sampleCubeLevel` is not wired into the vertex stage yet. A vertex-stage
sampler2D is declared `highp` on the GLSL targets, because the vertex-stage
default is `lowp`.

## Data Textures

`load(tex, ix, iy)` and `size(tex)` read a `texture2d` param at integer texel
positions (see the Data Textures section of the
[language guide](language-guide.md)). Per target:

| Target | `load(t, ix, iy)` | `size(t)` | Host note |
|---|---|---|---|
| `wgsl` | `textureLoad(t, vec2<i32>(ix, iy), 0)` | `vec2<f32>(textureDimensions(t))` | The texture is `texture_2d<f32>` for every format. The `float32-filterable` feature is not needed to load. |
| `gles` | `texelFetch(t, ivec2(ix, iy), 0)` | `vec2(textureSize(t, 0))` | Needs nothing beyond the texture. |
| `glsl` | `texture2D(t, (vec2(ix, iy) + 0.5) / _texSize_t)` | `_texSize_t` | WebGL 1 has no `texelFetch`. See `requires.glTexSizeUniforms`. |
| `metal` | `t.read(uint2(ix, iy), 0)` | `float2(t.get_width(), t.get_height())` | Needs nothing beyond the texture. |

Details a host should know:

- **Samplers stay.** The WGSL and Metal artifacts still declare the texture's
  sampler, so binding numbers do not depend on how a texture is read. A texture
  used only through `load` can bind any sampler. For `textures[].sampleMode:
  "load"`, bind a non-filtering one.
- **Float formats.** `textures[].format` is `rgba8unorm`, `rgba16float`,
  `rgba32float`, `r16float` or `r32float`; the field is absent when the author
  wrote no `format`. Create the texture in that format. For `rgba32float` and
  `r32float` the WebGPU bind-group entry must be `unfilterable-float` with a
  `non-filtering` sampler, and the compiler rejects `sample` and `sampleLevel`
  on them. Uploading `r16float` to WebGL 2 needs no extension; rendering to it
  needs `EXT_color_buffer_half_float`.
- **Out-of-range coordinates** give a defined value on WGSL and Metal, an
  undefined one on GLES, and the edge texel on GLSL ES 1.00. The targets do not
  agree, so clamp in the shader. Selena's tests do not exercise this.
- **Vertex stage.** `load` and `size` in an authored `vertex()` mark the
  texture `vertexStage: true` and count toward `requires.glVertexTextureUnits`,
  like `sampleLevel`.

### requires.glTexSizeUniforms

A list of `_texSize_<texture>` names, present when the `glsl` (WebGL 1 / GLSL ES
1.00) artifact reads a texture with `load` or `size`. GLSL ES 1.00 has neither
`texelFetch` nor `textureSize`, so the artifact declares `uniform highp vec2
_texSize_<texture>` and `load` becomes a `texture2D` read at the texel centre
divided by that size. The host must:

1. Set each listed uniform to the texture's size in texels, and set it again
   after a resize.
2. Bind the texture with `NEAREST` min and mag filters and `CLAMP_TO_EDGE`
   wrapping, with no mipmaps, so the read returns the stored texel.

`wgsl`, `gles` and `metal` read the size off the bound texture and need nothing.
The uniform is `highp` in the vertex and fragment stage alike, because a uniform
shared by both stages must have the same precision to link.

### textures[].format and textures[].sampleMode

Both are optional and absent from descriptors of materials that do not use data
textures; an older host that ignores them keeps working. `format` is the
declared storage format. `sampleMode` is `"load"` (only `load` reads the
texture) or `"both"` (`load` and a sampling call); absent means `"sample"`. A
texture that is only passed to `size()` has no `sampleMode`. Both fields appear
in the artifact files without a schema change.

## Screen Materials

A `kind screen` material (see the Screen Materials section of the [language
guide](language-guide.md)) is a fullscreen pass that returns a colour with
alpha and reads no scene colour. The descriptor has `kind: "screen"` and a
`pipeline` object:

```json
"pipeline": { "blend": "additive", "depthTest": false, "depthWrite": false }
```

`blend` is `opaque`, `alpha`, `premultiplied` or `additive`. `depthTest` and
`depthWrite` come from the `depth` word: `off` is false and false, `test` is
true and false, `write` is true and true. A material with no `render` block gets
`alpha`, false, false. The field is absent on every other kind, and older
descriptors and artifact files decode as before. Selena only declares this
state. The host builds its pipeline from it:

| `blend` | Colour result (`s` shader output, `d` target) | WebGPU colour factors (src, dst) | Alpha factors (src, dst) |
|---|---|---|---|
| `opaque` | `s.rgb` | blending off | blending off |
| `alpha` | `s.rgb*s.a + d*(1-s.a)` | `src-alpha`, `one-minus-src-alpha` | `one`, `one-minus-src-alpha` |
| `premultiplied` | `s.rgb + d*(1-s.a)` | `one`, `one-minus-src-alpha` | `one`, `one-minus-src-alpha` |
| `additive` | `s.rgb*s.a + d` | `src-alpha`, `one` | `one`, `one` |

The same factors apply with `gl.blendFuncSeparate` and Metal's
`MTLBlendFactor`. A host that draws into an opaque swapchain never reads the
alpha channel.

**Placement and draw call.** The vertex stage is a fullscreen triangle with no
vertex buffer, so the host places the pass with the **viewport and scissor**
set to the target rectangle. `screen.uv` is `0..1` across that rectangle with
the origin at the top left, and `screenSize()` is the rectangle's size in
pixels, on every target. Per target:

| Target | Vertex stage | Draw call | Bindings |
|---|---|---|---|
| `wgsl` | `@builtin(vertex_index)` | `draw(3)` | Uniform block `@group(0) @binding(0)`, present only when the material has a uniform; texture `i` at binding `1+2i` and its sampler at `2+2i`. Same arithmetic as a mesh material, with no `mvp` or `normalMatrix`. |
| `gles` | `gl_VertexID` | `drawArrays(TRIANGLES, 0, 3)` | Uniforms by name, textures on units from `textures[].gl.unit`. |
| `glsl` | `attribute vec2 a_position` | `drawArrays` over a clip-space triangle `(-1,-1) (3,-1) (-1,3)` or a quad over `-1..1` | Uniforms by name, textures on their units, `_texSize_<name>` per `requires.glTexSizeUniforms`. |
| `metal` | `[[vertex_id]]` | `drawPrimitives(.triangle, 0, 3)` | Uniforms at `[[buffer(0)]]` when present, texture `i` at `[[texture(i)]]` and `[[sampler(i)]]`. |

Notes for the host:

- **`screenSize`.** When the body calls `screenSize()`, the uniform block has a
  `vec2` field `screenSize` tagged `class: "context"` and listed in
  `layout.context`. Write the target rectangle's size in pixels each draw. It is
  absent when the body does not call `screenSize()`.
- **WebGL 1 precision.** The `glsl` fragment declares `highp` for `float` and
  `int` where the device has fragment `highp` (`GL_FRAGMENT_PRECISION_HIGH`),
  and falls back to `mediump` where it does not, so the shader still compiles.
  On a device without fragment `highp`, texel addressing in data textures
  loses precision above about 1024 texels, and decibel levels lose resolution.
  Check `getShaderPrecisionFormat` if the material needs `highp`.
- **`uv` on WebGL 1.** The `glsl` vertex stage maps clip `y = +1` to `uv.y = 0`,
  so the origin matches the other targets whatever triangle or quad you draw.
  (`post` on this target maps the other way.)
- **Depth.** `depthTest` and `depthWrite` assume the host has a depth buffer
  attached to the target. The shader writes no depth. The triangle is drawn at
  the nearest depth on every target: window depth 0 with the default depth
  range (the GLSL vertex shaders emit clip `z = -1` to match WebGPU and Metal,
  where clip `z = 0` is depth 0). The host chooses the depth compare function.
- **Uniform array limits.** A `param x : array<vec4, N>` costs `N` uniform
  vectors. GLSL ES 3.00 guarantees `MAX_FRAGMENT_UNIFORM_VECTORS` of 224 and
  GLSL ES 1.00 guarantees 16, so the meter-strip example (two arrays of 64,
  128 vectors) fits the GLSL ES 3.00 minimum but not the GLSL ES 1.00 one.
  GLSL ES 1.00 Appendix A guarantees only constant and loop indices for uniform
  arrays in fragment shaders, so a computed index (`levels[i]` with `i` from
  arithmetic) may be rejected by a WebGL 1 implementation. For more than a
  handful of values, use a data texture as in `screen-data-texture`.
- **Data textures.** `load`, `size`, `format` and `sampleMode` behave as in
  [Data Textures](#data-textures).
- **Validation.** The screen conformance shaders compile under naga (WGSL) and
  glslangValidator (ESSL 100 and 300). glslangValidator does not enforce the
  WebGL 1 limits above, so `screen-meter-strip` is not validated for WebGL 1.
  The Metal source is compiled only by the
  optional `xcrun metal` check. No browser or GPU has drawn a screen pass in
  this repository, so blend and depth behaviour is the host's to confirm.

## Instancing

A mesh material with an authored `vertex()` may declare `instance name : type`
attributes and read `instanceIndex` (see the Instancing section of the
[language guide](language-guide.md)). The descriptor carries the record layout:

```json
"instanceAttributes": [
  { "name": "rect",   "location": 0, "type": "vec4", "offset": 0 },
  { "name": "tone",   "location": 1, "type": "vec4", "offset": 16 },
  { "name": "region", "location": 2, "type": "int",  "offset": 32 }
],
"instanceStride": 36
```

- `location` continues after `attributes[]`: with `position` and `uv` in
  `attributes` (locations 0 and 1), the first instance attribute is 2. All
  instance attributes are declared in the shader whether or not `vertex()`
  reads them, so the record layout never depends on the body.
- `offset` is the byte offset inside one record, and `instanceStride` is the
  record size. Attributes are packed in declaration order with no padding;
  every size is a multiple of 4 bytes (`float`, `int`, `uint` 4; `vec2` 8;
  `vec3` 12; `vec4` 16), so every offset and the stride are 4-byte aligned.
- The fields are absent for a material with no `instance` declaration, and
  older descriptors and artifact files decode as before.
- Hosts stream the record as little-endian 32-bit values. Write `int` as a
  signed and `uint` as an unsigned 32-bit integer, not as a float.

| Target | Instance index | Instance attributes | Host binding |
|---|---|---|---|
| `wgsl` | `@builtin(instance_index) instanceIndex : u32` | `@location(n)` members of `VertexInput`, after the vertex attributes | One vertex buffer with `stepMode: "instance"`, `arrayStride: instanceStride`, and an entry per attribute: `shaderLocation`, `offset`, and `format` `float32`, `float32x2`, `float32x3`, `float32x4`, `sint32` or `uint32` by `type`. `instance_index` includes `firstInstance`, so draw with `firstInstance` 0 for the same values as the other targets. |
| `gles` | `gl_InstanceID` as `uint instanceIndex` | `in` attributes, after the vertex attributes | `getAttribLocation` by name, or `bindAttribLocation` with the descriptor `location` before linking. `vertexAttribDivisor(loc, 1)` on each. Bind `int` and `uint` attributes with `vertexAttribIPointer` (`gl.INT`, `gl.UNSIGNED_INT`); `vertexAttribPointer` converts them to float and the shader reads garbage. Draw with `drawArraysInstanced`. An attribute the body never reads is optimised out by the driver, so skip a location of -1. |
| `glsl` | not available: rejected | `attribute` float types only; `int` and `uint` are rejected | Request `ANGLE_instanced_arrays` (listed in `requires.glExtensions`), then `vertexAttribDivisorANGLE(loc, 1)` and `drawArraysInstancedANGLE`. If the body reads `vertexIndex`, `a_vertexIndex` is a per-vertex attribute: leave its divisor at 0. |
| `metal` | `[[instance_id]]` as `uint instanceIndex` | `[[attribute(n)]]` members of `VertexIn`, after the vertex attributes | In the `MTLVertexDescriptor`, give the instance buffer a layout with `stepFunction = .perInstance`, `stride = instanceStride`, and attribute formats `float`..`float4`, `int` or `uint`. Use a buffer index other than 0, which is the uniform buffer. `instance_id` runs from the base instance to `baseInstance + instanceCount - 1`, like WGSL's `instance_index`, so draw with base instance 0 for the same values as the other targets. Only GLSL's `gl_InstanceID` excludes the base instance. |

**Choosing the first location.** By default the first instance attribute sits
directly after the vertex attributes. A host that already binds its own
per-instance data at fixed locations sets the base at compile time, because on
WebGPU the buffer layout's `shaderLocation` must equal the WGSL `@location` and
nothing can be remapped afterwards. The option is
`CompileOptions.FirstInstanceLocation` (Go), `builder.Options.FirstInstanceLocation`
(recorded in the artifact as `options.firstInstanceLocation`), and
`--first-instance-location N` on `selena emit`, `selena inspect`, and
`selena artifact`. Zero means the default and leaves today's output unchanged.
The value must be at least the number of vertex attributes, and the last
instance location must stay below 16 (WebGPU's default `maxVertexAttributes`);
otherwise compile fails with both numbers in the message. The WGSL `@location`,
the Metal `[[attribute(n)]]` index and `instanceAttributes[].location` all use
the base; the GLES attributes are bound by name or by `bindAttribLocation` with
the descriptor location. The byte offsets and `instanceStride` do not change.

Example: GoSX binds its engine instance record at locations 3 to 8. Compile the
material with `FirstInstanceLocation: 9`, and `rect`, `tone` and `region` of
NoteQuad take locations 9, 10 and 11, in a separate buffer layout from the
engine record. Up to 7 authored attributes fit this way (locations 9 to 15).

**`instanceIndex` is not an identity.** It is the draw-order index of the
instance within one draw call, counted from 0. If the host culls, sorts,
compacts or draws indirectly, the same note can get a different
`instanceIndex` from one frame to the next. A material that needs a stable
identity (for picking, or to look up per-note data) must carry the identity as
an instance attribute, such as `instance noteId : uint`. The host permutes the
records together with the instances, so the attribute stays attached to its
instance.

**GLSL ES 1.00 rules.** The target has no instance id, no integer vertex
attributes and no bit operators. Selena rejects a material that needs one,
naming each, and the message suggests the fix: pass an index as a float
instance attribute and convert it with `int()` in `vertex()`. A float-only
instance material builds for `glsl` and records `ANGLE_instanced_arrays`. The
default target set leaves `glsl` out for a material that cannot build there;
a request that names `glsl` gets the error.

**Integer varyings.** An `int` or `uint` varying (such as a region index passed
to `surface()`) is declared flat on `wgsl` (`@interpolate(flat)`), `gles`
(`flat out` and `flat in`, with `precision highp int` in the fragment shader)
and `metal` (`[[flat]]`). GLSL ES 3.00 and the WGSL specification
require `flat` on an integer varying (glslangValidator rejects the `gles` form
without it; naga does not enforce the WGSL rule, so a browser's WGSL compiler is
the check). Earlier versions emitted no `flat`, so no earlier valid output
changes. `glsl` has no integer varyings.

**GLES and Metal uint literals.** A material that uses instancing features
keeps the `u` suffix on `uint` literals in `gles` and `metal` output, since
GLSL ES 3.00 has no implicit `int` to `uint` conversion. Other materials keep
the older bare spelling so their output does not change.

**Validation.** The conformance shaders compile under naga (WGSL) and
glslangValidator (ESSL 100 and 300). The Metal source is compiled only by the
optional `xcrun metal` check. No browser or GPU has drawn an instanced pass in
this repository.

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
fragment and both attachments. Metal output compiles with `xcrun metal` where
Xcode's Metal toolchain is installed (see Validating Metal output).
