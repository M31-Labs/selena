// Package result holds the data types that describe the outcome of a Selena
// compile: Target (with its constants and the AllTargets function),
// CompileOptions, FragmentArtifact, Artifact and Result.
//
// It is a leaf package. It imports only bindings, hir and ir, and never the
// parser, the lowerer, the emitters, the validator or the tree-sitter grammar.
// A host that only reads or transports compile output, such as a browser WASM
// module that receives artifacts compiled ahead of time, can import result
// without linking the compiler or running its package initializers. A test in
// this package fails if the import closure reaches the compiler.
//
// Package selena re-exports every name here as a type alias, a constant or a
// forwarding function. Code written against selena.Result, selena.Target and
// the rest compiles unchanged, and values pass between the two packages without
// conversion. The functions that produce a Result (Compile, CompileMaterial and
// CompileProgram) stay in package selena.
package result
