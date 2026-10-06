# Procedural materials

Link `materiallib.Procedural` to share texture-free wood and marble patterns,
noise and pixel-footprint filtering across all four shader targets.

```selena
let grain = mlWoodGrainAA(geo.uv * vec2f(3.0, 5.0), 28.0, 8.0)
let vein = mlMarbleVeinAA(geo.uv * 4.0, 9.0, 12.0)
```

| Helpers | Contract |
| --- | --- |
| `mlHash11(float)`, `mlHash21(vec2)`, `mlHash22(vec2)` | Float hashes in [0, 1); non-cryptographic |
| `mlNoiseGradient2(vec2)` | Unit lattice gradient |
| `mlNoiseFade2(vec2)` | Quintic interpolation weights |
| `mlValueNoise2(vec2)` | Value noise in [0, 1] |
| `mlGradientNoise2(vec2)`, `mlSimplexNoise2(vec2)` | Signed 2D noise, approximately [-1, 1] |
| `mlValueNoise2Deriv`, `mlGradientNoise2Deriv`, `mlSimplexNoise2Deriv` | `vec3(value, d/dx, d/dy)`, one `vec2` coordinate argument |
| `mlSimplexCorner2(vec2, vec2)` | Simplex kernel contribution with gradient |
| `mlNoiseFilter(float footprint)` | Octave attenuation; 1 below 0.25, 0 above 0.75 |
| `mlValueNoise2Filtered`, `mlGradientNoise2Filtered`, `mlSimplexNoise2Filtered` | `(vec2 p, float footprint)`; fade toward the noise's mean |
| `mlFBm4`, `mlFBm4AA` | Four octaves at 1×/2×/4×/8×, amplitudes 0.5/0.25/0.125/0.0625, normalized by 15/16 |
| `mlFBm4Filtered`, `mlFBm4DerivFiltered` | `(vec2 p, float footprint)`; scalar value or value plus analytic gradient |
| `mlFilteredSinWidth` | `(float phase, float footprint)`; Gaussian attenuation of a sinusoid |
| `mlFilteredSin` | `(float phase)`; fragment `fwidth` wrapper |
| `mlWoodGrainFiltered`, `mlMarbleVeinFiltered` | `(vec2 p, float frequency, float warp, float footprint)`; scalar [0, 1] motif |
| `mlWoodGrainAA`, `mlMarbleVeinAA` | `(vec2 p, float frequency, float warp)`; fragment `fwidth` wrapper |

The `AA` wrappers measure `length(fwidth(p))` after coordinate scaling. Call
them in uniform fragment control flow. The compiler records the existing
`OES_standard_derivatives` host requirement for WebGL1. GLSL ES 3, WGSL and Metal
have derivatives in core. Explicit-footprint helpers can also run without
screen derivatives, including a vertex stage. Supply a conservative pixel width
in the helper's coordinate units; do not pass the unscaled UV footprint.

Filtered fBm fades unresolved octaves to zero without renormalizing the visible
octaves. Value noise fades to 0.5. Wood estimates its warped phase gradient
analytically and fades unresolved rings to 0.5. Marble fades the narrow vein
motif to its analytic mean, 0.196380615234375. This avoids turning minified marble
into a uniformly vein-free surface. These are conservative attenuation helpers,
not exact integration of a warped pixel footprint. Warps with discontinuities or
poorly estimated explicit footprints can still alias.

Hashes use float arithmetic and avoid large sine multipliers. They are stable
for a given backend, but different GPU compilers can round differently. Keep
coordinates reasonably small (approximately |p| < 10,000), and do not rely on
bit-identical noise across devices. The quintic value/gradient noise and compact
simplex kernel have analytic first derivatives. Tests compare these derivatives
with finite differences, including negative coordinates and lattice boundaries.

The example's wood, marble and fBm are all evaluated in one gallery shader.
Its source sizes are 48,208 bytes WGSL, 48,028 GLSL, 49,478 Metal and 48,004 GLES.
The optimized fragment SPIR-V has 1,743 function instructions, counted with the
same Naga/spirv-opt method described in [BRDF helpers](brdf.md). This is a compiler
proxy, not a GPU hardware instruction count.
Production materials that call only one pattern emit only that pattern's
dependency chain. Prefer explicit footprints and a single noise layer for phone
materials when four octaves exceed the host's measured budget. Unused modules
still add no shader bytes or host bindings.

```sh
GOWORK=off go run ./examples/material-library/preview -modules procedural \
  -source examples/material-library/procedural.sel -out material-preview.html
GOWORK=off go test ./materiallib
```

Every helper and the example pass emission on WGSL/GLSL/Metal/GLES, Naga WGSL
validation, and glslang validation of both GL dialects. Metal native compilation
requires an Apple toolchain. Replace local `softBand` helpers with
`mlFilteredSin`; use wood/marble helpers when domain warping adds useful detail.
Keep low-cost material tiers selected by the host.

![Procedural wood, marble and noise in software WebGL2](procedural.png)

Capture: the example in this change, 960 × 1120 viewport, software WebGL2.
WebGPU browser rendering remains unverified because device creation fails in the
local graphics driver; Naga validates the WGSL.

For the simplex kernel and analytic derivative principle, see the primary
[simplex noise paper](https://jcgt.org/published/0011/01/02/paper-lowres.pdf).
This implementation uses a nonperiodic triangular lattice; it does not implement
that paper's periodic or rotating-gradient API.
