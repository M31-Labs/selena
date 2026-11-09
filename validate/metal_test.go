package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	selena "m31labs.dev/selena"
	"m31labs.dev/selena/materiallib"
	"m31labs.dev/selena/mslcheck"
	"m31labs.dev/selena/parse"
)

// metalMaterial is one corpus material compiled for the Metal target.
type metalMaterial struct {
	name   string
	result selena.Result
	source string
}

// metalCorpus compiles the Metal output of every material in the conformance
// corpus and the examples. Materials that do not compile are logged and left
// out: they are meant to fail, or need a host-specific module set, and the
// conformance tests cover them.
func metalCorpus(t *testing.T) []metalMaterial {
	t.Helper()
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
	var out []metalMaterial
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
				t.Logf("%s: not compiled: %v", name, err)
				continue
			}
			artifact, ok := res.Artifact(selena.TargetMetal)
			if !ok {
				t.Fatalf("%s: no metal artifact", name)
			}
			out = append(out, metalMaterial{name: name, result: res, source: artifact.Source})
		}
	}
	return out
}

// TestMetalCompilesWithXcrun compiles the Metal output of every material in
// the conformance corpus and the examples with `xcrun metal`. It skips on
// hosts without the Metal toolchain; see package mslcheck.
//
//	tailnet-run --host george-mbp <selena-worktree> go test ./validate/ -run Metal -v
func TestMetalCompilesWithXcrun(t *testing.T) {
	if _, ok := mslcheck.Find(); !ok {
		t.Skip("no Metal compiler (xcrun metal) on this host")
	}
	mats := metalCorpus(t)
	for _, m := range mats {
		t.Run(m.name, func(t *testing.T) {
			if _, err := mslcheck.Compile(m.source); err != nil {
				t.Fatal(err)
			}
		})
	}
	t.Logf("compiled %d materials with xcrun metal", len(mats))
}

// TestMetalUniformLayoutXcrun proves with Apple's compiler that the Metal
// uniform struct of every corpus material has the member offsets and total
// size of the binding descriptor, the contract a host fills the buffer from.
// It appends one static_assert per member and one for sizeof to the emitted
// source and compiles the result.
func TestMetalUniformLayoutXcrun(t *testing.T) {
	if _, ok := mslcheck.Find(); !ok {
		t.Skip("no Metal compiler (xcrun metal) on this host")
	}
	checked := 0
	for _, m := range metalCorpus(t) {
		block := m.result.Layout.UniformBlock
		if len(block.Fields) == 0 {
			continue
		}
		structName := uniformStructName(t, m)
		src := m.source + "\n"
		for _, f := range block.Fields {
			src += fmt.Sprintf("static_assert(__builtin_offsetof(%s, %s) == %d, \"offset of %s\");\n", structName, f.Name, f.Offset, f.Name)
		}
		src += fmt.Sprintf("static_assert(sizeof(%s) == %d, \"size of %s\");\n", structName, block.Size, structName)
		t.Run(m.name, func(t *testing.T) {
			if _, err := mslcheck.Compile(src); err != nil {
				t.Fatal(err)
			}
		})
		checked++
	}
	if checked == 0 {
		t.Fatal("no material with uniforms was checked")
	}
	t.Logf("checked the uniform layout of %d materials with xcrun metal", checked)
}

// TestMetalLayoutFactsXcrun checks with Apple's compiler the size and
// alignment table that the Go layout test and the emitter rely on.
func TestMetalLayoutFactsXcrun(t *testing.T) {
	if _, ok := mslcheck.Find(); !ok {
		t.Skip("no Metal compiler (xcrun metal) on this host")
	}
	src := "#include <metal_stdlib>\nusing namespace metal;\n"
	for typ, sa := range mslSizeAlign {
		src += fmt.Sprintf("static_assert(sizeof(%s) == %d, \"size of %s\");\n", typ, sa[0], typ)
		src += fmt.Sprintf("static_assert(alignof(%s) == %d, \"alignment of %s\");\n", typ, sa[1], typ)
	}
	// An array of float3 has stride 16; a struct ends at a multiple of its
	// alignment.
	src += "static_assert(sizeof(float3[2]) == 32, \"float3 array stride\");\n"
	if _, err := mslcheck.Compile(src); err != nil {
		t.Fatal(err)
	}
}
