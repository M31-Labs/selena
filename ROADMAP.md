# Selena Roadmap

Selena already has a useful vertical slice: `.sel` parsing, typed lowering,
binding descriptors, WGSL/GLSL/Metal/GLES emission, GoSX adaptation, CLI demos,
and optional offline shader validation.

The next phase should make the project easier to adopt and harder to misuse.

## Ergonomics

- Broaden structured diagnostics across parser, lowerer, and emitters. Source
  ranges, diagnostic codes, `CompileError`, CLI snippets, and basic hints now
  exist for common parser/lowerer failures.
- Keep expanding `selena inspect` as the debug surface for HIR, IR, interface
  layout, target output, and descriptor review.
- Improve diagnostic precision for inlined functions, `super.surface`, parser
  expected-token context, and backend emission failures. User function arity and
  invalid `super.surface` calls now report source-anchored diagnostics; parser
  syntax errors include expected-token context for common malformed input.
- Expand parameter defaults beyond the first scalar/vector slice. `.sel`
  defaults now flow into the binding descriptor and the Go std140 packer for
  float, vec2, vec3/color, vec4, mat3, and mat4 uniforms.
- Keep expanding the language guide as new material features land. The first
  guide now covers color, texture, lighting, composition, defaults, host
  packing, and diagnostics.

## Completeness

- Continue replacing hardcoded stdlib knowledge with registry entries. `Sun`
  record fields, geometry producers, and builtin typing metadata now live in
  the registry; builtin emission now routes through backend dialect call hooks
  backed by declarative per-target spelling tables.
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
  on them. Descriptor JSON now carries `schemaVersion` and `languageVersion`.
- Reject authored names that collide with generated symbols, shader keywords,
  and stdlib builtins before backend emission.
- Publish a few complete GoSX examples that load real textures, pack uniforms
  and defaults through the descriptor, and switch between standard and Selena
  materials. The CLI demo harness and GoSX adapter now carry descriptor defaults.
- Add benchmark coverage for parser generation/cache behavior and compile time.
