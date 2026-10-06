# BRDF helpers

`materiallib.BRDF` supplies direct-light building blocks for varnished wood,
cloth, brushed metal and ivory in a tabletop game scene. Inputs are linear-light
colors. Directions and half vectors must be normalized. Results are BRDF values:
multiply by incident radiance and `max(N.L, 0)` once at the lighting call site.
`mlWrapDiffuse` and `mlThinTransmission` already include their angular factor.

```go
result, err := materiallib.Compile(source, selena.CompileOptions{}, materiallib.BRDF)
```

For GoSX, use `materiallib.Functions(materiallib.BRDF)` with
`adapter/gosx.Material(material, functions)`, or call `materiallib.Link` before
`selena.CompileProgram`. `Link` rejects duplicate function names. It keeps the
application's source spans, params and context uniforms. Unused helpers add zero
shader bytes and do not change binding descriptors.

| Function | Inputs | Result |
| --- | --- | --- |
| `mlPow5` | `float x` | `float x⁵` |
| `mlFresnelSchlick` | `float cosTheta, vec3 f0` | `vec3` Fresnel |
| `mlGGXD` | `float nh, roughness` | GGX normal distribution |
| `mlGGXVisibility` | `float nv, nl, roughness` | Correlated Smith visibility, including `1/(4 nv nl)` |
| `mlGGXEnergyCompensation` | `vec3 f0, float singleScatterEnergy` | `vec3` multiplier |
| `mlGGX` | `float nv, nl, nh, vh, roughness, vec3 f0, float singleScatterEnergy` | Compensated GGX BRDF |
| `mlClearCoat` | `vec3 base, float nv, nl, nh, vh, roughness, weight` | Attenuated base BRDF plus dielectric coat |
| `mlCharlieD` | `float nh, roughness` | Charlie distribution |
| `mlSheenCharlie` | `float nv, nl, nh, roughness, vec3 tint` | Charlie sheen with cloth visibility |
| `mlAnisotropicGGXD` | `vec3 h, float alphaT, alphaB` | Anisotropic GGX distribution |
| `mlAnisotropicGGX` | `vec3 v, l, h, float alphaT, alphaB, vec3 f0, float singleScatterEnergy` | Compensated anisotropic BRDF |
| `mlWrapDiffuse` | `float nl, wrap` | Wrapped angular diffuse response |
| `mlThinTransmission` | `float nl, thickness, strength` | Cheap back-light response |

`nv`, `nl`, `nh` and `vh` are the corresponding direction dot products. GGX
roughness is perceptual: alpha is roughness squared, floored at 0.0025. The
anisotropic helpers instead accept alpha directly. Their vectors are in an
orthonormal tangent frame: x along brushing, y across it, z along the normal.
Use equal alphas for isotropic GGX. Swap them to rotate the lobe by 90 degrees.

Energy compensation uses `1 + f0 * (1 / E - 1)`, where E is the single-scatter
directional energy from the host's integrated BRDF lookup or an offline estimate
for the same roughness/view angle. E is clamped to [0.05, 1] to bound gain. Supply
1 to disable compensation. This library does not invent an environment-light
lookup, integrate image-based lighting, or claim energy conservation for arbitrary
layer combinations. The host must account for the remaining diffuse energy.

Clear coat has dielectric F0 = 0.04. It attenuates the base on both incident and
outgoing paths before adding the coat. Weight is clamped to [0, 1]. Charlie
roughness is floored at 0.07. Scale the base diffuse when adding sheen; adding
unbounded sheen to an already energy-complete base can exceed incoming energy.
Wrap and thin transmission are stylized phone-friendly terms rather than a
subsurface transport model. Treat thickness as a normalized artistic control.

The library preserves each called function's arguments and locals as call-site
`let` bindings. Arguments and returns are type checked. This avoids repeated
evaluation from expression substitution and emits no runtime function calls.
Calls remain inside branch and loop scopes, including loop conditions and post
updates. Derivative helpers, when used, must execute in uniform fragment control
flow. Core compilation without linked modules retains legacy output.

Run the example and open the resulting page through a local HTTP server:

```sh
GOWORK=off go run ./examples/material-library/preview -modules brdf \
  -source examples/material-library/brdf.sel -out material-preview.html
```

The preview draws varnish, felt sheen and brushed zinc with both browser targets.
Run `GOWORK=off go test ./materiallib` for every-helper WGSL/Naga and
GLSL/GLES/glslang checks, numerical distribution normalization, isotropic limits,
grazing guards, descriptor compatibility and call-site diagnostics. Metal emission
is covered; native MSL compilation requires an Apple toolchain.

The math follows the distribution and visibility definitions in
[Physically Based Rendering](https://www.pbr-book.org/4ed/Reflection_Models/Roughness_Using_Microfacet_Theory)
and the compensation, cloth and coating models in
[Filament's material model](https://github.com/google/filament/blob/main/docs/Filament.md.html).

For the three-material example, emitted source sizes are 7,153 bytes WGSL,
6,996 GLSL (both stages), 7,338 Metal and 7,020 GLES (both stages). The optimized
fragment SPIR-V has 199 function instructions, versus 189 with legacy expression
substitution; WGSL source grows from 5,716 to 7,153 bytes. Retained locals bound
repeated-expression growth in larger helpers, but are not a blanket size win.
These counts use Naga followed by `spirv-opt -O`, counting function instructions
except function declarations, parameters and labels. They are a portable compiler
proxy, not native GPU instruction counts. The existing CLI executable's size is
unchanged; it does not import the opt-in package. A warm library compile of the
example takes about 8.3 ms in a 20-iteration local sample; this is not a runtime
frame-time measurement.

Local shader helpers for anisotropic lobes and coat Fresnel can switch to these
functions. Keep the host's light, camera, tone mapping and environment wiring:
Selena still emits custom materials rather than extending GoSX's standard-lit
renderer. No GoSX descriptor migration is required.
