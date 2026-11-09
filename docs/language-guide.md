# Selena Language Guide

Selena `.sel` files define one or more materials plus optional reusable
functions. A material declares host-provided params and one `surface(geo)` body.
Selena lowers the surface once, then emits WGSL, WebGL GLSL, Metal, and GLES.
Selena materials are currently complete custom shaders. Standard/PBR material
hooks are planned separately; see
[standard-material-interop.md](standard-material-interop.md).
A material can read renderer-owned values and textures by declaring them in a
`context` block; see [engine-inputs.md](engine-inputs.md).
A mesh or general material may also author a `vertex() -> vec4` stage — see
[compatibility.md](compatibility.md) for the emitted shape on each backend. A
separate composing vertex hook that mutates geometry while the default
pipeline still runs is not part of the stable `.sel` surface yet.

`vertex()` can read a `texture2d` param with `sampleLevel(tex, uv, lod)`, for
example to displace vertices from a height texture (see
`testdata/conformance/vertex-texture.sel`). The LOD is explicit because a vertex
shader has no derivatives; `sample`, `sampleCube` and `dpdx`/`dpdy`/`fwidth` are
rejected there. The descriptor marks such textures `vertexStage: true`, and a
WebGL 1 host checks `requires.glVertexTextureUnits`.

## Solid Color

```selena
material Solid {
    param baseColor : color = rgb(0.78, 0.42, 0.98)

    surface(geo) -> color {
        return baseColor
    }
}
```

Compile and inspect:

```sh
go run ./cmd/selena inspect solid.sel
```

## Directional Diffuse

`Sun` expands into descriptor fields for `light_dir` and `light_ambient`.
`geo.worldNormal` asks Selena to synthesize the normal attribute, varying, and
normal-matrix transform.

```selena
material DirectionalDiffuse {
    param baseColor : color = rgb(0.78, 0.42, 0.98)
    param light : Sun

    surface(geo) -> color {
        let n = normalize(geo.worldNormal)
        return baseColor * (light.ambient + max(dot(n, light.dir), 0))
    }
}
```

## Textured Diffuse

`texture2d` params become texture/sampler descriptor entries. `sample(texture,
uv)` returns `vec4`; swizzle `.rgb` when the surface returns `color`.

```selena
material Textured {
    param albedo : texture2d
    param light : Sun

    surface(geo) -> color {
        let c = sample(albedo, geo.uv).rgb
        let n = normalize(geo.worldNormal)
        return c * (light.ambient + max(dot(n, light.dir), 0))
    }
}
```

## Reusable Functions

Top-level `fn` declarations are inlined during lowering, so they add structure
without adding backend function support requirements.

```selena
fn diffuse(n: vec3, dir: vec3, ambient: float) -> float {
    return ambient + max(dot(n, dir), 0)
}

material Composed {
    param baseColor : color = rgb(0.78, 0.42, 0.98)
    param light : Sun

    surface(geo) -> color {
        let n = normalize(geo.worldNormal)
        return baseColor * diffuse(n, light.dir, light.ambient)
    }
}
```

## Material Inheritance

Use `extends` and `super.surface(geo)` to build a derived material. When no
material name is passed to the CLI or `CompileOptions`, Selena compiles the last
material in the file.

```selena
material Base {
    param baseColor : color = rgb(0.78, 0.42, 0.98)
    param light : Sun

    surface(geo) -> color {
        let n = normalize(geo.worldNormal)
        return baseColor * (light.ambient + max(dot(n, light.dir), 0))
    }
}

material Tinted extends Base {
    param tint : color = rgb(1.0, 0.8, 0.9)

    surface(geo) -> color {
        return super.surface(geo) * tint
    }
}
```

Compile the base or derived material explicitly:

```sh
go run ./cmd/selena emit wgsl tinted.sel Base
go run ./cmd/selena emit wgsl tinted.sel Tinted
```

## Supported Types

Params currently support:

- `float`
- `vec2`
- `vec3`
- `vec4`
- `mat3`
- `mat4`
- `color`, lowered as `vec3`
- `Sun`, expanded into uniforms
- `texture2d`, emitted as backend-specific texture/sampler bindings
- `array<T, N>`, a fixed-size uniform array (see below)
- `textureDepth2d` and `textureDepth2dArray`, engine depth textures declared
  only in `context` (`opaqueDepth`, and the shadow maps `shadowMap0` and
  `shadowMap1`); read them with `sampleDepth(tex, uv)` and
  `sampleDepth(tex, uv, layer)`. See [engine-inputs.md](engine-inputs.md).

Defaults currently support constant `float`, `vec2`, `vec3`, `vec4`, `mat3`,
`mat4`, and `color` values. Matrix values are written in column-major order, the
same order accepted by the Go packer:

```selena
param gain : float = 1.0
param uvScale : vec2 = vec2(1.0, 1.0)
param baseColor : color = rgb(0.78, 0.42, 0.98)
param basis : mat3 = mat3(1, 0, 0, 0, 1, 0, 0, 0, 1)
```

`Sun` and `texture2d` defaults are intentionally rejected for now.

## Fixed-Size Array Params

A param can be a fixed-size array of a scalar or vector type. Write the type as
`array<T, N>`. The C-style spelling `vec4[8]` is not accepted; Selena reports it
at the declaration and names the correct form.

```selena
param rects : array<vec4, 8>
```

Rules:

- `N` must be a positive integer literal.
- Array params take no default value.
- Every element is packed at the std140 array stride of 16 bytes, whatever `T`
  is. The descriptor field carries `count` and `stride` so the host can pack it.
- Mesh, post, screen and feedback materials support array params. Points
  materials do not yet.

Read an element with `rects[i]`. The index must be `int` or `uint`, so write the
loop counter with the `i` integer-literal suffix — `0i`, not `0`. A bare `0` is
a float literal, and `rects[i]` then reports `SEL2005: array index must be int
or uint, got float`.

```selena
material Panels kind post {
    param rects : array<vec4, 8>

    surface(post) -> color {
        var best = 1.0
        for (var i = 0i; i < 8i; i = i + 1i) {
            let r = rects[i]
            let dx = abs(post.uv.x - r.x) - r.z
            let dy = abs(post.uv.y - r.y) - r.w
            best = min(best, length(vec2f(max(dx, 0.0), max(dy, 0.0))))
            if (best < 0.0) {
                break
            }
        }
        let c = sceneColor(post.uv)
        return rgb(c.r, c.g, c.b * best, c.a)
    }
}
```

## Data Textures

A `texture2d` param can hold data instead of an image: one spectrum row per
audio frame, a scope trace, a lookup table. Three additions read it exactly.

- `param spectrum : texture2d format r16float` declares the storage format. The
  formats are `rgba8unorm` (the default when you write no `format`),
  `rgba16float`, `rgba32float`, `r16float` and `r32float`. `format` is allowed
  only on `texture2d` params. It tells the host which GPU texture to create and
  appears in the descriptor as `textures[].format`; it does not change the
  emitted shader.
- `load(tex, ix, iy)` returns the `vec4` stored at integer texel `(ix, iy)` of
  the base level. There is no filtering and no UV math. `ix` and `iy` are `int`:
  convert with `int(x)` or write a literal such as `3i`. A one-channel format
  returns `(r, 0, 0, 1)`. It works in `surface()` and in an authored `vertex()`.
- `size(tex)` returns the width and height in texels as a `vec2`.

```selena
material SpectrogramBand {
    param spectrum : texture2d format r16float
    param head : float

    surface(geo) -> color {
        let sz = size(spectrum)
        let col = int(mod(geo.uv.x * sz.x + head, sz.x))
        let row = int(geo.uv.y * sz.y)
        let mag = load(spectrum, col, row).r
        return rgb(mag, mag * 0.6, 1.0 - mag)
    }
}
```

A complete material that also uses `load` in `vertex()` is
`testdata/conformance/data-texture.sel`.

Use `load` when the texture is a table of values. A texel then reaches the
shader unchanged, whatever the viewport or UV scale. Use `sample` when you want
a filtered image. Two cases leave no choice:

- `r32float` and `rgba32float` cannot be filtered on WebGPU without the
  `float32-filterable` feature, so `sample` and `sampleLevel` on them are a
  compile error that points you to `load`.
- `cube` and depth textures have no `load`; use `sampleCube` and `sampleDepth`.

Clamp your coordinates. A coordinate outside the texture is not defined to give
the same value on every target: WebGPU and Metal define the result, GLSL ES 3.00
leaves it undefined, and GLSL ES 1.00 returns the edge texel. `load` and `size` are ordinary
names to the language otherwise, so a local called `size` still compiles.

The descriptor says how each texture is read: `textures[].sampleMode` is
`"load"` when only `load` reads it and `"both"` when the material also samples
it. A texture that is only sampled has no `sampleMode`. See
[compatibility.md](compatibility.md) for the per-target spelling and host
requirements.

## Numeric Literals

Selena has three numeric literal forms, and the suffix chooses the type:

| Literal | Type    | Use |
|---------|---------|-----|
| `0`, `1.5` | `float` | all shading math |
| `0i`, `8i` | `int`   | loop counters, array indices |
| `0u`, `8u` | `uint`  | unsigned counters |

There is no implicit conversion between them. Convert explicitly with
`float(x)`, `int(x)` or `uint(x)`. A `for` loop takes the type of its init
value, so `for (var i = 0i; i < 8i; i = i + 1i)` gives an `int` counter and
`for (var i = 0; i < 8; i = i + 1)` gives a `float` one. Use the `i` form
whenever the counter indexes an array.

## Surface Materials

A material whose surface returns `Surface` instead of a color is a surface
material. It describes the surface; the host's standard lighting shades it,
so it shares lights, shadows, image-based lighting, fog and exposure with the
host's StandardMaterial.

```selena
material Lacquer {
    param tint : color = rgb(0.6, 0.1, 0.1)
    surface(geo) -> Surface {
        return withClearcoat(pbr(tint, 0.6, 0.0), 1.0, 0.05)
    }
}
```

`pbr(baseColor, roughness, metallic)` builds the record, the `with*` setters
replace one field, `mixSurface(a, b, t)` blends two, and `s.roughness` reads a
field. Surface materials emit WGSL and GLES fragment hooks only. The fields,
builtins, rules and host ABI are in
[Standard Material Interop](standard-material-interop.md); the
[surface module](material-library/surface.md) adds texture-set, triplanar and
layering helpers.

## Post Materials

A `kind post` material runs one fullscreen pass over the rendered scene. Its
surface reads `post.uv` (screen UV in `[0,1]`) and these engine builtins:

| Builtin | Result | Meaning |
|---------|--------|---------|
| `sceneColor(uv)` | `vec4` | the backdrop colour at `uv` |
| `sceneDepth(uv)` | `float` | the backdrop depth at `uv` |
| `sceneColorLevel(uv, lod)` | `vec4` | the backdrop colour at mip level `lod` |
| `sceneSize()` | `vec2` | the backdrop size in pixels |

`sceneColorLevel` replaces an N-tap blur kernel with one pre-filtered tap, which
is the cost driver for frosted-glass passes. `sceneSize()` lets a kernel radius
be written in pixels rather than UV corrected by an app-supplied aspect uniform.

Both need the host to cooperate, and the compiled descriptor says so in
`layout.requires`:

- `sceneColorLevel` sets `requires.sceneColorMips`. The host must render the
  post source into a texture that HAS a mip chain, regenerate the mips before
  the pass, and bind a sampler whose minification filter walks mips. Without a
  mip chain every backend clamps to level 0 and the pass renders unblurred.
- `sceneSize()` sets `requires.glSceneSizeUniform` to `_sceneSize`. GLSL ES 1.00
  has no `textureSize`, so the WebGL 1 artifact declares that uniform and the
  host must set it. WGSL, GLES and Metal read the size off the bound texture.
- Derivative builtins (`fwidth`, `dpdx`, `dpdy`) add `OES_standard_derivatives`
  to `requires.glExtensions`. See [compatibility.md](compatibility.md).

Post materials cannot declare `texture2d` params; the engine provides the scene
textures. They can declare scalar, vector and `array<T, N>` params.

## Screen Materials

A `kind screen` material draws one fullscreen pass that returns a colour with
alpha. Unlike `post`, it reads no scene colour: it is for overlays such as level
meters, spectrograms and scopes that the author computes from params and data
textures. The host decides where the pass lands and how it combines with what is
already there.

```selena
material MeterStrip kind screen {
    render { blend additive  depth off }

    param levels : array<vec4, 128>   // (peakL, peakR, rmsL, rmsR) in dB
    param strips : float = 16.0

    surface(screen) -> color {
        let x = screen.uv.x * strips
        let i = int(min(floor(x), 127.0))
        let l = levels[i]
        let peak = fract(x) < 0.5 ? l.x : l.y
        let lit = step(1.0 - screen.uv.y, (peak + 60.0) / 60.0)
        return rgb(lit * 0.2, lit, lit * 0.4, lit)
    }
}
```

The full version, with peak-hold marks and a one-pixel gap between strips, is
`testdata/conformance/screen-meter-strip.sel`; a data texture version is
`screen-data-texture.sel`.

**Inputs.**

- `screen.uv` is a `vec2`, `0..1` across the target rectangle. The origin is
  the **top left**: `uv.y` is 0 on the top row and grows downward, on every
  target. A bar that grows from the bottom therefore compares against
  `1.0 - screen.uv.y`. `load(tex, int(uv.x * w), int(uv.y * h))` reads texture
  row 0 at the top. (`post` is not uniform here: its WebGL 1 `uv` starts at the
  bottom left, the other targets at the top left. `screen` is the same on all
  four.) The name `screen` is the geometry binding you
  choose in `surface(screen)`, as `geo` is for a mesh.
- `screenSize()` returns the size of the target rectangle in pixels as a
  `vec2`, for example to draw a line one pixel wide whatever the rectangle's
  size. The host writes it into the descriptor field `screenSize` before each
  draw. It is available only in screen materials.

**Params.** Scalar, vector, matrix and `array<T, N>` params, and `texture2d`
params with the `format` qualifier, `load()`, `size()` and `sample()` from
[Data Textures](#data-textures). `Sun` and `textureCube` params are rejected.
`sceneColor`, `sceneDepth`, `sceneColorLevel` and `sceneSize` are `post`-only
and report an error in a screen material. A param or context field cannot be
named `screenSize`. An authored `vertex()`, `varying` or `state` is rejected:
Selena emits the vertex stage.

**Result.** The body returns a colour. `rgb(r, g, b, a)` sets alpha; a
three-component result has alpha 1. For `blend premultiplied`, return colour
already multiplied by alpha.

**The `render` block.** It is optional, and only a screen material may write
it. Selena records the state in the descriptor; the host applies it.

| Setting | Words | Default | Meaning |
|---------|-------|---------|---------|
| `blend` | `opaque`, `alpha`, `premultiplied`, `additive` | `alpha` | `opaque` replaces the target; `alpha` is `src*a + dst*(1-a)`; `premultiplied` is `src + dst*(1-a)`; `additive` is `src*a + dst` |
| `depth` | `off`, `test`, `write` | `off` | `off` skips the depth buffer; `test` tests without writing; `write` tests and writes |

A derived material (`extends`) inherits its parent's `render` block unless it
writes its own. An unknown key or word, a repeated key, or a `render` block on
another kind is error `SEL1008`.

**Placement.** Selena draws a fullscreen triangle. It does not take host
geometry. To draw into part of a window, the host sets the viewport and scissor
to that rectangle; `screen.uv` and `screenSize()` then describe the rectangle,
not the window. See [compatibility.md](compatibility.md#screen-materials) for the
bindings and the draw call.

## Instancing

A mesh material that authors `vertex()` can draw many copies of one mesh in a
single call, each with its own data. Declare the per-instance data with
`instance name : type`, and read it by name in `vertex()`:

```selena
material NoteQuad {
    instance rect   : vec4   // x, y, w, h
    instance tone   : vec4   // r, g, b, selected
    instance region : int    // atlas region index

    varying vTone : vec4
    varying vRegion : int

    vertex(geo) -> vec4 {
        let corner = vec2f(float(vertexIndex & 1u), float(vertexIndex >> 1u))
        let p = rect.xy + corner * rect.zw
        vTone = tone
        vRegion = region
        return mvp * vec4f(p, 0.0, 1.0)
    }

    surface(geo) -> color {
        return rgb(geo.vTone.r, geo.vTone.g, geo.vTone.b)
    }
}
```

This draws a quad per instance from four vertices as a triangle strip, with no
vertex buffer. The full material is `testdata/instancing/note-quad.sel`.

**Instance attributes.** The allowed types are `float`, `vec2`, `vec3`, `vec4`,
`int` and `uint`. The attribute is readable in `vertex()` only. To use it in
`surface()`, write it to a `varying`, as `vRegion` does above. An `int` or
`uint` varying is passed flat (without interpolation) on every target except
GLSL ES 1.00, which cannot carry one. `instance` is a keyword, and it is only
legal in a mesh material that has a `vertex()`. Points, post, feedback, screen
and surface materials, and a mesh material with no `vertex()`, reject it with a
diagnostic.

**`instanceIndex`.** A `uint` builtin beside `vertexIndex`: the index of the
instance being drawn, counted from 0. It works without any `instance`
declaration, and it is not available on GLSL ES 1.00.

**`instanceIndex` is not an identity.** It is the draw-order index of the
instance within the draw call. If the host culls, sorts, compacts or draws
indirectly, an instance can get a different `instanceIndex` from frame to frame.
When you need a stable identity, carry it as an attribute (`instance noteId :
uint`): the host reorders the records and the identity moves with its instance.

**Record layout.** The descriptor lists the attributes in declaration order and
packs them into one record with no padding. `rect`, `tone` and `region` above
sit at byte offsets 0, 16 and 32, and the record is 36 bytes. Locations come
after the vertex attributes the `vertex()` body reads, in the order
`position`, `normal`, `uv`. See [compatibility.md](compatibility.md#instancing)
for the descriptor fields and how each target binds the buffer.

**Bit operators.** `&`, `|`, `^`, `<<` and `>>` work on `int` and `uint`.
`&`, `|` and `^` take two operands of the same type. A shift takes an `int` or
`uint` on the left and a `uint` on the right (write `1u`), and the result has
the type of the left operand. Precedence follows Go: `&`, `<<` and `>>` bind
like `*`, `|` and `^` bind like `+`, and all of them bind tighter than a
comparison, so `x & 1u == 0u` means `(x & 1u) == 0u`. They are not allowed in a
param default. They are not available on GLSL ES 1.00.

A material that uses `instanceIndex`, an `int` or `uint` instance attribute, or
a bit operator has no GLSL ES 1.00 form. Compiling it with no target list emits
the other three targets and leaves `glsl` out; asking for `glsl` by name fails
with a message that lists each feature in the way. An instance material that
uses only float attributes and no bit operators builds for all four targets.

## Host Packing

The descriptor is the source of truth for uniform packing. In Go, use
`bindings.PackUniformsWithDefaults` so omitted values can come from `.sel`
defaults. Descriptor JSON includes `schemaVersion` (`selena.descriptor.v1`) and
`languageVersion` (`selena.lang.v1`) so hosts can reject incompatible artifacts
before binding them:

```go
packed, err := bindings.PackUniformsWithDefaults(res.Layout, map[string]any{
    "mvp": matrixMVP,
    "normalMatrix": normalMatrix,
    "light_dir": []float32{0.4, 0.85, 0.6},
    "light_ambient": 0.16,
})
```

The browser demo generated by `selena demo ... defaults` also reads
`uniformBlock.defaults` from the descriptor before packing. The GoSX adapter
surfaces those same defaults through `IRMaterial.CustomUniforms`.

Avoid using shader keywords, generated names, or Selena stdlib names for params
and locals. Selena rejects names such as `var`, `texture`, `position`,
`fragColor`, `rgb`, and `sample` before backend emission.

## Diagnostics

File-backed CLI commands render source snippets for diagnostics with ranges:

```text
selena: SEL2001 at 3:16: unknown name "missing"
  --> bad.sel:3:16
  |
3 |         return missing
  |                ^^^^^^^ unknown name "missing"
hint: Declare a material param or let binding with this name, or correct the identifier.
```

## Editor Support

`selena lsp` runs the Selena language server on stdin and stdout. `gosx lsp`
serves `.sel` files with the same server inside a GoSX project. It provides:

- compiler diagnostics as you type, with the code (for example `SEL2001`) and
  the hint shown above;
- hover with the type of params, context fields, locals and `geo` fields, the
  registry entry of each engine input (type, space, meaning), and the signature
  and documentation of builtins and material library helpers;
- completion of builtins, material library helpers, the file's functions,
  params and locals, `geo.` fields for the material's kind, engine inputs inside
  a `context` block, and type names after `:`;
- go-to-definition for locals, params, functions and materials in the file, and
  for material library helpers, whose module source it writes to the user cache
  directory (`selena/materiallib/<module>.sel`).

A file that calls a material library helper, such as `mlFog`, links that
helper's module without configuration. While a line is half typed, the server
keeps answering from the rest of the file.

Register the `.sel` extension with your editor's LSP client and run `selena
lsp` (or `gosx lsp`). For example, in Neovim:

```lua
vim.filetype.add({ extension = { sel = "selena" } })
vim.lsp.config("selena", { cmd = { "selena", "lsp" }, filetypes = { "selena" } })
vim.lsp.enable("selena")
```
