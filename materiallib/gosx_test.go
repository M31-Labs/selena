package materiallib

import (
	"os"
	"reflect"
	"testing"

	"m31labs.dev/selena"
	"m31labs.dev/selena/adapter/gosx"
	"m31labs.dev/selena/parse"
)

func TestLibraryFeedsGoSXAdapter(t *testing.T) {
	source, err := os.ReadFile("../examples/material-library/brdf.sel")
	if err != nil {
		t.Fatal(err)
	}
	program, err := parse.Program(source)
	if err != nil {
		t.Fatal(err)
	}
	functions, err := Functions(BRDF)
	if err != nil {
		t.Fatal(err)
	}
	material, layout, err := gosx.Material(program.Materials[0], functions)
	if err != nil {
		t.Fatal(err)
	}
	compiled, err := Compile(source, selena.CompileOptions{}, BRDF)
	if err != nil {
		t.Fatal(err)
	}
	wgsl, _ := compiled.Artifact(selena.TargetWGSL)
	glsl, _ := compiled.Artifact(selena.TargetGLSL)
	if material.CustomFragmentWGSL != wgsl.Source || material.CustomFragment != glsl.Fragment || !reflect.DeepEqual(layout, compiled.Layout) {
		t.Fatal("adapter differs from core compiler")
	}
	if material.CustomUniforms["roughness"] == nil {
		t.Fatal("adapter lost library material defaults")
	}
}

func BenchmarkLibraryCompile(b *testing.B) {
	source, err := os.ReadFile("../examples/material-library/brdf.sel")
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Compile(source, selena.CompileOptions{}, BRDF); err != nil {
			b.Fatal(err)
		}
	}
}
