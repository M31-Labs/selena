# Engine inputs

A material reads values and textures the renderer owns by declaring them in a
`context { ... }` block. Selena checks each declaration against a typed
registry, so a wrong type is a compile error instead of a silent runtime zero.
The descriptor tells the host which fields and textures are engine inputs, and
which registry version the material was compiled against.

```selena
material EngineLit {
    context {
        sunDir   : vec3
        sunColor : vec3
        exposure : float
        tint     : vec3      // not in the registry: a host-custom context field
    }
    surface(geo) -> color {
        let n = normalize(geo.worldNormal)
        return sunColor * max(dot(n, sunDir), 0.0) * tint * exposure
    }
}
```

## Rules

- A `context` name that is in the registry must use the registered type,
  including the array length of an array input such as
  `lights : array<vec4, 32>`. Otherwise the compiler reports `SEL1007`.
- A name that is not in the registry stays a host-custom context field, as before.
- The descriptor marks a registered value `class: "context"` and
  `source: "engine"`. A material that declares any engine input also carries
  `engineVersion` (currently `selena.engine.v1`); a host should refuse a
  material whose version it does not implement. Materials with no engine input
  have no `engineVersion`, so their descriptors are unchanged.
- The registry is available in Go as `bindings.EngineInputs()`. Adding an entry
  keeps the version. Changing an entry's type or meaning, or removing one,
  bumps it.

## Values

| Name | Type | Space or encoding | Meaning |
|---|---|---|---|
| `cameraPos` | `vec3` | world, meters | Camera position. |
| `sunDir` | `vec3` | world, unit vector | Direction toward the key directional light. |
| `sunColor` | `vec3` | linear RGB | Key directional light color times intensity. |
| `ambient` | `vec3` | linear RGB | Ambient light color times intensity. |
| `fog` | `vec4` | linear RGB, 1/meters | Fog color in xyz and density per meter in w; density 0 disables fog. |
| `exposure` | `float` | linear scale | Scene exposure multiplier applied before tone mapping. |
| `time` | `float` | seconds | Scene clock. |
| `cameraProximity` | `float` | 0..1 | How close the camera is to the object, as the renderer defines it. |
| `environmentInfo` | `vec4` | x: image-based light active, y: intensity, z: rotation radians, w: top radiance mip | Image-based lighting state. |
| `skyBetaR` | `vec4` | physical sky block 0 | Rayleigh coefficients in xyz and sun intensity in w; the first vec4 of the physical sky parameters. |
| `skyBetaM` | `vec4` | physical sky block 1 | Mie coefficients in xyz and sun fade in w. |
| `skySun` | `vec4` | physical sky block 2 | Unit direction toward the sun in xyz and the cosine of the sun disk radius in w (above 1 hides the disk). |
| `skyG` | `float` | physical sky block 3 | Mie directional factor g. |
| `wind` | `vec4` | xy unit XZ direction, z m/s, w seconds | Scene wind: the direction it blows toward in xy (world x and z), speed in z and the wind clock in w; all zero when the scene has no wind. |
| `windGust` | `vec4` | x 0..1, y meters, z m/s, w Hz | Wind gusts: strength (speed swings by ±x), distance between gust fronts, speed of the fronts and flutter frequency. |

## Textures

Declared in the same `context` block, with a texture type (see
[Engine textures](#engine-textures)).

| Name | Type | Space or encoding | Meaning |
|---|---|---|---|
| `planarReflection` | `texture2d` | linear RGB in rgb, coverage in a | Planar reflection capture. |
| `opaqueColor` | `texture2d` | linear RGB | Opaque scene color captured before transparent surfaces. |
| `opaqueDepth` | `textureDepth2d` | depth 0..1 | Depth of the opaque scene; read with sampleDepth. |

## Engine textures

An engine texture is declared in `context` with its registered name and type.
The renderer binds it by name, so the author supplies no source for it. It
takes the next texture slot after the author's `param ... : texture2d` entries,
in declaration order, with the usual binding numbers.

```selena
material Water {
    context {
        planarReflection : texture2d
        opaqueColor      : texture2d
        opaqueDepth      : textureDepth2d
    }
    surface(geo) -> color {
        let uv = vec2f(0.5, 0.5)
        let seen = sample(opaqueColor, uv).rgb
        let reflected = sample(planarReflection, uv).rgb
        return mix(seen, reflected, clamp(sampleDepth(opaqueDepth, uv) * 4.0, 0.0, 1.0))
    }
}
```

- Colors are read with `sample` and `sampleLevel` (and, in `vertex()`,
  `sampleLevel` only). A depth texture is read with `sampleDepth(tex, uv)`,
  which returns a `float` from the base level. It uses no derivatives, so it
  works in `vertex()` and inside branches.
- The descriptor lists each one in `textures` with `engine: "<name>"`. A depth
  texture has `dimension: "depth2d"`: bind it with a depth view (WebGPU sample
  type `depth` or `unfilterable-float`), and a NEAREST sampler on WebGL.
  Binding coordinates follow the usual rules, including `vertexStage`.
- Engine textures are available in mesh materials (with or without `vertex()`).
  Points, post and feedback materials reject a texture in `context`. A
  `context` block can declare only registered engine textures; declare other
  textures as `param name : texture2d`.
- Depth textures are not allowed as `param`s.

### Support by target

| Target | Color engine textures | `textureDepth2d` |
|---|---|---|
| `wgsl` | yes | yes: `texture_depth_2d`, read with `textureLoad` (the sampler binding is declared and unused) |
| `gles` (WebGL 2, GLSL ES 3.00) | yes | yes: `highp sampler2D` over a depth texture, read with `textureLod(...).x` |
| `glsl` (WebGL 1, GLSL ES 1.00) | yes | yes, if the host enables `WEBGL_depth_texture` (listed in `requires.glExtensions`) and the device supports `highp` in fragment shaders; read with `texture2D(...).x` |
| `metal` | yes | yes: `depth2d<float>`, read with `sample(..., level(0))` |

`textureDepth2dArray` (the shadow maps) is supported the same way, one layer
per cascade, read with `sampleDepth(tex, uv, layer)`:

| Target | `textureDepth2dArray` |
|---|---|
| `wgsl` | `texture_depth_2d_array`, read with `textureLoad(tex, texel, layer, 0)` |
| `gles` (WebGL 2) | `highp sampler2DArray` over a depth array, read with `textureLod(tex, vec3(uv, float(layer)), 0.0).x` |
| `glsl` (WebGL 1) | no array textures: the artifact declares `highp sampler2D`, the host binds layer 0, and the layer argument is ignored; needs `WEBGL_depth_texture` |
| `metal` | `depth2d_array<float>`, read with `sample(s, uv, uint(layer), level(0))` |

The GLSL ES 1.00 vertex stage still needs vertex texture units; see
`requires.glVertexTextureUnits` in [compatibility.md](compatibility.md).

## Scene lighting

A mesh material reads the renderer's lights, directional shadow cascades and
fog through engine inputs, like any other: declare the names you use in
`context`. The [lighting module](material-library/lighting.md) reads them for
you; link it with `materiallib.Lighting` (`selena artifact -modules lighting`,
or `"modules": ["lighting"]` in GoSX's `build.selena`).

```selena
material SceneLit {
    context {
        cameraPos       : vec3
        cameraForward   : vec3
        ambient         : vec3
        fog             : vec4
        lightCount      : float
        lights          : array<vec4, 32>
        shadow0         : vec4
        shadow1         : vec4
        shadowSplits0   : vec4
        shadowSplits1   : vec4
        shadowMatrices0 : array<vec4, 16>
        shadowMatrices1 : array<vec4, 16>
        shadowMap0      : textureDepth2dArray
        shadowMap1      : textureDepth2dArray
    }
    param albedo : color = rgb(0.72, 0.64, 0.52)

    surface(geo) -> color {
        let n = normalize(geo.worldNormal)
        let s0 = mlShadow0(geo.worldPos)
        let s1 = mlShadow1(geo.worldPos)
        var lit = ambient * albedo
        for (var i = 0i; i < 8i; i = i + 1i) {
            lit = lit + albedo * mlLightDiffuse(i, n, geo.worldPos) * mlLightShadow(i, s0, s1)
        }
        return mlFog(lit, geo.worldPos)
    }
}
```

The full example, with specular, is
[`examples/scene-lit.sel`](../examples/scene-lit.sel).

### Values

| Name | Type | Space or encoding | Meaning |
|---|---|---|---|
| `cameraForward` | `vec3` | world, unit vector | Direction the camera looks. View depth, which picks a shadow cascade, is `dot(p - cameraPos, cameraForward)`. |
| `lightCount` | `float` | 0..8 | Number of lights packed into `lights`. |
| `lights` | `array<vec4, 32>` | 4 vec4 per light | Up to 8 scene lights; see the packing below. |
| `shadow0` | `vec4` | x cascades, y bias, z filter radius (uv), w light index | Shadow slot 0: cascade count (0 means the slot is empty), depth bias, the five-tap filter radius in shadow-map uv, and the index into `lights` of the casting light (-1 for none). |
| `shadow1` | `vec4` | as `shadow0` | Shadow slot 1. |
| `shadowSplits0` | `vec4` | view depth, meters | Far view depth of cascades 0, 1, 2 and 3 of slot 0. |
| `shadowSplits1` | `vec4` | view depth, meters | The same for slot 1. |
| `shadowMatrices0` | `array<vec4, 16>` | world to shadow uvz, column-major | Four columns per cascade: cascade `c` is elements `4c` to `4c+3`. |
| `shadowMatrices1` | `array<vec4, 16>` | as `shadowMatrices0` | The same for slot 1. |

`cameraPos`, `ambient` and `fog` (listed under [Values](#values)) complete the
block.

### Textures

| Name | Type | Space or encoding | Meaning |
|---|---|---|---|
| `shadowMap0` | `textureDepth2dArray` | depth 0..1, one layer per cascade | Slot 0 shadow depth; read with `sampleDepth(shadowMap0, uv, cascade)`. |
| `shadowMap1` | `textureDepth2dArray` | depth 0..1, one layer per cascade | Slot 1 shadow depth. |

### Light packing

Light `i` occupies `lights[4i]` to `lights[4i+3]`. Entries at or past
`lightCount` are ignored by the lighting module.

| Element | xyz | w |
|---|---|---|
| `lights[4i]` | position, world meters | kind: 0 ambient, 1 directional, 2 point, 3 spot, 4 hemisphere |
| `lights[4i+1]` | direction the light travels, world (directional and spot) | range in meters, 0 for unlimited |
| `lights[4i+2]` | color times intensity, linear RGB (sky color for hemisphere) | decay exponent (used when range is 0) |
| `lights[4i+3]` | spot: x cos(outer cone), y cos(inner cone); hemisphere: ground color times intensity | shadow slot: 0, 1, or -1 for none |

Point and spot falloff, the spot cone, the hemisphere blend and the flat
ambient term match the GoSX standard material (`materiallib` tests compare
them with its formulas).

### Shadow cascades

Each shadow slot holds up to 4 cascades of one directional light. The lighting
module, or your own code, does this per fragment:

1. Pick cascade `c`: the first `c` whose split `shadowSplitsN[c]` lies beyond
   the view depth, among the `shadowN.x` cascades.
2. Project: `s = M[4c] * p.x + M[4c+1] * p.y + M[4c+2] * p.z + M[4c+3]`, then
   `uvz = s.xyz / s.w`. The matrices already include the renderer's
   clip-to-texture mapping, so `uvz.xy` is a texture coordinate on every
   backend and `uvz.z` is comparable with the stored depth.
3. Compare: the point is lit when `uvz.z - shadowN.y <= sampleDepth(shadowMapN, uvz.xy, c)`.
   A point outside the map (`uvz.xy` outside 0..1, or `uvz.z > 1`) is lit.

The cascade count is data, not a compile-time constant: a renderer with one
map per slot sets `shadowN.x = 1` and binds a one-layer array, and the same
artifact uses more cascades when the renderer provides them.
