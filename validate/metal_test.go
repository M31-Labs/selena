package validate

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	selena "m31labs.dev/selena"
	"m31labs.dev/selena/materiallib"
	"m31labs.dev/selena/mslcheck"
	"m31labs.dev/selena/parse"
)

// TestMetalCompilesWithXcrun compiles the Metal output of every material in
// the conformance corpus and the examples with `xcrun metal`. It skips on
// hosts without the Metal toolchain; see package mslcheck.
//
//	tailnet-run --host george-mbp <selena-worktree> go test ./validate/ -run Metal -v
func TestMetalCompilesWithXcrun(t *testing.T) {
	if _, ok := mslcheck.Find(); !ok {
		t.Skip("no Metal compiler (xcrun metal) on this host")
	}
	var files []string
	for _, pattern := range []string{"../testdata/conformance/*.sel", "../testdata/conformance/material-library/*.sel", "../testdata/instancing/*.sel", "../examples/*.sel", "../examples/material-library/*.sel"} {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, matches...)
	}
	sort.Strings(files)
	if len(files) < 40 {
		t.Fatalf("found only %d sources; the corpus moved?", len(files))
	}
	compiled := 0
	for _, file := range files {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		program, err := parse.Program(src)
		if err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		linked, err := materiallib.Link(program, materiallib.Modules()...)
		if err != nil {
			linked = program // the file defines a helper name itself
		}
		for _, m := range program.Materials {
			name := filepath.Base(file) + "/" + m.Name
			res, err := selena.CompileProgram(linked, selena.CompileOptions{Material: m.Name, Targets: []selena.Target{selena.TargetMetal}})
			if err != nil {
				// Sources that are meant to fail, or need a host-specific
				// module set, are covered by the conformance tests.
				t.Logf("%s: not compiled: %v", name, err)
				continue
			}
			artifact, ok := res.Artifact(selena.TargetMetal)
			if !ok {
				t.Fatalf("%s: no metal artifact", name)
			}
			t.Run(name, func(t *testing.T) {
				if _, err := mslcheck.Compile(artifact.Source); err != nil {
					t.Fatal(err)
				}
			})
			compiled++
		}
	}
	t.Logf("compiled %d materials with xcrun metal", compiled)
}
