package materiallib

import (
	"math"
	"testing"

	"m31labs.dev/selena"
)

func TestNoiseAnalyticDerivatives(t *testing.T) {
	r := referenceLibrary(t)
	for _, name := range []string{"mlValueNoise2Deriv", "mlGradientNoise2Deriv", "mlSimplexNoise2Deriv"} {
		for _, p := range [][]float64{{0.23, 0.71}, {-3.41, 2.17}, {9.81, -11.13}} {
			v := r.call(name, p)
			const epsilon = 0.00001
			for axis := 0; axis < 2; axis++ {
				a, b := append([]float64(nil), p...), append([]float64(nil), p...)
				a[axis] += epsilon
				b[axis] -= epsilon
				derivative := (r.call(name, a)[0] - r.call(name, b)[0]) / (2 * epsilon)
				near(t, v[axis+1], derivative, 0.00001)
			}
		}
	}
}

func TestNoiseBoundsContinuityAndFiltering(t *testing.T) {
	r := referenceLibrary(t)
	for i := 0; i < 200; i++ {
		p := []float64{float64(i)*0.371 - 27.0, float64(i)*0.193 - 15}
		h := r.call("mlHash21", p)[0]
		if h < 0 || h >= 1 {
			t.Fatal("hash outside [0,1)", h)
		}
		v := r.call("mlValueNoise2", p)[0]
		if v < 0 || v > 1 {
			t.Fatal("value noise outside [0,1]", v)
		}
		for _, name := range []string{"mlGradientNoise2", "mlSimplexNoise2"} {
			v := r.call(name, p)[0]
			if math.Abs(v) > 1.05 {
				t.Fatal("noise outside expected bounds", name, v)
			}
		}
		near(t, r.call("mlValueNoise2Filtered", p, scalar(2))[0], 0.5, 1e-12)
		near(t, r.call("mlFBm4Filtered", p, scalar(2))[0], 0, 1e-12)
		near(t, r.call("mlWoodGrainFiltered", p, scalar(20), scalar(8), scalar(2))[0], 0.5, 1e-12)
		near(t, r.call("mlMarbleVeinFiltered", p, scalar(20), scalar(8), scalar(2))[0], 0.196380615234375, 1e-12)
	}
	for _, name := range []string{"mlValueNoise2Deriv", "mlGradientNoise2Deriv"} {
		a := r.call(name, []float64{1 - 1e-6, 0.37})
		b := r.call(name, []float64{1 + 1e-6, 0.37})
		for i := range a {
			near(t, a[i], b[i], 0.00002)
		}
	}
	noise := r.call("mlFBm4DerivFiltered", []float64{0.27, -0.83}, scalar(0.02))
	for axis := 0; axis < 2; axis++ {
		a, b := []float64{0.27, -0.83}, []float64{0.27, -0.83}
		a[axis] += 1e-5
		b[axis] -= 1e-5
		near(t, noise[axis+1], (r.call("mlFBm4Filtered", a, scalar(0.02))[0]-r.call("mlFBm4Filtered", b, scalar(0.02))[0])/2e-5, 1e-5)
	}
}

func TestDerivativeLibraryStageRestrictions(t *testing.T) {
	source := []byte(`material Invalid { vertex(geo) -> vec4 { return vec4f(geo.position + vec3f(mlFBm4AA(geo.uv),0.0,0.0),1.0) } surface(geo) -> color { return rgb(1.0,1.0,1.0) } }`)
	if _, err := Compile(source, selena.CompileOptions{}, Procedural); err == nil {
		t.Fatal("fragment derivative accepted in vertex")
	}
}
