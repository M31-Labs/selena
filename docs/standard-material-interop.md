# Standard Material Interop

A Selena material reaches a host's PBR renderer in one of two ways:

| Kind | Source | What the host does | Lighting |
|---|---|---|---|
| Full material | `surface(geo) -> color` | Runs Selena's complete vertex and fragment shaders. | The material's own. The [scene lighting block](engine-inputs.md#scene-lighting) and the [lighting module](material-library/lighting.md) give it every light, shadow cascade and fog. |
| Surface material | `surface(geo) -> Surface` | Splices Selena's fragment hook into its own standard material shader. | The host's standard lighting: the same lights, shadows, image-based lighting, fog, exposure and tone mapping as its StandardMaterial. |

Use a surface material when a custom look should still be lit like every
other object: sculpted rock, layered terrain, painted props. Use a full
material for unlit effects, stylized lighting, or anything the standard
model cannot express.

## Surface materials

```selena
material WetRock {
    param albedo : texture2d
    param wet : float = 0.0

    surface(geo) -> Surface {
        let s = pbr(sample(albedo, geo.uv).rgb, 0.85, 0.0)
        return withRoughness(s, mix(s.roughness, 0.2, wet))
    }
}
```

`surface(geo) -> Surface` makes the material surface-kind; `material X kind
surface` says the same explicitly. Selena splits every `Surface` value into one
local per field during lowering, so the record costs nothing at run time.

### The Surface record

| Field | Type | Default from `pbr` | Meaning |
|---|---|---|---|
| `baseColor` | `vec3` | argument | Linear base color (albedo, or F0 for metals). |
| `alpha` | `float` | 1 | Coverage. Multiplies the material opacity; the host's alpha cutoff and blend mode apply. |
| `normal` | `vec3` | `normalize(geo.worldNormal)` | Shading normal, unit length, world space. |
| `roughness` | `float` | argument | Perceptual roughness 0..1. The host clamps it to at least 0.04. |
| `metallic` | `float` | argument | 0 dielectric, 1 metal. |
| `occlusion` | `float` | 1 | Ambient occlusion; scales image and ambient light only. |
| `emissive` | `vec3` | 0 | Linear emitted radiance, added after lighting. |
| `clearcoat` | `float` | 0 | Clear-coat layer weight 0..1. |
| `clearcoatRoughness` | `float` | 0.03 | Coat roughness; the host clamps it to at least 0.04. |
| `sheen` | `float` | 0 | Grazing sheen weight, tinted by the base color. |
| `transmission` | `float` | 0 | Thin transmission weight for dielectrics. |

### Building a Surface

| Call | Result |
|---|---|
| `pbr(baseColor, roughness, metallic)` | A Surface with the defaults above. |
| `withBaseColor(s, c)`, `withAlpha(s, a)`, `withNormal(s, n)`, `withRoughness(s, r)`, `withMetallic(s, m)`, `withOcclusion(s, ao)`, `withEmissive(s, e)`, `withSheen(s, k)`, `withTransmission(s, t)` | `s` with one field replaced. |
| `withClearcoat(s, amount, roughness)` | `s` with the coat weight and roughness replaced. |
| `mixSurface(a, b, t)` | Field-wise `mix`; the normal is renormalized. |
| `s.roughness`, `s.normal`, ... | Reads a field. |
| `cond ? a : b` | Picks a whole Surface. |

A Surface can be bound with `let` or `var`, reassigned, passed to and
returned from functions (`fn wetten(s: Surface, k: float) -> Surface`), and
returned at the end of the surface body. It cannot be used as a plain value:
`s * 2.0` is an error.

### Geometry

| Field | Type | Meaning |
|---|---|---|
| `geo.worldPos` | `vec3` | World position. |
| `geo.worldNormal` | `vec3` | Interpolated geometric normal, unit length. |
| `geo.uv` | `vec2` | First UV set. |
| `geo.tangent`, `geo.bitangent` | `vec3` | Mesh tangent frame, world space, not normalized; zero when the mesh has no tangents. |
| `geo.viewDir` | `vec3` | Unit vector from the surface toward the camera. |
| `geo.color` | `vec4` | Instance or vertex color. The host already multiplies the base color by its rgb. |

### Rules

- Params and `context` values (for example `time`) work as in a full
  material. Engine textures (`shadowMap0`, `opaqueColor`, ...) and
  `textureCube` params are rejected: the host already does shadows and
  reflections.
- At most `bindings.SurfaceMaxTextures` (6) `texture2d` params. Hosts bind
  them in their standard material texture slots.
- No `vertex()`, varyings, statefields or `specular` companion; the host's
  standard vertex stage runs and it computes the specular output itself.
- No early `return`; assign a `var` Surface and return it at the end.
  `discard` works.
- Targets: `wgsl` and `gles`. A surface material compiles with no target list
  to exactly those two; asking for `glsl` (WebGL 1) or `metal` is an error.

### The hook ABI

The layout of a surface material has `kind: "surface"` and a `surface`
descriptor (`bindings.SurfaceHook`, ABI `selena.surface.v1`):

```json
"surface": {
  "abi": "selena.surface.v1",
  "function": "selenaSurface",
  "textureNames": ["selenaSurfaceTex0"],
  "glUniformPrefix": "selena_"
}
```

- The WGSL artifact's `source` and the GLES artifact's `fragment` each declare
  the `SelenaSurfaceGeo` and `SelenaSurface` structs, the params, and
  `selenaSurface(selenaGeo: SelenaSurfaceGeo) -> SelenaSurface`. They are
  fragments to splice into a host fragment shader, not modules.
- WGSL params are one struct, `selenaParams`, at `@group(2) @binding(0)`
  (`layout.wgsl`), packed by `layout.uniformBlock` exactly as for a full
  material. GLES params are plain uniforms named `glUniformPrefix + name`.
- Texture `i` (`layout.textures[i]`) is referenced as `textureNames[i]`, with
  the WGSL sampler `textureNames[i] + "Sampler"`. The hook declares neither;
  the host declares them or renames them to its own bindings.
- `wgsl.SurfaceHarness` and `gles.SurfaceHarness` wrap a hook in a complete
  shader for validation; Selena's tests run every hook through naga and
  glslangValidator that way.

### GoSX

`scene.CompileSelenaMaterial` and the artifact loaders return a
`CustomMaterial` whose `FragmentGLSL` and `FragmentWGSL` are the two hooks.
Both GoSX renderers replace the material-resolution block of their standard
PBR fragment with a call to the hook, so a surface material and a
StandardMaterial with the same inputs render the same pixels. The GoSX side is
documented in `docs/scene3d-selena.md` in the GoSX repository.

## Full materials

A full material emits complete shaders. The GoSX adapter returns a
`CustomMaterial` with GLSL and WGSL vertex and fragment programs, descriptor
defaults flow into the material's uniforms, and the material replaces the
whole shader. Standard material fields such as roughness maps, image-based
lighting and tone mapping are not applied to it.

## Non-goals

- No arbitrary lighting loops in a surface material: the host owns lighting.
- No separate Selena PBR implementation for surface materials.
- No implicit use of a host's standard material texture maps from Selena
  source; declare `texture2d` params.
- No backend-specific escape hatches in `.sel` source.
