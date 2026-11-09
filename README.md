# selena

**One shader source → every surface.** Selena is the shader & material authoring
language for the GoSX ecosystem. You describe a material *once*; Selena emits the
right shader for each rendering target — so a custom look renders identically in
the browser, in a GoSX desktop window, on iOS, and on Android, with no
per-backend shader work.

> Named because she is the one who makes the light behave.

## Why

GoSX scenes are authored once (Go/JSX → `scene.IR`) and already render everywhere
through that one contract. There are two material tiers today:

- **Standard / lit materials** are *structured props* (color, roughness, metalness,
  maps). Every target renders them with its own built-in shader, so they already
  work on WebGL, WebGPU, SceneKit, and GLES with zero extra work. This tier is
  seamless.
- **Custom shaders** fall off a cliff: you hand-write **GLSL *and* WGSL** (and
  mobile gets nothing), and GoSX's WebGPU *honesty gate* then degrades any backend
  you didn't hand-author. The moment you want a look beyond the standard material,
  the cross-target story breaks and you're maintaining four shader dialects by hand.

Selena closes that cliff. It extends the seamless property of the standard
material to *custom* looks: author in one high-level language, lower to a neutral
shader IR, and let target emitters produce WGSL / GLSL / Metal / GLSL-ES.

## Pipeline

```
  material.sel                          (author once — grammargen-defined DSL)
      │  parse  (gotreesitter / grammargen, the same engine behind .gsx)
      ▼
  Selena AST
      │  lower
      ▼
  Selena IR        ← the neutral, typed shader graph (the heart of the project)
      │
      ├── emit/wgsl  → WGSL        ┐ browser + GoSX desktop (Chromium / WebView2)
      ├── emit/glsl  → GLSL        ┘   via 16a-scene-webgpu.js / 16-scene-webgl.js
      ├── emit/metal → Metal (MSL) ┐ mobile via gosx-native
      └── emit/gles  → GLSL ES     ┘   SceneKit (iOS) / GLSurfaceView (Android)
```

## Target map

The shipping surfaces collapse to **two shader worlds**:

| Surface | Shell | Renderer | Selena emits |
|---|---|---|---|
| Browser | — | JS runtime (`16a` WebGPU / `16` WebGL) | **WGSL + GLSL** |
| Desktop | GoSX `desktop/` (Win32 + WebView2, no cgo) | *same JS runtime in Chromium* | **WGSL + GLSL** (shared with browser) |
| Mobile | `gosx-native` (SwiftUI / Compose) | SceneKit (iOS) / GLES (Android) | **Metal + GLSL-ES** |

Because browser and desktop are both Chromium, emitting WGSL alongside GLSL lights
up WebGPU on *both* at once — which is exactly how "reach WebGPU the same way we
reach WebGL" stops being a feature chase and falls out of the compiler.

## Try it

```sh
go test ./...
go run ./cmd/selena check examples/textured.sel
go run ./cmd/selena inspect examples/tinted.sel Tinted
go run ./cmd/selena inspect examples/defaults.sel
go run ./cmd/selena emit wgsl examples/directional-diffuse.sel
go run ./cmd/selena artifact -o - -targets wgsl,gles examples/tinted.sel
go run ./cmd/selena artifact -modules lighting examples/scene-lit.sel
go run ./cmd/selena demo /tmp/selena-textured.html textured
go run ./cmd/selena demo /tmp/selena-defaults.html defaults
```

Open the generated demo HTML in Chrome to compare the WGSL/WebGPU path with the
GLSL/WebGL path from the same `.sel` material.

See [docs/language-guide.md](docs/language-guide.md) for copy-pasteable material
patterns, supported types, defaults, host packing, and diagnostics. See
[docs/compatibility.md](docs/compatibility.md) for the current backend and host
descriptor contract, and
[docs/standard-material-interop.md](docs/standard-material-interop.md) for the
PBR/standard-material boundary.

The opt-in [`materiallib` package](docs/material-library/README.md) provides typed
BRDF, procedural, shape, normal and effect helpers authored in Selena. Use its
functions with the GoSX adapter or compile a material with `materiallib.Compile`. Unused helpers add no
shader source and the host binding descriptor stays unchanged.

The `lighting` module lets a mesh material read the scene's lights, cascaded
directional shadows and fog through the renderer's
[scene lighting block](docs/engine-inputs.md#scene-lighting); see
[lighting.md](docs/material-library/lighting.md) and `examples/scene-lit.sel`.

Mesh materials can optionally expose a linear `specular(geo) -> color` companion.
`CompileOptions{SpecularMRT: true}` adds typed color/specular MRT artifacts while
keeping the ordinary artifacts intact; omitted companions write zero. See the
[output and host contract](docs/compatibility.md#optional-linear-specular-output-mrt).

## Use as a library

```go
src, err := os.ReadFile("examples/textured.sel")
if err != nil {
    return err
}
res, err := selena.Compile(src, selena.CompileOptions{})
if err != nil {
    return err
}
wgsl, _ := res.Artifact(selena.TargetWGSL)
fmt.Println(res.Layout.UniformBlock.Size, len(wgsl.Source))
```

`CompileOptions{}` emits WGSL, GLSL, Metal, and GLES in a deterministic order.
Set `Material` to choose a named material from a file, or pass
`Targets: []selena.Target{}` to parse and lower without emitting shader source.
Use `bindings.PackUniforms(res.Layout, values)` to fill the generated std140
uniform block without hand-packing `vec3` tails or `mat3` column strides.
`bindings.PackUniformsWithDefaults` also fills omitted uniform fields from
descriptor defaults declared in `.sel`.
The compile-result types (`Target`, `CompileOptions`, `FragmentArtifact`,
`Artifact` and `Result`), the `Target` constants and the `AllTargets` function
live in the `result` package, which imports only `bindings`, `hir` and `ir`. A
program that only reads or passes along compile output, such as a browser WASM
module that loads artifacts compiled ahead of time, can import
`m31labs.dev/selena/result` instead of the root package and so avoid linking
the parser, the tree-sitter grammar, the lowerer and the emitters. The root
package re-exports every name in `result` (`selena.Result` is `result.Result`),
so existing code compiles unchanged and values pass between the two packages
without conversion. A test in `result/` fails if its import closure reaches the
compiler.
Compile failures that can be tied to source return `*selena.CompileError` with
diagnostic codes and 1-based line/column ranges. The CLI renders those ranges
as annotated snippets with fix-oriented hints, and common syntax errors include
expected-token context.
Authored params and locals are rejected before emission when they collide with
shader keywords, generated symbols, or Selena stdlib builtins.
Mesh and general materials may author a full `vertex() -> vec4` stage — the
author computes the clip-space position and writes any declared varyings, and
`vertex(geo) -> vec4` reads the raw `position`/`normal`/`uv` attributes. Points,
post, screen, and feedback materials reject an authored vertex stage with a diagnostic.
Mesh materials with an authored vertex stage may also declare `instance name : type`
attributes and read `instanceIndex` to draw many instances in one call; see the
Instancing section of the language guide.
A separate composing hook — `vertex(geo) { ... }` that mutates geometry while
the default derived-field and interpolant pipeline still runs — is not
implemented yet.

## Build-time artifacts

Compile once at build time and ship the result. An artifact file holds, for
every material in one `.sel` source, the binding layout and the shader for each
target, plus the source's SHA-256, the build options, the language version and
the Selena version that wrote it:

```sh
selena artifact -targets wgsl,gles -modules lighting materials/rock.sel   # writes materials/rock.sel.json
selena artifact -check materials/rock.sel                                 # fails when the file is stale
```

The same source, options and Selena version always produce the same bytes.
Package `artifact` decodes a file and converts a material to a `result.Result`;
it imports only `bindings`, `hir`, `ir` and `result`, so a browser WASM module
can load compiled materials without linking the parser, the tree-sitter grammar,
the lowerer or the emitters (a test fails if that changes). Package
`artifact/builder` writes files (`builder.Build`) and checks a shipped file
against its source (`builder.Check`, which returns a `*builder.StaleError` when
the source hash differs or the linked compiler emits a different layout or
shader; a new Selena version that emits the same output is not stale).

```go
file, err := artifact.Decode(data)
if err != nil {
    return err
}
res, err := file.Result("Rock") // "" selects the file's default, the last material
wgsl, _ := res.Artifact(result.TargetWGSL)
```

GoSX runs this step in `gosx build` and loads the files with
`scene.MaterialFromSelenaArtifact`.

## How it plugs into GoSX

- **Grammar engine:** `gotreesitter` + `grammargen` — the same stack that builds
  `.gsx` (Go+JSX) and `.swift.gsx` (Swift+JSX). The Selena DSL is the natural
  third member of that family.
- **Scene contract:** the Selena IR is the source of truth for a material; the
  per-backend string slots already present on `gosx/scene.IR`'s `IRMaterial`
  (`CustomVertexWGSL`, `CustomFragment`, …) become *emitter outputs* rather than
  authoring inputs.
- **Honesty gate:** GoSX already models "which backends can serve this material,"
  so partial emitter coverage degrades gracefully — Selena can ship one emitter at
  a time and the gate handles the rest.

Selena's core (grammar → IR → emitters) is intentionally **standalone and
dependency-free**: the GoSX/`scene.IR` and `gosx-native` integrations live in thin
adapter layers so the language stays reusable.

## Project layout

| Path | Role |
|---|---|
| `grammar/` | the `grammargen`-defined Selena shader grammar (`.sel`) |
| `lower/` | Selena AST → Selena IR |
| `ir/` | the neutral, typed shader IR — the heart of the project |
| `emit/wgsl/` | IR → WGSL (browser + desktop WebGPU) |
| `emit/glsl/` | IR → GLSL (browser + desktop WebGL) |
| `emit/metal/` | IR → Metal MSL (iOS SceneKit) |
| `emit/gles/` | IR → GLSL ES (Android GLES) |
| `result/` | compile-result types (`Result`, `Artifact`, `Target`, …) in a leaf package that does not import the compiler |
| `artifact/` | the build artifact file format, decoded without the compiler |
| `artifact/builder/` | writes and checks artifact files (links the compiler) |
| `materiallib/` | opt-in Selena helper modules, including `lighting` for scene lights, shadows and fog |
| `cmd/selena/` | CLI: `selena emit`, `selena check`, `selena inspect`, `selena artifact`, `selena demo` |
| `examples/`, `testdata/` | sample materials, conformance inputs, and golden outputs |

## Open design decisions

These are the calls to make before deep implementation (a dedicated design pass
will resolve them):

1. **IR shape** — typed expression AST vs. an explicit node-graph. Leaning typed
   AST with a small, total expression language (no unbounded loops) so all four
   emitters are straightforward and the output is deterministic.
2. **DSL surface** — a standalone `.sel` file vs. a `<Material>`/shader block
   embedded in `.gsx`. Likely both: a file form for reusable materials and an
   inline form for one-offs.
3. **Type & binding model** — how uniforms/attributes/samplers/varyings are
   declared once and mapped to each backend's binding conventions (WGSL bind
   groups vs. GLSL uniforms vs. SceneKit `SCNProgram` semantics).
4. **Standard-material interop** — Selena materials should compose with the
   existing lit/PBR standard material (extend it, not replace it).
5. **Conformance** — the native Go WebGPU renderer (`gosx/render/bundle`) stays
   useful as a *pixel oracle* to validate the WGSL emitter; golden tests per
   emitter otherwise.

## Status

Vertical slice online. Selena now parses `.sel` files, lowers typed HIR into the
neutral shader IR, computes host binding layouts, emits WGSL / GLSL / Metal /
GLSL-ES, exposes a root `Compile` API, packs host uniform blocks from the
descriptor, adapts into GoSX `scene.IRMaterial`, and compile-checks emitted WGSL
and GLSL-ES where offline validators are installed.

Current compiler coverage includes directional diffuse materials, texture
sampling, reusable functions, material inheritance via `extends` /
`super.surface`, deterministic binding descriptors with scalar/vector defaults,
matrix defaults, CLI inspectability, and source-aware semantic validation with
annotated CLI snippets before backend shader emission. Function inlining and
invalid `super.surface` usage now also report call-site diagnostics. The stdlib
registry now centralizes `Sun` fields, geometry producers, and builtin typing
metadata; backend builtin emission goes through declarative per-target spelling
tables; descriptors carry explicit schema/language versions; and descriptor
defaults now feed the demo harness plus GoSX `CustomUniforms`. Compatibility
notes, a `.sel` conformance corpus, and parser/compile benchmarks now exist as
the baseline for broader material/PBR interop and runtime conformance work. The
standard-material interop boundary is now documented so future PBR hooks extend
GoSX's renderer instead of forking it.

See [CONTRIBUTING.md](CONTRIBUTING.md) for development notes. Planning lives in
the project's knowledge space, not in this repository.
