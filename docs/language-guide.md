# Selena Language Guide

Selena `.sel` files define one or more materials plus optional reusable
functions. A material declares host-provided params and one `surface(geo)` body.
Selena lowers the surface once, then emits WGSL, WebGL GLSL, Metal, and GLES.

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

## Host Packing

The descriptor is the source of truth for uniform packing. In Go, use
`bindings.PackUniformsWithDefaults` so omitted values can come from `.sel`
defaults:

```go
packed, err := bindings.PackUniformsWithDefaults(res.Layout, map[string]any{
    "mvp": matrixMVP,
    "normalMatrix": normalMatrix,
    "light_dir": []float32{0.4, 0.85, 0.6},
    "light_ambient": 0.16,
})
```

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
