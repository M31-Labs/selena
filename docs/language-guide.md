# Selena Language Guide

Selena `.sel` files define one or more materials plus optional reusable
functions. A material declares host-provided params and one `surface(geo)` body.
Selena lowers the surface once, then emits WGSL, WebGL GLSL, Metal, and GLES.
Selena materials are currently complete custom shaders. Standard/PBR material
hooks are planned separately; see
[standard-material-interop.md](standard-material-interop.md).
Vertex hooks are not part of the stable `.sel` surface yet.

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
- Mesh, post and feedback materials support array params. Points materials do
  not yet.

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
