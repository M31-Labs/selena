# Physical sky

`materiallib.Sky` supplies `physicalSky(dir, betaR, betaM, sun, g)`, the
daylight sky GoSX draws as `gosxPhysicalSky`, written once in Selena. Rayleigh
and Mie single scattering after Preetham, Shirley and Smits (1999), in the
real-time form of Hoffman and Preetham (2002), with a sun disk.

```selena
material Sea {
    context {
        skyBetaR : vec4
        skyBetaM : vec4
        skySun   : vec4
        skyG     : float
    }
    surface(geo) -> color {
        let d = normalize(geo.worldNormal)
        return physicalSky(vec3f(d.x, abs(d.y), d.z), skyBetaR, skyBetaM, skySun, skyG)
    }
}
```

```go
result, err := materiallib.Compile(source, selena.CompileOptions{}, materiallib.Sky)
```

For GoSX, use `materiallib.Functions(materiallib.Sky)` with
`adapter/gosx.Material`, or call `materiallib.Link` first.

## Arguments

The four parameter arguments are the engine inputs `skyBetaR`, `skyBetaM`,
`skySun` and `skyG` (see [engine-inputs.md](../engine-inputs.md)), the same
sixteen floats GoSX packs for its sky pass.

| Argument | Meaning |
|---|---|
| `dir` | Unit view direction in world space. |
| `betaR` | Rayleigh coefficients in `xyz`, sun intensity in `w`. |
| `betaM` | Mie coefficients in `xyz`, sun fade in `w`. |
| `sun` | Unit direction toward the sun in `xyz`; cosine of the sun disk radius in `w` (above 1 hides the disk). |
| `g` | Mie directional factor. |

The result is display-referred linear radiance for an ACES-style tone mapper at
exposure 0.5 to 1. To see the underside as GoSX's ocean does, pass the direction
with `y` folded to `abs(y)`.

## Precision

The extinction terms span many orders of magnitude. WGSL, GLSL ES 3.00 and Metal
compute in 32-bit floats. The GLSL ES 1.00 fragment shader Selena emits declares
`precision mediump float`, which loses accuracy in this function on devices where
`mediump` is 16 bits; prefer the `gles` artifact on WebGL 2.

## Tests

`GOWORK=off go test ./materiallib` evaluates the compiled IR at nine directions
under two skies and compares it with GoSX's CPU model
(`scene.Sky.PhysicalRadiance`); agreement is better than 1e-5 relative.
`go test ./validate` runs the emitted shaders through naga and glslangValidator.
