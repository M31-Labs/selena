# Surface authoring

`materiallib.Surface` holds the helpers a surface-kind material needs to turn
texture sets into a `Surface`: PBR texture sets, triplanar projection, height
and slope layer masks, and detail normals. A surface material returns the
typed record and the host's standard lighting shades it; see
[Standard material interop](../standard-material-interop.md).

```go
result, err := materiallib.Compile(source, selena.CompileOptions{}, materiallib.Surface)
```

```sh
selena artifact -targets wgsl,gles -modules surface materials/rock.sel
```

In a GoSX app, add the module to `gosx.config.json`:

```json
{ "build": { "selena": { "modules": ["surface"] } } }
```

## Helpers

A normal is a unit world-space vector unless its name says tangent. Pass geo
fields straight in: `geo.uv`, `geo.worldPos`, `geo.worldNormal`, `geo.tangent`
and `geo.bitangent`.

| Helper | Returns | Use |
|---|---|---|
| `mlPBRSet(baseColor, normalMap, orm, uv, tangent, bitangent, normal, normalStrength)` | `Surface` | A UV-mapped texture set: base color (sRGB, alpha in a), a tangent-space normal map, and a glTF ORM map (r occlusion, g roughness, b metallic). |
| `mlSurfaceFromSet(base, normal, orm)` | `Surface` | The same from already sampled texels. |
| `mlUnpackNormal(texel, strength)` | `vec3` | Decodes a normal-map texel to a tangent normal; strength scales the slope. |
| `mlTangentToWorld(tangentNormal, tangent, bitangent, normal)` | `vec3` | Tangent normal to world, with the frame orthogonalized against `normal`. |
| `mlTriplanar(tex, worldPos, normal, scale, sharpness)` | `vec4` | Samples `tex` along the three world axes, `scale` repeats per meter. No UVs are read, so a rotated mesh shows no seam; negative faces are not mirrored. |
| `mlTriplanarNormal(tex, worldPos, normal, scale, sharpness, strength)` | `vec3` | Triplanar normal mapping with the whiteout blend (Ben Golus, "Normal Mapping for a Triplanar Shader", 2017). |
| `mlTriplanarSet(baseColor, normalMap, orm, worldPos, normal, scale, sharpness, normalStrength)` | `Surface` | A whole texture set by triplanar projection: 9 texture reads. |
| `mlTriplanarWeights(normal, sharpness)` | `vec3` | The three blend weights; they sum to 1. |
| `mlSlopeMask(normal, up, start, end)` | `float` | 1 on faces that point along `up`, 0 on steep faces: `smoothstep(start, end, dot(n, up))`. |
| `mlHeightMask(height, low, high)` | `float` | 0 below `low`, 1 above `high`, in world meters. |
| `mlHeightBlend(heightA, heightB, weightB, contrast)` | `float` | Sharpens a layer weight with two height maps, so the higher layer wins first: moss fills the cracks of rock before it covers the bumps. `contrast` is the blend band in height units. |
| `mlLayer(a, b, heightA, heightB, weightB, contrast)` | `Surface` | `mixSurface(a, b, mlHeightBlend(...))`. |
| `mlBlendNormalsRNM(base, detail)` | `vec3` | Reoriented normal mapping of two tangent normals (Barré-Brisebois and Hill, "Blending in Detail", 2012). |
| `mlBlendNormalsUDN(base, detail)` | `vec3` | Cheaper slope addition. |
| `mlDetailNormal(normal, detailMap, uv, tiling, strength, tangent, bitangent)` | `vec3` | A tiled detail normal map over a world normal. |
| `mlDetailWorldNormal(base, detail, geometric, strength)` | `vec3` | Adds a world-space detail normal's deviation from the geometric normal to `base`, for a finer second triplanar fetch. |

The surface builtins in every surface material are `pbr(baseColor, roughness,
metallic)`, `mixSurface(a, b, t)` and the `with*` setters; the interop page
lists them.

## Sculpted rock

[`examples/material-library/sculpted-rock.sel`](../../examples/material-library/sculpted-rock.sel)
shades a Blender-sculpted boulder with no clean UVs: triplanar rock with a
finer detail normal, moss on upward faces that fills the cracks first, sand
around the base and a wet band at the water line. It samples six textures,
the most a surface material may bind, and 21 texels per pixel.

```selena
let n = normalize(geo.worldNormal)
let rockOrm = mlTriplanar(rockORM, geo.worldPos, n, scale, 4.0)
let rockN = mlTriplanarNormal(rockNormal, geo.worldPos, n, scale, 4.0, 1.0)
let detailN = mlTriplanarNormal(rockNormal, geo.worldPos, n, scale * 6.0, 4.0, 0.5)
let rock = mlSurfaceFromSet(mlTriplanar(rockAlbedo, geo.worldPos, n, scale, 4.0), mlDetailWorldNormal(rockN, detailN, n, 0.6), rockOrm.rgb)

let mossC = mlTriplanar(mossAlbedo, geo.worldPos, n, scale * 2.0, 4.0)
let moss = withNormal(pbr(mossC.rgb, 0.9, 0.0), mlTriplanarNormal(mossNormal, geo.worldPos, n, scale * 2.0, 4.0, 0.7))
let mossy = mlLayer(rock, moss, rockOrm.a, mossC.a, mlSlopeMask(n, vec3f(0.0, 1.0, 0.0), 0.55, 0.85) * mossCover, 0.08)
```

## Limits

- `mlPBRSet`, `mlTangentToWorld` and `mlDetailNormal` need mesh tangents. A
  mesh without them passes zero vectors and the normal becomes undefined;
  export tangents from Blender, or use the triplanar helpers.
- Each `sample` inside a helper is a texture read: `mlTriplanar` costs three,
  `mlTriplanarSet` nine. Budget phones accordingly.
- `sample` needs uniform control flow in WGSL, so call these helpers outside
  `if` and `for` bodies.
