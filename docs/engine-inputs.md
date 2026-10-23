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

- A `context` name that is in the registry must use the registered type, and
  cannot be an array. Otherwise the compiler reports `SEL1007`.
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

The GLSL ES 1.00 vertex stage still needs vertex texture units; see
`requires.glVertexTextureUnits` in [compatibility.md](compatibility.md).
