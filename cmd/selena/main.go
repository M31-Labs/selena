// Command selena is the CLI for the Selena shader authoring language.
//
//	selena emit <target> <file.sel> [material]  # target: wgsl | glsl | metal | gles
//	selena check <file.sel> [material]          # parse + lower (front-end check)
//	selena inspect <file.sel> [material]        # print interface + descriptor
//	selena demo  <out.html> [material]
//
// emit/check parse a .sel file through the full pipeline (parse -> HIR -> lower
// -> emit). The names "sample"/"directional-diffuse" and "textured" resolve to
// the built-in materials instead of a file.
package main

import (
	"fmt"
	"os"

	"m31labs.dev/selena"
	"m31labs.dev/selena/bindings"
	"m31labs.dev/selena/hir"
	"m31labs.dev/selena/ir"
	"m31labs.dev/selena/parse"
)

const usage = `selena - shader authoring for the GoSX ecosystem

usage:
  selena emit <target> <file.sel> [material]  emit a shader (target: wgsl|glsl|metal|gles)
  selena check <file.sel> [material]          parse + lower a material
  selena inspect <file.sel> [material]        print material interface + descriptor
  selena demo <out.html> [material] render harness (material: directional-diffuse|textured)
  selena help                       show this help

<file.sel> may also be a built-in material name: 'directional-diffuse' (or
'sample') and 'textured'. Examples:

  selena emit wgsl examples/directional-diffuse.sel
  selena emit metal textured
  selena inspect examples/tinted.sel Tinted
  selena check examples/textured.sel
`

var emitTargets = map[string]selena.Target{
	"wgsl":  selena.TargetWGSL,
	"glsl":  selena.TargetGLSL,
	"metal": selena.TargetMetal,
	"gles":  selena.TargetGLES,
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		printCommandError(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Print(usage)
		return nil
	}
	switch args[0] {
	case "emit":
		if len(args) < 3 || len(args) > 4 {
			return fmt.Errorf("usage: selena emit <target> <file.sel> [material]")
		}
		target, ok := emitTargets[args[1]]
		if !ok {
			return fmt.Errorf("unknown target %q (want one of: wgsl, glsl, metal, gles)", args[1])
		}
		material := ""
		if len(args) == 4 {
			material = args[3]
		}
		return emit(target, args[2], material)
	case "check":
		if len(args) < 2 || len(args) > 3 {
			return fmt.Errorf("usage: selena check <file.sel> [material]")
		}
		material := ""
		if len(args) == 3 {
			material = args[2]
		}
		return check(args[1], material)
	case "inspect":
		if len(args) < 2 || len(args) > 3 {
			return fmt.Errorf("usage: selena inspect <file.sel> [material]")
		}
		material := ""
		if len(args) == 3 {
			material = args[2]
		}
		return inspect(args[1], material)
	case "demo":
		if len(args) < 2 || len(args) > 3 {
			return fmt.Errorf("usage: selena demo <out.html> [material]  (material: directional-diffuse|textured)")
		}
		mat := ""
		if len(args) == 3 {
			mat = args[2]
		}
		return runDemo(args[1], mat)
	default:
		return fmt.Errorf("unknown command %q (run 'selena help')", args[0])
	}
}

// resolveProgram resolves a built-in material name or parses a .sel file.
// Downstream compile calls select an explicit material name, or the last material
// declared when no name is given.
type resolvedProgram struct {
	Program hir.Program
	File    string
	Source  []byte
}

func resolveProgram(nameOrFile string) (resolvedProgram, error) {
	switch nameOrFile {
	case "sample", "directional-diffuse":
		return resolvedProgram{Program: hir.Program{Materials: []hir.Material{hir.DirectionalDiffuse()}}}, nil
	case "textured":
		return resolvedProgram{Program: hir.Program{Materials: []hir.Material{hir.Textured()}}}, nil
	default:
		src, err := os.ReadFile(nameOrFile)
		if err != nil {
			return resolvedProgram{}, err
		}
		p, err := parse.Program(src)
		if err != nil {
			return resolvedProgram{}, attachSource(nameOrFile, src, err)
		}
		if len(p.Materials) == 0 {
			return resolvedProgram{}, fmt.Errorf("%s: no material declared", nameOrFile)
		}
		return resolvedProgram{Program: p, File: nameOrFile, Source: src}, nil
	}
}

func emit(target selena.Target, file, material string) error {
	prog, err := resolveProgram(file)
	if err != nil {
		return err
	}
	res, err := selena.CompileProgram(prog.Program, selena.CompileOptions{
		Material: material,
		Targets:  []selena.Target{target},
	})
	if err != nil {
		return attachResolvedSource(prog, err)
	}
	artifact, ok := res.Artifact(target)
	if !ok {
		return fmt.Errorf("target %q was not emitted", target)
	}
	printArtifact(artifact)
	return nil
}

func printArtifact(a selena.Artifact) {
	if a.Source != "" {
		fmt.Print(a.Source)
		return
	}
	fmt.Printf("// --- vertex ---\n%s\n// --- fragment ---\n%s", a.Vertex, a.Fragment)
}

func check(file, material string) error {
	prog, err := resolveProgram(file)
	if err != nil {
		return err
	}
	res, err := selena.CompileProgram(prog.Program, selena.CompileOptions{
		Material: material,
		Targets:  []selena.Target{},
	})
	if err != nil {
		return attachResolvedSource(prog, err)
	}
	fmt.Printf("ok: %s — %d uniforms (%d-byte block), %d attributes, %d varyings, %d textures\n",
		res.Module.Name, len(res.Module.Uniforms), res.Layout.UniformBlock.Size, len(res.Module.Attributes), len(res.Module.Varyings), len(res.Module.Textures))
	return nil
}

func inspect(file, material string) error {
	prog, err := resolveProgram(file)
	if err != nil {
		return err
	}
	res, err := selena.CompileProgram(prog.Program, selena.CompileOptions{Material: material})
	if err != nil {
		return attachResolvedSource(prog, err)
	}
	desc, err := res.Layout.JSON()
	if err != nil {
		return err
	}

	fmt.Printf("material: %s\n", res.Module.Name)
	fmt.Printf("program: %d funcs, %d materials\n", len(res.Program.Funcs), len(res.Program.Materials))
	fmt.Println()
	printHIRSummary(res)
	fmt.Println()
	printIRSummary(res.Module)
	fmt.Println()
	printLayoutSummary(res.Layout)
	fmt.Println()
	printTargetSummary(res.Artifacts)
	fmt.Println()
	fmt.Println("descriptor:")
	fmt.Println(desc)
	return nil
}

func printHIRSummary(res selena.Result) {
	fmt.Println("hir:")
	fmt.Printf("  selected: %s", res.Material.Name)
	if res.Material.Extends != "" {
		fmt.Printf(" extends %s", res.Material.Extends)
	}
	fmt.Println()
	fmt.Printf("  surface geo: %s\n", res.Material.Surface.Geo)
	printParams("  authored params", res.Material.Params)
}

func printParams(label string, params []hir.Param) {
	fmt.Printf("%s:\n", label)
	if len(params) == 0 {
		fmt.Println("    (none)")
		return
	}
	for _, p := range params {
		fmt.Printf("    %s: %s\n", p.Name, p.Type)
	}
}

func printIRSummary(mod ir.Module) {
	fmt.Println("ir:")
	printBindings("  uniforms", mod.Uniforms)
	printBindings("  attributes", mod.Attributes)
	printBindings("  varyings", mod.Varyings)
	if len(mod.Textures) == 0 {
		fmt.Println("  textures:")
		fmt.Println("    (none)")
		return
	}
	fmt.Println("  textures:")
	for _, t := range mod.Textures {
		fmt.Printf("    %s\n", t.Name)
	}
}

func printBindings(label string, bindings []ir.Binding) {
	fmt.Printf("%s:\n", label)
	if len(bindings) == 0 {
		fmt.Println("    (none)")
		return
	}
	for _, b := range bindings {
		fmt.Printf("    %s: %s\n", b.Name, b.Type)
	}
}

func printLayoutSummary(layout bindings.Layout) {
	fmt.Println("layout:")
	fmt.Printf("  uniform block: %d bytes\n", layout.UniformBlock.Size)
	for _, f := range layout.UniformBlock.Fields {
		fmt.Printf("    %s: %s offset=%d size=%d\n", f.Name, f.Type, f.Offset, f.Size)
	}
	if len(layout.UniformBlock.Defaults) > 0 {
		fmt.Println("  defaults:")
		for _, d := range layout.UniformBlock.Defaults {
			fmt.Printf("    %s: %v\n", d.Name, d.Values)
		}
	}
	fmt.Println("  textures:")
	if len(layout.Textures) == 0 {
		fmt.Println("    (none)")
		return
	}
	for _, t := range layout.Textures {
		fmt.Printf("    %s: wgsl @group(%d) texture=%d sampler=%d, gl unit=%d, metal texture=%d sampler=%d\n",
			t.Name, t.WGSL.Group, t.WGSL.TextureBinding, t.WGSL.SamplerBinding, t.GL.Unit, t.Metal.Texture, t.Metal.Sampler)
	}
}

func printTargetSummary(artifacts []selena.Artifact) {
	fmt.Println("targets:")
	if len(artifacts) == 0 {
		fmt.Println("  (none)")
		return
	}
	for _, a := range artifacts {
		if a.Source != "" {
			fmt.Printf("  %s: source %d bytes\n", a.Target, len(a.Source))
			continue
		}
		fmt.Printf("  %s: vertex %d bytes, fragment %d bytes\n", a.Target, len(a.Vertex), len(a.Fragment))
	}
}
