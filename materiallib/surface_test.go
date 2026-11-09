package materiallib

import (
	"math"
	"testing"

	"m31labs.dev/selena"
)

// The surface helpers' math, evaluated on the IR the backends emit: the
// triplanar weights sum to one and follow |n|^sharpness, flat per-axis normals
// blend back to the geometric normal, a flat detail leaves RNM unchanged, a
// zero-strength normal map is flat, and the height blend lets the higher
// layer win.
func TestSurfaceHelpersMath(t *testing.T) {
	source := []byte(`material Eval {
    param n : vec3
    param tn : vec3
    surface(geo) -> Surface {
        let nn = normalize(n)
        let w = mlTriplanarWeights(nn, 4.0)
        let flat = vec3f(0.0, 0.0, 1.0)
        let blended = mlTriplanarBlendNormals(flat, flat, flat, nn, w)
        let rnm = mlBlendNormalsRNM(normalize(tn), flat)
        let unpacked = mlUnpackNormal(vec3f(0.9, 0.1, 0.7), 0.0)
        let s0 = pbr(w, mlHeightBlend(0.8, 0.2, 0.5, 0.05), mlHeightBlend(0.2, 0.8, 0.5, 0.05))
        let s1 = withOcclusion(withAlpha(s0, mlHeightBlend(0.9, 0.1, 0.0, 0.05)), mlHeightBlend(0.1, 0.9, 1.0, 0.05))
        let s2 = withSheen(withClearcoat(s1, mlSlopeMask(nn, vec3f(0.0, 1.0, 0.0), 0.5, 0.9), unpacked.z), mlHeightMask(2.5, 1.0, 4.0))
        return withEmissive(withNormal(s2, blended), rnm)
    }
}`)
	res, err := Compile(source, selena.CompileOptions{Targets: []selena.Target{}}, Surface)
	if err != nil {
		t.Fatal(err)
	}
	n := [3]float64{0.3, 0.8, -0.5}
	tn := [3]float64{0.2, -0.3, 0.9}
	m := machine{env: map[string]val{"n": {n[0], n[1], n[2]}, "tn": {tn[0], tn[1], tn[2]}, "selenaGeo_worldNormal": {0, 1, 0}}}
	if _, err := m.run(res.Module.Fragment); err != nil {
		t.Fatal(err)
	}
	out := func(field string) val { return m.env["selenaOut_"+field] }
	norm := func(v [3]float64) [3]float64 {
		l := math.Sqrt(v[0]*v[0] + v[1]*v[1] + v[2]*v[2])
		return [3]float64{v[0] / l, v[1] / l, v[2] / l}
	}
	nn := norm(n)
	var w [3]float64
	sum := 0.0
	for i := range w {
		w[i] = math.Pow(math.Abs(nn[i]), 4)
		sum += w[i]
	}
	near := func(name string, got val, want ...float64) {
		t.Helper()
		for i, v := range want {
			if math.Abs(got.at(i)-v) > 2e-6 {
				t.Errorf("%s[%d] = %v, want %v", name, i, got.at(i), v)
			}
		}
	}
	near("weights", out("baseColor"), w[0]/sum, w[1]/sum, w[2]/sum)
	near("flat triplanar normal", out("normal"), nn[0], nn[1], nn[2])
	ntn := norm(tn)
	near("RNM with a flat detail", out("emissive"), ntn[0], ntn[1], ntn[2])
	near("zero-strength unpack", out("clearcoatRoughness"), 1)
	near("height blend, B in the cracks", out("roughness"), 0)
	near("height blend, B above A", out("metallic"), 1)
	near("height blend, weight 0", out("alpha"), 0)
	near("height blend, weight 1", out("occlusion"), 1)
	// dot(n, up) = 0.8 / |n| = 0.808; smoothstep(0.5, 0.9) of it.
	x := (nn[1] - 0.5) / 0.4
	near("slope mask", out("clearcoat"), x*x*(3-2*x))
	near("height mask", out("sheen"), 0.5)
}
