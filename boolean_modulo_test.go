package selena

import (
	"os"
	"strings"
	"testing"
)

func TestBooleanEqualityRejectsOrderingAndArithmetic(t *testing.T) {
	for _, expr := range []string{"true < false", "true + false", "true == 1.0"} {
		if _, err := Compile([]byte("material Bad { surface(geo) -> color { let invalid = "+expr+" return rgb(1.0,0.0,0.0) } }"), CompileOptions{}); err == nil {
			t.Fatal("accepted invalid boolean operation", expr)
		}
	}
}

func TestMetalFloorModuloForScalarAndVector(t *testing.T) {
	source, err := os.ReadFile("testdata/conformance/floor-modulo.sel")
	if err != nil {
		t.Fatal(err)
	}
	res, err := Compile(source, CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	a, _ := res.Artifact(TargetMetal)
	if strings.Contains(a.Source, "fmod(") || strings.Count(a.Source, "floor(") != 2 {
		t.Fatal("Metal mod must use floor modulo for both scalar and vector")
	}
}
