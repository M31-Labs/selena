# Contributing to Selena

Selena is early, but the goal is stable: one material source should produce
correct shaders and binding descriptors for every GoSX rendering surface.

## Good First Areas

- Add `.sel` examples that cover one material feature clearly.
- Improve diagnostics with precise source locations and actionable wording.
- Expand emitter conformance tests, especially for texture and uniform binding.
- Add docs for host integration and material authoring patterns.
- File compatibility reports for real WebGPU, WebGL, Metal, or GLES targets.

## Development

```sh
go test ./...
go run ./cmd/selena check examples/textured.sel
go run ./cmd/selena emit wgsl examples/directional-diffuse.sel
go run ./cmd/selena demo /tmp/selena-textured.html textured
```

The validator package uses optional external tools:

- `naga` for WGSL compile checks
- `glslangValidator` for GLSL ES compile checks

Tests skip those checks cleanly when the tools are not installed.

## Pull Requests

Keep changes focused and include tests for compiler behavior. For emitter
changes, add an assertion around the emitted source and prefer a validation path
that compiles the shader when tooling is available.
