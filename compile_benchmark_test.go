package selena

import (
	"os"
	"testing"
)

var benchmarkCompileResult Result

func BenchmarkCompileTexturedAllTargets(b *testing.B) {
	src, err := os.ReadFile("examples/textured.sel")
	if err != nil {
		b.Fatal(err)
	}

	var res Result
	for i := 0; i < b.N; i++ {
		res, err = Compile(src, CompileOptions{})
		if err != nil {
			b.Fatal(err)
		}
	}
	benchmarkCompileResult = res
}

func BenchmarkCompileTexturedLowerOnly(b *testing.B) {
	src, err := os.ReadFile("examples/textured.sel")
	if err != nil {
		b.Fatal(err)
	}

	var res Result
	opts := CompileOptions{Targets: []Target{}}
	for i := 0; i < b.N; i++ {
		res, err = Compile(src, opts)
		if err != nil {
			b.Fatal(err)
		}
	}
	benchmarkCompileResult = res
}
