// Command selena is the CLI for the Selena shader authoring language.
//
// Planned usage:
//
//	selena emit <target> <file.sel>   # target: wgsl | glsl | metal | gles
//	selena check <file.sel>           # parse + type-check only
//
// The compiler pipeline (grammar -> AST -> IR -> emit) is not yet implemented;
// this scaffold establishes the command surface.
package main

import (
	"fmt"
	"os"

	"m31labs.dev/selena/emit/gles"
	"m31labs.dev/selena/emit/glsl"
	"m31labs.dev/selena/emit/metal"
	"m31labs.dev/selena/emit/wgsl"
	"m31labs.dev/selena/ir"
)

const usage = `selena - shader authoring for the GoSX ecosystem

usage:
  selena emit <target> <file.sel>   emit a shader (target: wgsl|glsl|metal|gles)
  selena check <file.sel>           parse + type-check only
  selena help                       show this help

The grammar/front-end is not built yet; pass the built-in material name
'sample' as <file.sel> to emit the DirectionalDiffuse sample, e.g.

  selena emit wgsl sample
  selena emit glsl sample
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
		return fmt.Errorf("check %s: compiler pipeline not yet implemented", args[1])
	default:
		return fmt.Errorf("unknown command %q (run 'selena help')", args[0])
	}
}

// emit resolves the material (only the built-in 'sample' until the front-end
// lands) and prints the requested target's shader source.
func emit(target, file string) error {
	if file != "sample" {
		return fmt.Errorf("front-end not implemented yet; pass 'sample' to emit the built-in material")
	}
	m := ir.DirectionalDiffuse()
	switch target {
	case "wgsl":
		src, err := wgsl.Emit(m)
		if err != nil {
			return err
		}
		fmt.Print(src)
	case "glsl":
		vert, frag, err := glsl.Emit(m)
		if err != nil {
			return err
		}
		fmt.Printf("// --- vertex ---\n%s\n// --- fragment ---\n%s", vert, frag)
	case "metal":
		src, err := metal.Emit(m)
		if err != nil {
			return err
		}
		fmt.Print(src)
	case "gles":
		vert, frag, err := gles.Emit(m)
		if err != nil {
			return err
		}
		fmt.Printf("// --- vertex ---\n%s\n// --- fragment ---\n%s", vert, frag)
	default:
		return fmt.Errorf("emit %s: emitter not implemented", target)
	}
	return nil
}
