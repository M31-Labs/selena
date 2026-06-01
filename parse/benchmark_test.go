package parse

import (
	"os"
	"testing"

	"m31labs.dev/selena/hir"
)

var benchmarkProgram hir.Program
var benchmarkLanguageErr error

func BenchmarkLanguageCached(b *testing.B) {
	var err error
	for i := 0; i < b.N; i++ {
		_, err = language()
	}
	benchmarkLanguageErr = err
}

func BenchmarkProgramParseTextured(b *testing.B) {
	src, err := os.ReadFile("../examples/textured.sel")
	if err != nil {
		b.Fatal(err)
	}

	var p hir.Program
	for i := 0; i < b.N; i++ {
		p, err = Program(src)
		if err != nil {
			b.Fatal(err)
		}
	}
	benchmarkProgram = p
}
