package materiallib

import (
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"m31labs.dev/selena"
)

var updateGameGolden = flag.Bool("update-game-golden", false, "rewrite the game material golden shaders")

// gameExamples are the examples/material-library sources built on the game
// module.
var gameExamples = []string{"foliage", "toon", "dissolve", "hologram", "force-field", "ice", "water-surface", "skin"}

func TestGameExamplesMatchGolden(t *testing.T) {
	for _, name := range gameExamples {
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(filepath.Join("..", "examples", "material-library", name+".sel"))
			if err != nil {
				t.Fatal(err)
			}
			res, err := Compile(src, selena.CompileOptions{}, Game, Lighting)
			if err != nil {
				t.Fatal(err)
			}
			validateArtifacts(t, res)
			for _, a := range res.Artifacts {
				outputs := map[string]string{}
				switch a.Target {
				case selena.TargetWGSL, selena.TargetMetal:
					outputs[string(a.Target)] = a.Source
				default:
					outputs[string(a.Target)+".vert"] = a.Vertex
					outputs[string(a.Target)+".frag"] = a.Fragment
				}
				for suffix, got := range outputs {
					path := filepath.Join("testdata", "game", name+"."+suffix)
					if *updateGameGolden {
						if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
							t.Fatal(err)
						}
						if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
							t.Fatal(err)
						}
						continue
					}
					want, err := os.ReadFile(path)
					if err != nil {
						t.Fatalf("%v; run go test ./materiallib -run TestGameExamplesMatchGolden -update-game-golden", err)
					}
					if string(want) != got {
						t.Fatalf("%s changed; rerun with -update-game-golden after reviewing the diff", path)
					}
				}
			}
		})
	}
}

// gameProbe compiles a material whose surface returns vec3f(expr, 0, 0) and
// evaluates it on the CPU over the IR every backend emits.
func gameProbe(t *testing.T, params, expr string, env map[string]val) float64 {
	t.Helper()
	src := fmt.Sprintf("material Probe {\n%s\n    surface(geo) -> color {\n        return vec3f(%s, 0.0, 0.0)\n    }\n}\n", params, expr)
	res, err := Compile([]byte(src), selena.CompileOptions{Targets: []selena.Target{}}, Game)
	if err != nil {
		t.Fatalf("compile %s: %v", expr, err)
	}
	for k, v := range env {
		for i := range v {
			env[k][i] = f32(v[i])
		}
	}
	got, err := machine{env: env}.run(res.Module.Fragment)
	if err != nil {
		t.Fatalf("evaluate %s: %v", expr, err)
	}
	return got[0]
}

// goWindGust is GoSX's scene.WindIR.gustFactor, written out.
func goWindGust(x, z, t float64, dirX, dirZ, gust, gustScale, gustSpeed float64) float64 {
	along := x*dirX + z*dirZ
	across := x*dirZ - z*dirX
	wobble := 0.35 * math.Sin(across*(2*math.Pi)/(gustScale*1.7))
	phase := (along-gustSpeed*t)*(2*math.Pi)/gustScale + wobble
	return 1 + gust*math.Sin(phase)
}

func TestWindGustMatchesGoSXWind(t *testing.T) {
	const params = "    param p : vec3\n    param w : vec4\n    param g : vec4"
	angle := 0.7
	dirX, dirZ := math.Sin(angle), math.Cos(angle)
	for _, c := range [][3]float64{{0, 0, 0}, {3.5, 0, -2}, {-7, 0, 11}, {12.25, 0, 4}} {
		for _, now := range []float64{0, 1.5, 9.75} {
			got := gameProbe(t, params, "mlWindGust(p, w, g)", map[string]val{
				"p": {c[0], c[1], c[2]}, "w": {dirX, dirZ, 6, now}, "g": {0.35, 24, 6, 1.3},
			})
			want := goWindGust(c[0], c[2], now, dirX, dirZ, 0.35, 24, 6)
			if math.Abs(got-want) > 2e-5 {
				t.Errorf("mlWindGust at %v t=%v: Selena %.6f, GoSX %.6f", c, now, got, want)
			}
		}
	}
}

func TestWindSwayKeepsRootsPlanted(t *testing.T) {
	const params = "    param p : vec3\n    param w : vec4\n    param g : vec4\n    param bend : float"
	sway := func(bend, speed float64) float64 {
		return gameProbe(t, params, "length(mlWindSway(p, bend, 1.0, w, g))", map[string]val{
			"p": {1, 0, 2}, "w": {1, 0, speed, 2}, "g": {0.35, 24, speed, 1.3}, "bend": {bend},
		})
	}
	if root := sway(0, 10); root != 0 {
		t.Fatalf("root moved by %v", root)
	}
	if calm := sway(1, 0); calm != 0 {
		t.Fatalf("tip moved by %v with no wind", calm)
	}
	mid, tip := sway(0.5, 10), sway(1, 10)
	if !(mid > 0 && tip > mid*3) {
		t.Fatalf("sway should grow with bend squared: mid %v tip %v", mid, tip)
	}
}

func TestDissolveMaskCoversTheRange(t *testing.T) {
	const params = "    param noise : float\n    param progress : float"
	for _, noise := range []float64{0, 0.3, 0.999} {
		if m := gameProbe(t, params, "mlDissolveMask(noise, progress)", map[string]val{"noise": {noise}, "progress": {0}}); m <= 0 {
			t.Errorf("progress 0 removes noise %v (mask %v)", noise, m)
		}
		if m := gameProbe(t, params, "mlDissolveMask(noise, progress)", map[string]val{"noise": {noise}, "progress": {1}}); m >= 0 {
			t.Errorf("progress 1 keeps noise %v (mask %v)", noise, m)
		}
	}
}

func TestToonBandsAreFlatSteps(t *testing.T) {
	const params = "    param ndl : float"
	for _, c := range []struct{ ndl, want float64 }{{-0.4, 0}, {0.15, 0}, {0.5, 1.0 / 3}, {0.8, 2.0 / 3}, {1, 1}} {
		got := gameProbe(t, params, "mlToonBands(ndl, 3.0, 0.02)", map[string]val{"ndl": {c.ndl}})
		if math.Abs(got-c.want) > 1e-5 {
			t.Errorf("mlToonBands(%v) = %v, want %v", c.ndl, got, c.want)
		}
	}
}

func TestWaterFresnelEndpointsAndSubsurfaceWrap(t *testing.T) {
	const params = "    param n : vec3\n    param v : vec3"
	if f := gameProbe(t, params, "mlWaterFresnel(n, v)", map[string]val{"n": {0, 1, 0}, "v": {0, 1, 0}}); math.Abs(f-0.02) > 1e-6 {
		t.Errorf("head-on water reflects %v, want 0.02", f)
	}
	if f := gameProbe(t, params, "mlWaterFresnel(n, v)", map[string]val{"n": {0, 1, 0}, "v": {1, 0, 0}}); math.Abs(f-1) > 1e-6 {
		t.Errorf("grazing water reflects %v, want 1", f)
	}
	// Wrapped diffuse lights the terminator (n.l = 0) at wrap/(1+wrap).
	if w := gameProbe(t, params, "mlSubsurfaceWrap(n, v, 0.5)", map[string]val{"n": {0, 1, 0}, "v": {1, 0, 0}}); math.Abs(w-1.0/3) > 1e-6 {
		t.Errorf("wrap at the terminator = %v, want 1/3", w)
	}
}

func TestGameModuleIsListed(t *testing.T) {
	src, err := Source(Game)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), "fn mlWindSway(") {
		t.Fatal("game module lacks mlWindSway")
	}
}
