# Scene lighting

`materiallib.Lighting` lets a custom mesh material use the scene's lights,
its directional shadow cascades and its fog. The renderer supplies them as
engine inputs (see [Scene lighting](../engine-inputs.md#scene-lighting)); the
helpers do the per-light math the GoSX standard material does.

```go
result, err := materiallib.Compile(source, selena.CompileOptions{}, materiallib.Lighting)
```

```sh
selena artifact -modules lighting materials/rock.sel
```

In a GoSX app, add the module to `gosx.config.json`:

```json
{ "build": { "selena": { "modules": ["lighting"] } } }
```

## Pattern

Selena functions cannot loop, so the material loops over the light capacity
(8) and passes each index to a helper. Compute the two shadow factors once,
before the loop: each costs five depth reads.

```selena
let n = normalize(geo.worldNormal)
let v = normalize(cameraPos - geo.worldPos)
let s0 = mlShadow0(geo.worldPos)
let s1 = mlShadow1(geo.worldPos)
var lit = ambient * albedo
for (var i = 0i; i < 8i; i = i + 1i) {
    let shadow = mlLightShadow(i, s0, s1)
    lit = lit + (albedo * mlLightDiffuse(i, n, geo.worldPos) + 0.25 * mlLightSpecular(i, n, v, geo.worldPos, 48.0)) * shadow
}
return mlFog(lit, geo.worldPos)
```

Declare in `context` every engine input the helpers you call read. The full
block is in [`examples/scene-lit.sel`](../../examples/scene-lit.sel). A missing
declaration is a compile error that names the input (`SEL2001 unknown name`).

## Functions

| Function | Reads | Returns |
|---|---|---|
| `mlLightDiffuse(i, n, worldPos)` | `lights`, `lightCount` | Lambert irradiance of light `i` (no cosine for ambient and hemisphere lights) |
| `mlLightSpecular(i, n, v, worldPos, shininess)` | `lights`, `lightCount` | Energy-normalized Blinn-Phong highlight of light `i` |
| `mlLightRadiance(i, n, worldPos)` | `lights`, `lightCount` | Color times intensity, falloff and cone, before the cosine term |
| `mlLightDir(i, worldPos)` | `lights` | Unit vector toward light `i` |
| `mlLightKind(i)`, `mlLightActive(i)` | `lights`, `lightCount` | Kind code; 1 below `lightCount`, else 0 |
| `mlShadow0(worldPos)`, `mlShadow1(worldPos)` | `cameraPos`, `cameraForward`, `shadowN`, `shadowSplitsN`, `shadowMatricesN`, `shadowMapN` | 1 lit, 0 shadowed, filtered over five taps; 1 for an empty slot or outside the map |
| `mlLightShadow(i, s0, s1)` | `lights` | `s0` or `s1` when light `i` casts that slot's shadow, else 1 |
| `mlShadowCascade(depth, count, splits)` | none | Cascade index for a view depth |
| `mlFog(color, worldPos)`, `mlFogFactor(worldPos)` | `cameraPos`, `fog` | Color blended toward the fog color; the surviving fraction `exp(-(density * distance)^2)` |

## What it matches, and what it does not

- Point and spot falloff (`range`, `decay`), the spot cone (`angle`,
  `penumbra`), the hemisphere blend and the flat ambient term match the GoSX
  standard material. `materiallib` tests evaluate the emitted IR against those
  formulas.
- The specular term is Blinn-Phong, not the standard material's GGX. Use the
  [BRDF](brdf.md) module with `mlLightRadiance` and `mlLightDir` for GGX.
- Rect-area lights and light probes are not packed into `lights`; the renderer
  folds light probes into `ambient`.
- The shadow filter is five taps at `shadowN.z` uv apart. The standard
  material's own filter may differ by renderer.
