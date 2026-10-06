# Material library

`materiallib` adds 73 opt-in Selena functions for a tabletop game scene.
One authored material still compiles to WGSL, GLSL, Metal and GLES plus the
existing `selena.descriptor.v1` host layout.

```go
result, err := materiallib.Compile(source, selena.CompileOptions{},
    materiallib.BRDF, materiallib.Procedural, materiallib.Shapes,
    materiallib.Normals, materiallib.Effects)
```

Use `Functions` to append declarations to an authored program for the GoSX
adapter. Use `Link` before `selena.CompileProgram` when selecting a material or
resolving inheritance. Duplicate function names are errors. Only called helpers
are emitted; requesting an unused module changes neither source nor descriptor.
Library functions retain typed arguments and local bindings to bound expression
growth. Standard user functions keep their existing substitution behavior.

| Module | Features and examples |
| --- | --- |
| [BRDF](brdf.md) | Compensated GGX, coat, Charlie sheen, anisotropy, wrap and transmission |
| [Procedural](procedural.md) | Hashes, value/gradient/simplex noise, analytic gradients, filtered fBm, wood and marble |
| [Shapes](shapes.md) | AA distances, regular/arbitrary polygons, stars, stripes, saltires and booleans |
| [Normals](normals-effects.md) | Engraved wells, tangent/bump normals, bounded parallax and phone bypass |
| [Effects](normals-effects.md) | Finite ripple, dust and flash from event uniforms; loads Procedural |

## Integration boundary

The library replaces shared shader math; it does not supply GoSX's lighting,
environment maps, shadows, tone mapping or native program transport. The current
GoSX custom material API consumes full GLSL/WGSL vertex/fragment programs. A
renderer-owned standard-lit surface hook must be added in GoSX before Selena can
override PBR properties inside that pipeline. That hook needs a typed output
contract for base color, roughness, metalness, normal, coat, sheen and tangent
anisotropy, plus per-target renderer insertion points. Emitting standalone
custom shaders cannot implement that contract by itself.

Until those hooks exist, keep explicit camera/light inputs and standard material
fallbacks in the consumer. Native custom rendering also needs GoSX transport and
renderer support for the Metal/GLES artifacts; compiling them is not evidence
of native execution. The library keeps the binding schema and language versions
unchanged. See [standard material interop](../standard-material-interop.md).

## Consumer migration

| Existing local math | Shared replacement |
| --- | --- |
| Filtered sine bands | `mlFilteredSin`, or explicit-width `mlFilteredSinWidth` |
| Anisotropic lobe and coat Fresnel | `mlAnisotropicGGX`, `mlFresnelSchlick`, `mlClearCoat`; project vectors into the tangent frame and multiply BRDF by N.L once |
| Felt grazing sheen | `mlSheenCharlie`; budget the base diffuse energy |
| Procedural wood and marble | `mlWoodGrainAA` / `mlMarbleVeinAA`, or explicit-footprint variants |
| Flag edge masks, triangles and folded stars | Shape distances followed by `mlSDFMask`; use padded vec4 polygon arrays with `.xy` |
| Pip height and derivative normal | `mlPipWellHeight` plus `mlBumpNormal`, or analytic `mlPipWellNormalAA` |
| Single-height UV shift | `mlParallaxUV`; phone source omits the height read and uses `mlParallaxUVPhone` |
| Impact/pass envelopes, rings and dust | `mlEventEnvelope`, `mlRadialRippleAA`, `mlDustAA`, `mlImpactFlashAA`; expire overlay draws in the host |

Boolean parity can use `true`, `false` and boolean equality/inequality. Remove
expanded logical-XOR shims. `mod` now has floor semantics on Metal too, so custom
negative-angle modulo expansions can return to the builtin. Standard-lit and
native-transport shims remain until the corresponding GoSX hooks land.

## Validation and cost

`GOWORK=off go test ./...` passes. Every helper is compiled with dynamic inputs
for all four emit targets. Naga validates WGSL; glslang validates both GLSL and
GLES stages. Tests also cover numerical BRDF normalization, analytic gradients,
SDF signs/parity, event boundaries, type diagnostics, GoSX adapter output and
unchanged descriptors/source for unused modules. Hash/noise statistics evaluate
the emitted expressions of all four targets in float32, nearest-rounded mediump
and truncated mediump. The new lattice hash retains 256 output levels and four
zeros per 1,024-cell sample in every model. Software WebGL1 and WebGL2 hash-grid
readbacks have mean 0.498055 and variance 0.083007 after 8-bit framebuffer
quantization. See [precision limits and captures](procedural.md).
Metal native compilation needs
an Apple toolchain. Software WebGL1/WebGL2 captures cover the running examples, small
render targets and effect expiry. Local WebGPU device creation fails before
shader execution, so browser WebGPU appearance is not verified.

| Example | WGSL bytes | GLSL bytes | Metal bytes | GLES bytes | Optimized fragment instructions |
| --- | ---: | ---: | ---: | ---: | ---: |
| BRDF gallery | 7,153 | 6,996 | 7,338 | 7,020 | 199 |
| Procedural gallery | 104,448 | 105,900 | 107,350 | 105,876 | 4,239 |
| Shape gallery | 10,774 | 10,575 | 11,034 | 10,551 | 257 |
| Engraved wells | 4,078 | 3,903 | 4,164 | 3,879 | 87 |
| Table events | 12,718 | 12,842 | 13,087 | 12,818 | 429 |

GL source totals include both stages. Instruction counts are a portable proxy:
emit the example WGSL with the preview's `-artifacts` flag, compile fragmentMain
with Naga, optimize SPIR-V with `spirv-opt -O`, then count instructions inside
functions except OpFunction, OpFunctionEnd, OpFunctionParameter and OpLabel.
They are not hardware ISA counts or frame-time measurements.

```sh
naga --entry-point fragmentMain --shader-stage frag material.wgsl fragment.spv
spirv-opt -O fragment.spv -o fragment.opt.spv
spirv-dis fragment.opt.spv -o fragment.spvasm
```

The retained-local BRDF example grows from 5,716 to 7,153 WGSL bytes and from
189 to 199 optimized instructions versus legacy expression substitution.
Retained locals control repeated-expression growth but do not guarantee a size
reduction. The procedural gallery evaluates three patterns; individual production
materials emit only their chosen dependency chain. The mediump hash fix adds
56,240 WGSL bytes / 2,496 optimized instructions to that gallery, and 4,567 bytes /
208 instructions to the event example's dust dependency. Standalone wood and
marble AA probes stay below 36 KB across targets, enforced by a 64-KiB regression
budget. Keep cheaper phone variants
when noise octaves exceed the scene's measured GPU budget.

For reproducible host compilation, use `GOWORK=off go build -trimpath
-buildvcs=false`. Compared with base commit `905d253`, the core CLI
grows from 13,688,358 to 13,699,363 bytes (+11,005 bytes). A minimal compile
consumer grows from 13,178,090 to 13,231,715 bytes when linking all modules
(+53,625 bytes, including 20,334 bytes of bundled Selena source). The hash fix
adds 719 embedded source bytes and 4,104 bytes to that sample binary, including
linker alignment; the core CLI size is unchanged by this fix. These are local
Linux/amd64 samples. An unused library costs zero shader instructions and zero
new host bindings; importing the Go package still includes its embedded sources.
