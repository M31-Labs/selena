# Standard Material Interop

Selena's long-term job is to make custom material authoring feel as portable as
GoSX standard materials. That does not mean forking GoSX's PBR renderer.

This document pins the current boundary and the intended extension path.

## Current Boundary

Selena currently emits complete custom shaders.

- The GoSX adapter returns an `IRMaterial` with `Kind: "custom"`.
- The adapter fills GLSL and WGSL custom shader slots from one `.sel` source.
- Descriptor defaults flow into `IRMaterial.CustomUniforms`.
- Standard material fields such as `Roughness`, `Metalness`, `NormalMap`,
  `RoughnessMap`, `MetalnessMap`, image-based lighting, tone mapping, and
  environment integration remain owned by the GoSX standard material renderer.

In other words, today's Selena material is a custom material that is fully
served across shader backends. It is not yet a standard/PBR material override.

## Why The Boundary Matters

The standard material path already handles production PBR concerns:

- backend-specific lighting code
- roughness/metalness workflows
- normal, roughness, metalness, and emissive maps
- renderer tone mapping and post-processing expectations
- capability fallback through GoSX's backend gate

Duplicating that logic inside Selena would create two material systems that
drift. The better model is for Selena to author small, typed hooks into the
standard material pipeline.

## Intended Interop Model

The next interop layer should be hook-based:

```selena
material BrandedPlastic extends StandardLit {
    param stripe : color = rgb(0.1, 0.7, 1.0)
    param roughnessBoost : float = 0.12

    surface(geo, pbr) -> PBRSurface {
        let base = super.surface(geo, pbr)
        return pbrSurface(
            base.color * stripe,
            clamp(base.roughness + roughnessBoost, 0, 1),
            base.metalness
        )
    }
}
```

The exact syntax is not committed yet, but the contract should be:

- `StandardLit` stays a renderer-owned shading model.
- Selena can override typed surface properties, not the whole lighting loop.
- Standard material maps remain host assets and stay in the GoSX descriptor.
- Selena-generated hooks must emit for every backend before the material claims
  standard-material compatibility.
- The binding descriptor must expose which standard slots and custom params are
  required.

## Candidate Hook Outputs

The first hook surface should stay small:

| Output | Type | Purpose |
|---|---|---|
| `color` | `color` / `vec3` | base color before lighting |
| `roughness` | `float` | roughness override or adjustment |
| `metalness` | `float` | metalness override or adjustment |
| `emissive` | `color` / `vec3` | emitted light contribution |
| `alpha` | `float` | opacity / cutout input |
| `normal` | `vec3` | optional perturbed normal in the same space the renderer expects |

Add normal last. Normal-space mistakes are expensive and should be covered by
browser pixel tests before being presented as stable.

## Non-Goals For V1

- No arbitrary lighting loops in `.sel`.
- No separate Selena PBR implementation.
- No implicit use of standard material texture maps from a custom shader.
- No backend-specific escape hatches in the `.sel` source.
- No claim that custom materials inherit standard material behavior unless the
  descriptor and GoSX adapter can prove the backend is served.

## Practical Next Steps

1. Add a typed `PBRSurface` record to HIR/lowering without changing emitters.
2. Add parser support for a `surface(geo, pbr) -> PBRSurface` form.
3. Add a GoSX adapter path that marks a material as a standard hook instead of
   a full custom shader.
4. Add golden outputs for every hook target and browser pixel tests before the
   feature is documented as stable.
