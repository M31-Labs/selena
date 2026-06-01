# Selena Roadmap

Selena already has a useful vertical slice: `.sel` parsing, typed lowering,
binding descriptors, WGSL/GLSL/Metal/GLES emission, GoSX adaptation, CLI demos,
and optional offline shader validation.

The next phase should make the project easier to adopt and harder to misuse.

## Ergonomics

- Extend the new top-level `Compile` API with structured diagnostics once source
  ranges exist. It already returns HIR, IR, shader outputs, and the binding
  descriptor in one object.
- Keep expanding `selena inspect` as the debug surface for HIR, IR, interface
  layout, target output, and descriptor review.
- Add source ranges to parse and lowering errors so diagnostics can point at the
  exact param, expression, or function call.
- Add parameter defaults and generated host-side default values in the binding
  descriptor. The Go std140 packer exists; defaults should feed it directly.
- Add a small language guide with copy-pasteable materials for color, texture,
  tinting, lighting, and composition.

## Completeness

- Replace hardcoded stdlib knowledge with a registry for records, functions,
  geometry fields, and backend spellings.
- Define the material/PBR interop boundary: extension hooks for standard lit
  materials, not a forked material system.
- Add vertex hooks to the parser and lowering path, with clear rules for which
  geometry fields are mutable.
- Add arrays, bools, and integer types only after the binding descriptor can
  express them consistently across WGSL, GLSL, Metal, and GLES.
- Build a conformance corpus of `.sel` inputs and golden outputs for every
  backend.

## Real-World Readiness

- Exercise generated shaders in browser automation and native/mobile CI, not
  only string and offline compiler tests.
- Add public compatibility notes for WebGPU, WebGL1, WebGL2, SceneKit, and
  Android GLES.
- Version the `.sel` language and descriptor schema before downstream tools rely
  on them.
- Publish a few complete GoSX examples that load real textures, pack uniforms
  through the descriptor, and switch between standard and Selena materials.
- Add benchmark coverage for parser generation/cache behavior and compile time.
