package materiallib

import (
	"math"
	"testing"

	"m31labs.dev/selena"
	"m31labs.dev/selena/parse"
)

func TestWellGradientAndNormals(t *testing.T) {
	r := referenceLibrary(t)
	for _, p := range [][]float64{{0, 0}, {0.05, 0.03}, {0.1, -0.07}, {0.3, 0}} {
		gradient := r.call("mlPipWellGradient", p, scalar(0.2), scalar(0.04))
		for axis := 0; axis < 2; axis++ {
			a, b := append([]float64(nil), p...), append([]float64(nil), p...)
			a[axis] += 1e-6
			b[axis] -= 1e-6
			near(t, gradient[axis], (r.call("mlPipWellHeight", a, scalar(0.2), scalar(0.04))[0]-r.call("mlPipWellHeight", b, scalar(0.2), scalar(0.04))[0])/2e-6, 1e-8)
		}
		n := r.call("mlPipWellNormal", p, scalar(0.2), scalar(0.04))
		near(t, math.Sqrt(n[0]*n[0]+n[1]*n[1]+n[2]*n[2]), 1, 1e-12)
	}
	for _, radius := range []float64{0, -1, 0.2} {
		n := r.call("mlPipWellNormal", []float64{0, 0}, scalar(radius), scalar(0.04))
		near(t, n[2], 1, 1e-12)
	}
	near(t, r.call("mlPipWellHeight", []float64{0.2, 0}, scalar(0.2), scalar(0.04))[0], 0, 1e-12)
	n := r.call("mlNormalToWorld", []float64{0, 0, 1}, []float64{0, 1, 0}, []float64{1, 0, 0}, scalar(-1))
	near(t, n[1], 1, 1e-12)
}

func TestBoundedParallaxAndPhoneFallback(t *testing.T) {
	r := referenceLibrary(t)
	uv := []float64{0.5, 0.5}
	for _, view := range [][]float64{{0, 0, 1}, {1, 0, 0.001}, {-1, 1, 0}} {
		shift := r.call("mlParallaxUV", uv, view, scalar(1), scalar(0.5), scalar(0.04))
		if math.Hypot(shift[0]-uv[0], shift[1]-uv[1]) > 0.04000001 {
			t.Fatal("parallax exceeded maximum offset", shift)
		}
	}
	for i, v := range r.call("mlParallaxUVPhone", uv) {
		near(t, v, uv[i], 1e-12)
	}
	source := []byte(`material Phone { surface(geo) -> color { let mappedUV = mlParallaxUVPhone(geo.uv) return vec3f(mappedUV, 0.0) } }`)
	res, err := Compile(source, selena.CompileOptions{}, Normals)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Layout.Textures) != 0 || len(res.Layout.Requires.GLExtensions) != 0 {
		t.Fatal("phone fallback should need no textures or derivatives")
	}
	validateArtifacts(t, res)
}

func TestEventLifetimesAndPositions(t *testing.T) {
	r := referenceLibrary(t)
	for _, age := range []float64{-1, -0.01, 0.5, 1, 10} {
		near(t, r.call("mlEventEnvelope", scalar(age), scalar(0.5), scalar(1))[0], 0, 1e-12)
		near(t, r.call("mlDustWidth", []float64{0, 0}, []float64{0, 0}, scalar(age), scalar(1), scalar(0.5), scalar(0.5), scalar(0.1), scalar(30), scalar(0.01))[0], 0, 1e-12)
	}
	for _, duration := range []float64{0, -1} {
		near(t, r.call("mlEventEnvelope", scalar(0), scalar(duration), scalar(1))[0], 0, 1e-12)
	}
	near(t, r.call("mlEventEnvelope", scalar(0), scalar(1), scalar(2))[0], 1, 1e-12)
	near(t, r.call("mlEventEnvelope", scalar(0.5), scalar(1), scalar(1))[0], 0.25, 1e-12)
	near(t, r.call("mlEventEnvelope", scalar(0), scalar(1), scalar(-1))[0], 0, 1e-12)
	a := r.call("mlImpactFlashWidth", []float64{0.3, 0.4}, []float64{0.3, 0.4}, scalar(0), scalar(1), scalar(0.3), scalar(0.1), scalar(0))[0]
	b := r.call("mlImpactFlashWidth", []float64{0.7, 0.4}, []float64{0.3, 0.4}, scalar(0), scalar(1), scalar(0.3), scalar(0.1), scalar(0))[0]
	if a <= b {
		t.Fatal("flash is not centered on event position")
	}
	c := r.call("mlImpactFlashWidth", []float64{0.3, 0.4}, []float64{0.3, 0.4}, scalar(0), scalar(1), scalar(0.3), scalar(0.1), scalar(1))[0]
	if c >= a {
		t.Fatal("unresolved flash should attenuate")
	}
}

func TestEffectDependenciesDeduplicate(t *testing.T) {
	functions, err := Functions(Effects, Procedural, Effects)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, f := range functions {
		if seen[f.Name] {
			t.Fatal("duplicate dependency", f.Name)
		}
		seen[f.Name] = true
	}
	if !seen["mlValueNoise2Filtered"] {
		t.Fatal("Effects did not load Procedural")
	}
	program, _ := parse.Program([]byte(`material Empty { surface(geo) -> color { return rgb(0.0,0.0,0.0) } }`))
	if _, err := Link(program, Procedural, Effects); err != nil {
		t.Fatal(err)
	}
}
