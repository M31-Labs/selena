package materiallib

import (
	"math"
	"os"
	"testing"

	"m31labs.dev/selena"
)

func TestPolygonLoopConformsAcrossTargets(t *testing.T) {
	source, err := os.ReadFile("../testdata/conformance/material-library/polygon.sel")
	if err != nil {
		t.Fatal(err)
	}
	res, err := Compile(source, selena.CompileOptions{}, Shapes)
	if err != nil {
		t.Fatal(err)
	}
	validateArtifacts(t, res)
}

func TestShapeDistancesAndCoverage(t *testing.T) {
	r := referenceLibrary(t)
	near(t, r.call("mlSDCircle", []float64{0, 0}, scalar(1))[0], -1, 1e-12)
	near(t, r.call("mlSDRect", []float64{2, 2}, []float64{1, 1})[0], math.Sqrt(2), 1e-12)
	near(t, r.call("mlSDRect", []float64{0, 0}, []float64{1, 2})[0], -1, 1e-12)
	near(t, r.call("mlSDSegment", []float64{1, 1}, []float64{0, 0}, []float64{0, 0})[0], math.Sqrt(2), 1e-12)
	for _, vertices := range [][][]float64{{{0, 0}, {1, 0}, {0, 1}}, {{0, 1}, {1, 0}, {0, 0}}} {
		near(t, r.call("mlSDTriangle", []float64{0.2, 0.2}, vertices[0], vertices[1], vertices[2])[0], -0.2, 1e-12)
		near(t, r.call("mlSDTriangle", []float64{-1, 0}, vertices[0], vertices[1], vertices[2])[0], 1, 1e-12)
	}
	near(t, r.call("mlSDRegularPolygon", []float64{0, 0}, scalar(1), scalar(4), scalar(0))[0], -1, 1e-12)
	near(t, r.call("mlSDRegularPolygon", []float64{2, 2}, scalar(1), scalar(4), scalar(0))[0], math.Sqrt(2), 1e-10)
	for _, p := range [][]float64{{0.2, 0.3}, {0.8, -0.1}, {-0.4, 0.7}} {
		a := r.call("mlSDStar", p, scalar(1), scalar(0.4), scalar(5), scalar(0))[0]
		b := r.call("mlSDStar", []float64{p[0], -p[1]}, scalar(1), scalar(0.4), scalar(5), scalar(0))[0]
		near(t, a, b, 1e-12)
	}
	if r.call("mlSDStar", []float64{0, 0}, scalar(1), scalar(0.4), scalar(5), scalar(0))[0] >= 0 {
		t.Fatal("star center is outside")
	}
	near(t, r.call("mlSDStar", []float64{1, 0}, scalar(1), scalar(0.4), scalar(5), scalar(0))[0], 0, 1e-12)
	near(t, r.call("mlSDStar", []float64{2, 0}, scalar(0), scalar(0), scalar(5), scalar(0))[0], 2, 1e-12)
	for _, width := range []float64{0, 0.001, 0.1} {
		near(t, r.call("mlSDFMaskWidth", scalar(0), scalar(width))[0], 0.5, 1e-12)
		near(t, r.call("mlSDFMaskWidth", scalar(-1), scalar(width))[0], 1, 1e-12)
		near(t, r.call("mlSDFMaskWidth", scalar(1), scalar(width))[0], 0, 1e-12)
	}
}

func TestArbitraryConcavePolygonParity(t *testing.T) {
	r := referenceLibrary(t)
	vertices := [][]float64{{0, 0}, {2, 0}, {2, 1}, {1, 1}, {1, 2}, {0, 2}}
	for _, test := range []struct {
		p        []float64
		distance float64
	}{{[]float64{0.5, 0.5}, -0.5}, {[]float64{1.5, 1.5}, 0.5}, {[]float64{2, 0.5}, 0}} {
		distance, parity := 1e6, 1.0
		for i, a := range vertices {
			edge := r.call("mlPolygonEdge", test.p, a, vertices[(i+1)%len(vertices)])
			distance = math.Min(distance, edge[0])
			parity *= edge[1]
		}
		near(t, r.call("mlSDPolygonFinish", scalar(distance), scalar(parity))[0], test.distance, 1e-12)
	}
}
