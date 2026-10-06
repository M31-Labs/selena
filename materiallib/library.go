// Package materiallib composes opt-in material helpers with Selena programs.
// Helpers are authored in Selena and use the same lowering, emitters and host
// binding descriptor as application materials. Only called helpers are emitted.
package materiallib

import (
	"embed"
	"fmt"
	"strings"
	"sync"

	"m31labs.dev/selena"
	"m31labs.dev/selena/hir"
	"m31labs.dev/selena/parse"
)

// Module identifies a bundled collection of typed Selena functions.
type Module string

const BRDF Module = "brdf"

// Procedural provides hashes, differentiable noise, filtering and grain.
const Procedural Module = "procedural"

//go:embed modules/*.sel
var sources embed.FS
var cache sync.Map

// Functions returns the module's function declarations. Callers may append
// these to a parsed program or pass them to adapter/gosx.Material. The returned
// slice is independent; declarations and their expression trees are read-only.
func Functions(modules ...Module) ([]hir.FuncDecl, error) {
	var out []hir.FuncDecl
	seen := map[Module]bool{}
	for _, module := range modules {
		if seen[module] {
			continue
		}
		seen[module] = true
		if strings.ContainsAny(string(module), "/\\.") {
			return nil, fmt.Errorf("unknown material library module %q", module)
		}
		cached, ok := cache.Load(module)
		if !ok {
			source, err := sources.ReadFile("modules/" + string(module) + ".sel")
			if err != nil {
				return nil, fmt.Errorf("unknown material library module %q", module)
			}
			program, err := parse.Program(source)
			if err != nil {
				return nil, fmt.Errorf("material library %s: %w", module, err)
			}
			for i := range program.Funcs {
				program.Funcs[i].BindLocals = true
			}
			cached, _ = cache.LoadOrStore(module, program.Funcs)
		}
		out = append(out, cached.([]hir.FuncDecl)...)
	}
	return out, nil
}

// Link adds modules without altering the input program. Duplicate function
// names are errors, including an application function that shadows a helper.
func Link(program hir.Program, modules ...Module) (hir.Program, error) {
	functions, err := Functions(modules...)
	if err != nil {
		return hir.Program{}, err
	}
	functions = append(append([]hir.FuncDecl(nil), program.Funcs...), functions...)
	seen := map[string]bool{}
	for _, f := range functions {
		if seen[f.Name] {
			return hir.Program{}, fmt.Errorf("duplicate function %q in material library composition", f.Name)
		}
		seen[f.Name] = true
	}
	program.Funcs = functions
	return program, nil
}

// Compile parses application source, links the requested modules, then runs
// Selena's compiler. Application source spans are preserved for diagnostics.
func Compile(source []byte, opts selena.CompileOptions, modules ...Module) (selena.Result, error) {
	// Use the core parser path so syntax failures retain CompileError diagnostics.
	program, parseErr := parse.Program(source)
	if parseErr != nil {
		return selena.Compile(source, opts)
	}
	program, err := Link(program, modules...)
	if err != nil {
		return selena.Result{}, err
	}
	return selena.CompileProgram(program, opts)
}
