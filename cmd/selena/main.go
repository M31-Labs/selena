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
)

const usage = `selena - shader authoring for the GoSX ecosystem

usage:
  selena emit <target> <file.sel>   emit a shader (target: wgsl|glsl|metal|gles)
  selena check <file.sel>           parse + type-check only
  selena help                       show this help
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
		return fmt.Errorf("emit %s %s: compiler pipeline not yet implemented", args[1], args[2])
	case "check":
		if len(args) != 2 {
			return fmt.Errorf("usage: selena check <file.sel>")
		}
		return fmt.Errorf("check %s: compiler pipeline not yet implemented", args[1])
	default:
		return fmt.Errorf("unknown command %q (run 'selena help')", args[0])
	}
}
