// Command selena is the CLI for the Selena shader authoring language.
//
//	selena emit <target> <file.sel>   # target: wgsl | glsl | metal | gles
//	selena check <file.sel>           # parse + lower (front-end check)
//	selena demo  <out.html> [material]
//
// emit/check parse a .sel file through the full pipeline (parse -> HIR -> lower
// -> emit). The names "sample"/"directional-diffuse" and "textured" resolve to
// the built-in materials instead of a file.
package main

import (
	"fmt"
	"os"

	"m31labs.dev/selena/emit/gles"
	"m31labs.dev/selena/emit/glsl"
	"m31labs.dev/selena/emit/metal"
	"m31labs.dev/selena/emit/wgsl"
	"m31labs.dev/selena/hir"
	"m31labs.dev/selena/lower"
	"m31labs.dev/selena/parse"
)

const usage = `selena - shader authoring for the GoSX ecosystem

usage:
  selena emit <target> <file.sel>   emit a shader (target: wgsl|glsl|metal|gles)
  selena check <file.sel>           parse + lower a material
  selena demo <out.html> [material] render harness (material: directional-diffuse|textured)
  selena help                       show this help

<file.sel> may also be a built-in material name: 'directional-diffuse' (or
'sample') and 'textured'. Examples:

  selena emit wgsl examples/directional-diffuse.sel
  selena emit metal textured
  selena check examples/textured.sel
`

var emitTargets = map[string]bool{"wgsl": true, "glsl": true, "metal": true, "gles": true}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "selena:", err)
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
		if len(args) != 3 {
			return fmt.Errorf("usage: selena emit <target> <file.sel>")
		}
		if !emitTargets[args[1]] {
			return fmt.Errorf("unknown target %q (want one of: wgsl, glsl, metal, gles)", args[1])
		}
		return emit(args[1], args[2])
	case "check":
		if len(args) != 2 {
			return fmt.Errorf("usage: selena check <file.sel>")
		}
		return check(args[1])
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

// resolveProgram resolves a built-in material name or parses a .sel file,
// returning the program and the index of the target material (the last, i.e.
// most-derived, material declared).
func resolveProgram(nameOrFile string) (hir.Program, int, error) {
	switch nameOrFile {
	case "sample", "directional-diffuse":
		return hir.Program{Materials: []hir.Material{hir.DirectionalDiffuse()}}, 0, nil
	case "textured":
		return hir.Program{Materials: []hir.Material{hir.Textured()}}, 0, nil
	default:
		src, err := os.ReadFile(nameOrFile)
		if err != nil {
			return hir.Program{}, 0, err
		}
		p, err := parse.Program(src)
		if err != nil {
			return hir.Program{}, 0, err
		}
		if len(p.Materials) == 0 {
			return hir.Program{}, 0, fmt.Errorf("%s: no material declared", nameOrFile)
		}
		return p, len(p.Materials) - 1, nil
	}
}

func emit(target, file string) error {
	prog, idx, err := resolveProgram(file)
	if err != nil {
		return err
	}
	mod, _, err := lower.LowerProgram(prog, idx)
	if err != nil {
		return err
	}
	switch target {
	case "wgsl":
		src, err := wgsl.Emit(mod)
		if err != nil {
			return err
		}
		fmt.Print(src)
	case "metal":
		src, err := metal.Emit(mod)
		if err != nil {
			return err
		}
		fmt.Print(src)
	case "glsl":
		vert, frag, err := glsl.Emit(mod)
		if err != nil {
			return err
		}
		fmt.Printf("// --- vertex ---\n%s\n// --- fragment ---\n%s", vert, frag)
	case "gles":
		vert, frag, err := gles.Emit(mod)
		if err != nil {
			return err
		}
		fmt.Printf("// --- vertex ---\n%s\n// --- fragment ---\n%s", vert, frag)
	}
	return nil
}

func check(file string) error {
	prog, idx, err := resolveProgram(file)
	if err != nil {
		return err
	}
	mod, layout, err := lower.LowerProgram(prog, idx)
	if err != nil {
		return err
	}
	fmt.Printf("ok: %s — %d uniforms (%d-byte block), %d attributes, %d varyings, %d textures\n",
		mod.Name, len(mod.Uniforms), layout.UniformBlock.Size, len(mod.Attributes), len(mod.Varyings), len(mod.Textures))
	return nil
}
