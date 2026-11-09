# Game materials

`materiallib.Game` (module `game`) holds helpers for the looks games reach
for: plants that sway in the wind, cel shading, objects that burn away,
holograms and shields, ice and water, and skin that does not look like
plastic. Each helper takes plain vectors, so it works with the
[lighting module](lighting.md) (every scene light) or with the single key
light `sunDir` and `sunColor`.

```go
result, err := materiallib.Compile(source, selena.CompileOptions{}, materiallib.Game, materiallib.Lighting)
```

In a GoSX app, list the modules in `gosx.config.json`:

```json
{ "build": { "selena": { "modules": ["game", "lighting"] } } }
```

Every material below is a full material, an example in
[`examples/material-library`](../../examples/material-library), and has
golden WGSL, GLSL, GLES and Metal output in `materiallib/testdata/game`. The
tests validate each with naga and glslangValidator, and with `xcrun metal` on
a Mac with Xcode's Metal toolchain.

| Example | Look | Modules | Draw it with |
|---|---|---|---|
| `foliage.sel` | Two-sided leaves, wind sway, backlight glow | game, lighting | `DoubleSided` |
| `toon.sel` | Three light bands, hard highlight, lit rim, outline | game | |
| `dissolve.sel` | Burns away as `progress` goes 0 to 1, glowing edge | game | |
| `hologram.sel` | Rim glow, moving scanlines, flicker | game | additive blend |
| `force-field.sel` | Energy shell, sweeping bands, impact ring | game | additive blend, no depth write |
| `ice.sel` | Deep body color, white rim, sparkling facets | game, lighting | |
| `water-surface.sel` | Rippled normals, Fresnel sky reflection, sun glint | game | |
| `skin.sel` | Soft shadow line, warm scatter band, backlit ears | game, lighting | |

## Wind

`mlWindSway` and `mlWindGust` read the scene wind through two engine inputs,
declared in the material's `context` block:

| Input | Encoding |
|---|---|
| `wind : vec4` | xy: unit direction the wind blows toward (world x and z), z: speed in m/s, w: wind clock in seconds |
| `windGust : vec4` | x: gust strength 0..1, y: meters between gust fronts, z: front speed in m/s, w: flutter frequency in Hz |

This is the encoding of GoSX's `scene.Wind`, and `mlWindGust` evaluates the
same gust expression as `scene.Wind.Sample`, so gameplay (sails, smoke,
particles) and plants agree. The test `TestWindGustMatchesGoSXWind` checks it.
Both inputs are zero when the host supplies no wind, and the plants stand still.

```selena
vertex(geo) -> vec4 {
    let p = geo.position + mlWindSway(geo.position, geo.uv.y, stiffness, wind, windGust)
    worldPos = p
    worldNormal = normalMatrix * geo.normal
    return mvp * vec4f(p, 1.0)
}
```

`bend` (here `geo.uv.y`) is 0 at the root and 1 at the tip; the offset grows
with its square, so stems stay planted. Export leaf cards with V running up
the blade, or pass a height you compute.

## Helpers

| Helper | Returns | Use |
|---|---|---|
| `mlWindGust(worldPos, wind, gust)` | `float` | Gust factor 1 ± strength at a point. |
| `mlWindSway(worldPos, bend, stiffness, wind, gust)` | `vec3` | World offset for a plant vertex; `stiffness` 1 suits grass. |
| `mlFoliageNormal(n, v)` | `vec3` | Turns the normal toward the viewer for two-sided leaves. |
| `mlFoliageTranslucency(n, v, l, thickness)` | `float` | Light from `l` that shows through a leaf. |
| `mlToonBands(ndl, bands, softness)` | `float` | Lambert term in flat steps. |
| `mlToonSpecular(n, v, l, size, softness)` | `float` | Hard-edged highlight. |
| `mlToonRim(n, v, l, width, softness)` | `float` | Rim on the lit side. |
| `mlToonOutline(n, v, width)` | `float` | 1 on the silhouette, about `width` pixels wide; surface() only. |
| `mlDissolveMask(noise, progress)` | `float` | Positive where the surface remains; discard below 0. |
| `mlDissolveEdge(mask, width)` | `float` | 1 in the burning band inside the cut. |
| `mlScanlines(height, time, density, speed, sharpness)` | `float` | Moving line pattern along a height. |
| `mlHologram(n, v, worldPos, time, density, flicker)` | `float` | Hologram brightness. |
| `mlForceField(n, v, worldPos, time)` | `float` | Shield brightness: rim and sweeping bands. |
| `mlForceFieldHit(worldPos, hitPos, age, speed, width)` | `float` | Ring expanding from an impact, fading over 1 s. |
| `mlWaterFresnel(n, v)` | `float` | Schlick reflectance with water's F0 of 0.02. |
| `mlWaterRipples(xz, time, scale, strength)` | `vec3` | Upward normal tilted by three moving waves. |
| `mlIceSparkle(worldPos, n, v, l, density)` | `float` | Glints from randomly tilted facets. |
| `mlIceTint(surface, deep, n, v)` | `vec3` | Body color that deepens when seen head on. |
| `mlSubsurfaceWrap(n, l, wrap)` | `float` | Wrapped diffuse that softens the shadow line. |
| `mlSubsurfaceBack(n, v, l, distortion, power, thickness)` | `float` | Light that scatters through thin parts toward the viewer. |
| `mlSubsurfaceTint(n, l, scatter, width)` | `vec3` | Scatter color near the shadow line. |
| `mlRimFactor(n, v, power)` | `float` | 1 where the surface is seen edge on. |
| `mlGameHash(p)`, `mlGameNoise(p)`, `mlGameNoise3(p)` | `float` | Hash and value noise, 0..1. |

`n` and `v` are unit vectors, with `v` pointing from the surface to the
camera; `l` points from the surface toward the light.

## Limits

- `mlToonOutline` uses screen derivatives and draws inside the silhouette. For
  a thick, even outline, add a slightly larger copy of the mesh with front
  faces culled and an unlit dark material (an inverted hull).
- `force-field.sel` draws no contact glow where the shell cuts into geometry;
  that needs screen-space depth, which a full material cannot read yet.
- These are full materials: they light themselves and skip the host's
  image-based lighting and tone mapping inputs. Dissolve and ice could also be
  written as surface-kind materials once the host's surface hook is available
  in a renderer.
