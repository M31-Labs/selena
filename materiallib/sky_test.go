package materiallib

import (
	"math"
	"strings"
	"testing"

	"m31labs.dev/selena"
)

const skyProbeSource = `material SkyProbe {
    context {
        skyBetaR : vec4
        skyBetaM : vec4
        skySun   : vec4
        skyG     : float
        probeDir : vec3
    }
    surface(geo) -> color {
        return physicalSky(normalize(probeDir), skyBetaR, skyBetaM, skySun, skyG)
    }
}`

func TestSkyModuleCompilesEveryTarget(t *testing.T) {
	res, err := Compile([]byte(skyProbeSource), selena.CompileOptions{}, Sky)
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range selena.AllTargets() {
		a, ok := res.Artifact(target)
		if !ok {
			t.Fatalf("no %s artifact", target)
		}
		if strings.Contains(a.Source+a.Fragment, "gosxPhysicalSky") {
			t.Errorf("%s: Selena output names the GoSX text-spliced function", target)
		}
	}
	// Engine inputs are typed and marked in the descriptor.
	source := map[string]string{}
	for _, f := range res.Layout.UniformBlock.Fields {
		source[f.Name] = f.Source
	}
	for _, name := range []string{"skyBetaR", "skyBetaM", "skySun", "skyG"} {
		if source[name] != "engine" {
			t.Errorf("%s source = %q, want engine", name, source[name])
		}
	}
	if source["probeDir"] != "" {
		t.Errorf("probeDir is host-custom; source = %q", source["probeDir"])
	}
}

// GoSX reference values. Both skies were evaluated by GoSX's own CPU model,
// scene.Sky.PhysicalRadiance(dir, includeSun=true) at m31labs.dev/gosx commit
// 740f6d01d4 (scene/sky_physical.go, the Go twin the GoSX tests pin against the
// browser's parameter packing). The parameter block is what GoSX's
// newPhysicalSkyParams computes for each sky, in the layout the shaders take:
// betaR.xyz+sunE, betaM.xyz+sunFade, sun.xyz+diskCos, g.
type skyCase struct {
	name              string
	betaR, betaM, sun [4]float64
	g                 float64
	dirs              [][3]float64
	want              [][3]float64
}

var skyCases = []skyCase{
	{
		name:  "golden hour: sun 8 degrees up, turbidity 6, Rayleigh 1.5, Mie 0.004, g 0.85, disk 0.6 degrees",
		betaR: [4]float64{8.70681449e-06, 2.03443671e-05, 4.53988537e-05, 113.022307},
		betaM: [4]float64{3.83307102e-06, 5.79088434e-06, 8.4974727e-06, 1},
		sun:   [4]float64{0.338691627, 0.139173101, -0.930547597, 0.999945169},
		g:     0.85,
		dirs: [][3]float64{
			{0, 1, 0}, {0, 0.447213595, -0.894427191}, {0.980580676, 0.196116135, 0},
			{-0.599251403, 0.049937617, 0.799001871}, {0.301511345, 0.904534034, 0.301511345},
			{0, 0.019996001, -0.99980006}, {0, 0, -1}, {0, -0.371390676, 0.928476691},
			{0.338691627, 0.139173101, -0.930547597},
		},
		want: [][3]float64{
			{0.191334417, 0.307103344, 0.471276675}, {0.688868035, 0.977939569, 1.24000219}, {0.464048455, 0.668161983, 0.78158113},
			{0.921751306, 1.01165011, 0.931568058}, {0.196081162, 0.316236215, 0.483222017},
			{1.98928522, 1.54308693, 1.25374886}, {1.85440424, 1.39532309, 1.22438075}, {0.919030618, 0.867034149, 0.87273265},
			{91.2059001, 68.6185526, 37.3945812},
		},
	},
	{
		name:  "noon: sun 60 degrees up, turbidity 3, Rayleigh 1, Mie 0.003, g 0.7, disk 0.5 degrees",
		betaR: [4]float64{5.804543e-06, 1.35629114e-05, 3.02659025e-05, 515.667159},
		betaM: [4]float64{1.43740163e-06, 2.17158163e-06, 3.18655226e-06, 1},
		sun:   [4]float64{-0.171010072, 0.866025404, 0.46984631, 0.999961923},
		g:     0.7,
		dirs: [][3]float64{
			{0, 1, 0}, {0, 0.447213595, -0.894427191}, {0.980580676, 0.196116135, 0},
			{-0.599251403, 0.049937617, 0.799001871}, {0.301511345, 0.904534034, 0.301511345},
			{0, 0.019996001, -0.99980006}, {0, 0, -1}, {0, -0.371390676, 0.928476691},
			{-0.171010072, 0.866025404, 0.46984631},
		},
		want: [][3]float64{
			{0.578456581, 0.906542139, 1.37339959}, {0.508301002, 0.837870143, 1.27793258}, {0.802681922, 1.26101312, 1.75030225},
			{1.73433208, 2.28157626, 2.52989956}, {0.624753718, 0.974689112, 1.46686045},
			{1.82274972, 2.24303515, 2.3757535}, {2.02663407, 2.31855809, 2.39680539}, {1.87273039, 2.11555639, 2.16898938},
			{208.900873, 202.369647, 189.05771},
		},
	},
}

// TestSkyModuleMatchesGoSXRadiance evaluates the IR that every Selena backend
// emits for physicalSky and compares it with GoSX's CPU radiance. The evaluator
// rounds each operation to float32, so the tolerance covers single-precision
// error and the GoSX reference's float64 arithmetic. It does not run the
// emitted GLSL or WGSL text on a GPU: validation of that text is in
// validate/sky_test.go, and the IR is the shared source of all four emitters.
func TestSkyModuleMatchesGoSXRadiance(t *testing.T) {
	res, err := Compile([]byte(skyProbeSource), selena.CompileOptions{}, Sky)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range skyCases {
		for i, d := range c.dirs {
			m := machine{env: map[string]val{
				"skyBetaR": val{c.betaR[0], c.betaR[1], c.betaR[2], c.betaR[3]},
				"skyBetaM": val{c.betaM[0], c.betaM[1], c.betaM[2], c.betaM[3]},
				"skySun":   val{c.sun[0], c.sun[1], c.sun[2], c.sun[3]},
				"skyG":     val{c.g},
				"probeDir": val{d[0], d[1], d[2]},
			}}
			for k, v := range m.env {
				for j := range v {
					m.env[k][j] = f32(v[j])
				}
			}
			got, err := m.run(res.Module.Fragment)
			if err != nil {
				t.Fatalf("%s dir %d: %v", c.name, i, err)
			}
			if len(got) != 4 {
				t.Fatalf("output has %d components, want vec4", len(got))
			}
			for ch := 0; ch < 3; ch++ {
				want := c.want[i][ch]
				if diff := math.Abs(got[ch] - want); diff > 1e-5*math.Abs(want)+1e-6 {
					t.Errorf("%s dir %v channel %d: Selena %.7g, GoSX %.7g (diff %.2g)", c.name, d, ch, got[ch], want, diff)
				}
			}
		}
	}
}
