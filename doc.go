// Package selena is the shader & material authoring language for the GoSX
// ecosystem: author a material once and emit the correct shader for every
// rendering target (WGSL/GLSL for browser + desktop, Metal/GLSL-ES for mobile
// via gosx-native).
//
// The pipeline is: .sel source -> grammar (gotreesitter/grammargen) -> AST ->
// lower -> the neutral Selena IR (package ir) -> per-target emitters
// (package emit/...). See README.md for the architecture and design notes.
//
// The root Compile APIs are the stable integration point for tools and hosts:
// parse .sel source, select a material, lower it to IR, compute the binding
// descriptor, and optionally emit all supported shader targets.
//
// The compile-result types (Target, CompileOptions, FragmentArtifact, Artifact
// and Result) are defined in package result, a leaf package that does not
// import the compiler. This package re-exports them as aliases, so a host that
// only reads or passes along compile output can import result and avoid
// linking the parser, the lowerer and the emitters.
package selena
