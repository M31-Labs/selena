package materiallib

import (
	"math"
	"os"
	"strings"
	"testing"

	"m31labs.dev/selena"
	"m31labs.dev/selena/bindings"
	"m31labs.dev/selena/ir"
)

func TestLightingExampleCompilesEveryTarget(t *testing.T) {
	src, err := os.ReadFile("../examples/scene-lit.sel")
	if err != nil {
		t.Fatal(err)
	}
	res, err := Compile(src, selena.CompileOptions{}, Lighting)
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range selena.AllTargets() {
		if _, ok := res.Artifact(target); !ok {
			t.Fatalf("no %s artifact", target)
		}
	}
	fields := map[string]bindings.Field{}
	for _, f := range res.Layout.UniformBlock.Fields {
		fields[f.Name] = f
	}
	for _, name := range []string{"cameraPos", "cameraForward", "ambient", "fog", "lightCount", "lights",
		"shadow0", "shadow1", "shadowSplits0", "shadowSplits1", "shadowMatrices0", "shadowMatrices1"} {
		if fields[name].Source != "engine" || fields[name].Class != "context" {
			t.Errorf("%s: class %q source %q, want an engine context field", name, fields[name].Class, fields[name].Source)
		}
	}
	if fields["lights"].Count != 32 || fields["shadowMatrices0"].Count != 16 {
		t.Fatalf("array counts lights=%d shadowMatrices0=%d", fields["lights"].Count, fields["shadowMatrices0"].Count)
	}
	if fields["albedo"].Source != "" {
		t.Fatal("an author param was marked as an engine input")
	}
	if len(res.Layout.Textures) != 2 {
		t.Fatalf("textures = %+v", res.Layout.Textures)
	}
	for i, tex := range res.Layout.Textures {
		want := []string{"shadowMap0", "shadowMap1"}[i]
		if tex.Name != want || tex.Engine != want || tex.Dimension != "depth2d-array" {
			t.Errorf("texture %d = %+v, want %s as a depth2d-array engine texture", i, tex, want)
		}
	}
	if res.Layout.EngineVersion != bindings.EngineVersion {
		t.Fatalf("engineVersion = %q", res.Layout.EngineVersion)
	}
	wgsl, _ := res.Artifact(selena.TargetWGSL)
	gles, _ := res.Artifact(selena.TargetGLES)
	if !strings.Contains(wgsl.Source, "texture_depth_2d_array") || !strings.Contains(gles.Fragment, "sampler2DArray") {
		t.Fatal("layered shadow maps were not declared as depth arrays")
	}
}

// sceneLight is one light as the GoSX scene IR describes it.
type sceneLight struct {
	kind                    float64
	pos, dir, color, ground [3]float64
	intensity, rng, decay   float64
	angle, penumbra, slot   float64
}

// packLights writes lights in the engine-input layout a renderer uploads.
func packLights(lights []sceneLight) []val {
	out := make([]val, bindings.SceneLightCapacity*bindings.SceneLightVec4s)
	for i := range out {
		out[i] = val{0, 0, 0, 0}
	}
	for i, l := range lights {
		c := [3]float64{l.color[0] * l.intensity, l.color[1] * l.intensity, l.color[2] * l.intensity}
		out[4*i] = val{l.pos[0], l.pos[1], l.pos[2], l.kind}
		out[4*i+1] = val{l.dir[0], l.dir[1], l.dir[2], l.rng}
		out[4*i+2] = val{c[0], c[1], c[2], l.decay}
		e := val{math.Cos(l.angle), math.Cos(l.angle * (1 - l.penumbra)), 0, l.slot}
		if l.kind == bindings.LightHemisphere {
			e = val{l.ground[0] * l.intensity, l.ground[1] * l.intensity, l.ground[2] * l.intensity, l.slot}
		}
		out[4*i+3] = e
	}
	for _, v := range out {
		for j := range v {
			v[j] = f32(v[j])
		}
	}
	return out
}

func norm3(v [3]float64) [3]float64 {
	n := math.Sqrt(v[0]*v[0] + v[1]*v[1] + v[2]*v[2])
	return [3]float64{v[0] / n, v[1] / n, v[2] / n}
}

func dot3(a, b [3]float64) float64 { return a[0]*b[0] + a[1]*b[1] + a[2]*b[2] }

// standardDiffuse is the GoSX standard material's per-light irradiance (the
// WGSL light loop in client/runtime/scene3d/webgpu.ts: pointLightAttenuation,
// spotConeAttenuation, the hemisphere blend and the flat ambient term), before
// albedo.
func standardDiffuse(l sceneLight, n, p [3]float64) [3]float64 {
	scale := func(s float64) [3]float64 {
		return [3]float64{l.color[0] * l.intensity * s, l.color[1] * l.intensity * s, l.color[2] * l.intensity * s}
	}
	switch l.kind {
	case bindings.LightAmbient:
		return scale(1)
	case bindings.LightHemisphere:
		t := n[1]*0.5 + 0.5
		var out [3]float64
		for i := range out {
			out[i] = (l.ground[i]*(1-t) + l.color[i]*t) * l.intensity
		}
		return out
	case bindings.LightDirectional:
		return scale(math.Max(dot3(n, norm3([3]float64{-l.dir[0], -l.dir[1], -l.dir[2]})), 0))
	}
	to := [3]float64{l.pos[0] - p[0], l.pos[1] - p[1], l.pos[2] - p[2]}
	dist := math.Sqrt(dot3(to, to))
	L := norm3(to)
	atten := 1 / math.Max(math.Pow(dist, l.decay), 0.0001)
	if l.rng > 0 {
		ratio := clamp1(1-math.Pow(dist/l.rng, 4), 0, 1)
		atten = ratio * ratio / math.Max(dist*dist, 0.0001)
	}
	if l.kind == bindings.LightSpot {
		d := norm3(l.dir)
		cosAxis := dot3(L, [3]float64{-d[0], -d[1], -d[2]})
		outer, inner := math.Cos(l.angle), math.Cos(l.angle*(1-l.penumbra))
		t := clamp1((cosAxis-outer)/(inner-outer), 0, 1)
		atten *= t * t * (3 - 2*t)
	}
	return scale(atten * math.Max(dot3(n, L), 0))
}

const lightProbeSource = `material LightProbe {
    context {
        lightCount : float
        lights     : array<vec4, 32>
        probeN     : vec3
        probeP     : vec3
        probeI     : float
    }
    surface(geo) -> color {
        return mlLightDiffuse(int(probeI), normalize(probeN), probeP)
    }
}`

// TestLightingMatchesStandardMaterial evaluates the IR all four emitters share
// for mlLightDiffuse and compares it with the GoSX standard material's light
// terms for every light kind. It runs the IR on the CPU in float32; the emitted
// shader text is compile-checked in validate/lighting_test.go.
func TestLightingMatchesStandardMaterial(t *testing.T) {
	res, err := Compile([]byte(lightProbeSource), selena.CompileOptions{Targets: []selena.Target{}}, Lighting)
	if err != nil {
		t.Fatal(err)
	}
	lights := []sceneLight{
		{kind: bindings.LightDirectional, dir: [3]float64{-0.3, -1, 0.2}, color: [3]float64{1, 0.9, 0.8}, intensity: 2, slot: 0},
		{kind: bindings.LightPoint, pos: [3]float64{1, 3, 1}, color: [3]float64{0.2, 0.5, 1}, intensity: 6, decay: 2, slot: -1},
		{kind: bindings.LightPoint, pos: [3]float64{-2, 1, 0}, color: [3]float64{1, 0.3, 0.1}, intensity: 4, rng: 6, decay: 2, slot: -1},
		{kind: bindings.LightSpot, pos: [3]float64{0, 4, 0}, dir: [3]float64{0.1, -1, 0}, color: [3]float64{1, 1, 1}, intensity: 8, decay: 2, angle: 0.6, penumbra: 0.3, slot: 1},
		{kind: bindings.LightHemisphere, color: [3]float64{0.4, 0.6, 1}, ground: [3]float64{0.3, 0.2, 0.1}, intensity: 0.5, slot: -1},
		{kind: bindings.LightAmbient, color: [3]float64{0.1, 0.1, 0.12}, intensity: 1, slot: -1},
	}
	packed := packLights(lights)
	normals := [][3]float64{{0, 1, 0}, {0.3, 0.8, -0.2}, {-0.6, 0.2, 0.7}}
	points := [][3]float64{{0, 0, 0}, {0.5, 0.2, -1}, {-1, 0.5, 1.5}}
	for i, l := range lights {
		for _, n := range normals {
			for _, p := range points {
				got := evalProbe(t, res.Module.Fragment, map[string]val{
					"lightCount": {float64(len(lights))}, "probeN": {n[0], n[1], n[2]},
					"probeP": {p[0], p[1], p[2]}, "probeI": {float64(i)},
				}, map[string][]val{"lights": packed}, nil)
				want := standardDiffuse(l, norm3(n), p)
				for ch := 0; ch < 3; ch++ {
					if math.Abs(got[ch]-want[ch]) > 2e-4+2e-4*math.Abs(want[ch]) {
						t.Fatalf("light %d (kind %v) n=%v p=%v: got %v, want %v", i, l.kind, n, p, got[:3], want)
					}
				}
			}
		}
	}
	// An index at or past lightCount contributes nothing.
	got := evalProbe(t, res.Module.Fragment, map[string]val{
		"lightCount": {2}, "probeN": {0, 1, 0}, "probeP": {0, 0, 0}, "probeI": {3},
	}, map[string][]val{"lights": packed}, nil)
	if got[0] != 0 || got[1] != 0 || got[2] != 0 {
		t.Fatalf("inactive light contributed %v", got)
	}
}

const fogProbeSource = `material FogProbe {
    context {
        cameraPos : vec3
        fog       : vec4
        probeP    : vec3
    }
    surface(geo) -> color {
        return mlFog(vec3f(1.0, 0.0, 0.0), probeP)
    }
}`

// The standard material's fog is exp(-(density * distance)^2) toward the fog
// color.
func TestFogMatchesStandardMaterial(t *testing.T) {
	res, err := Compile([]byte(fogProbeSource), selena.CompileOptions{Targets: []selena.Target{}}, Lighting)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ density, dist float64 }{{0, 50}, {0.02, 10}, {0.05, 30}, {0.3, 2}} {
		got := evalProbe(t, res.Module.Fragment, map[string]val{
			"cameraPos": {0, 0, 0}, "fog": {0, 0, 1, c.density}, "probeP": {0, 0, -c.dist},
		}, nil, nil)
		keep := math.Exp(-(c.density * c.dist) * (c.density * c.dist))
		if math.Abs(got[0]-keep) > 1e-5 || math.Abs(got[2]-(1-keep)) > 1e-5 {
			t.Fatalf("density %v distance %v: got %v, want red %v blue %v", c.density, c.dist, got[:3], keep, 1-keep)
		}
	}
}

const shadowProbeSource = `material ShadowProbe {
    context {
        cameraPos       : vec3
        cameraForward   : vec3
        shadow0         : vec4
        shadowSplits0   : vec4
        shadowMatrices0 : array<vec4, 16>
        shadowMap0      : textureDepth2dArray
        lights          : array<vec4, 32>
        probeP          : vec3
        probeI          : float
    }
    surface(geo) -> color {
        let s = mlShadow0(probeP)
        return vec3f(s, mlLightShadow(int(probeI), 0.25, 0.5), 0.0)
    }
}`

// TestShadowCascadesFilterAndSlots checks cascade selection by view depth, the
// five-tap filter, the out-of-map and empty-slot fallbacks, and mlLightShadow's
// slot routing. Each cascade matrix maps world xy straight to shadow uv and
// world z to depth, so a test can place the occluder exactly.
func TestShadowCascadesFilterAndSlots(t *testing.T) {
	res, err := Compile([]byte(shadowProbeSource), selena.CompileOptions{Targets: []selena.Target{}}, Lighting)
	if err != nil {
		t.Fatal(err)
	}
	matrices := make([]val, 16)
	for c := 0; c < 4; c++ {
		matrices[4*c] = val{1, 0, 0, 0}
		matrices[4*c+1] = val{0, 1, 0, 0}
		matrices[4*c+2] = val{0, 0, 1, 0}
		matrices[4*c+3] = val{0, 0, 0, 1}
	}
	lights := packLights([]sceneLight{{slot: 0}, {slot: 1}, {slot: -1}})
	var layers []int
	// Layer c holds depth 0.5 for uv.x < edge and 1 elsewhere: an occluder at
	// depth 0.5 covering the left part of the map.
	edge := 0.5
	depth := func(tex string, uv val, layer int) float64 {
		if tex != "shadowMap0" {
			t.Fatalf("sampled %s", tex)
		}
		layers = append(layers, layer)
		if uv[0] < edge {
			return 0.5
		}
		return 1
	}
	run := func(p [3]float64, viewDepth float64, shadow val, light float64) val {
		layers = layers[:0]
		return evalProbe(t, res.Module.Fragment, map[string]val{
			"cameraPos": {0, 0, -viewDepth + p[2]}, "cameraForward": {0, 0, 1},
			"shadow0": shadow, "shadowSplits0": {5, 15, 40, 100},
			"probeP": {p[0], p[1], p[2]}, "probeI": {light},
		}, map[string][]val{"shadowMatrices0": matrices, "lights": lights}, depth)
	}
	four := val{4, 0.001, 0.01, 0}

	// Behind the occluder (depth 0.8 > 0.5) and well inside it: shadowed.
	if got := run([3]float64{0.2, 0.5, 0.8}, 3, four, 0); got[0] != 0 {
		t.Fatalf("occluded point lit: %v", got[0])
	}
	// In front of the occluder: lit.
	if got := run([3]float64{0.2, 0.5, 0.3}, 3, four, 0); got[0] != 1 {
		t.Fatalf("unoccluded point shadowed: %v", got[0])
	}
	// Taps sit at x = 0.19, 0.20 (three of them) and 0.21. With the occluder
	// edge at x = 0.195 only the 0.19 tap is occluded, so four of five are lit.
	edge = 0.195
	if got := run([3]float64{0.2, 0.5, 0.8}, 3, four, 0); math.Abs(got[0]-0.8) > 1e-6 {
		t.Fatalf("edge filter = %v, want 0.8 (four of five taps lit)", got[0])
	}
	edge = 0.5
	// Outside the map: lit, whatever the depth says.
	if got := run([3]float64{1.4, 0.5, 0.8}, 3, four, 0); got[0] != 1 {
		t.Fatalf("point outside the map shadowed: %v", got[0])
	}
	// Empty slot: lit.
	if got := run([3]float64{0.2, 0.5, 0.8}, 3, val{0, 0.001, 0.01, -1}, 0); got[0] != 1 {
		t.Fatalf("empty slot shadowed: %v", got[0])
	}
	// Cascade selection follows the splits and the cascade count.
	for _, c := range []struct {
		viewDepth, count float64
		want             int
	}{{3, 4, 0}, {6, 4, 1}, {20, 4, 2}, {60, 4, 3}, {60, 2, 1}, {60, 1, 0}} {
		run([3]float64{0.2, 0.5, 0.8}, c.viewDepth, val{c.count, 0.001, 0.01, 0}, 0)
		for _, layer := range layers {
			if layer != c.want {
				t.Fatalf("view depth %v with %v cascades read layer %d, want %d", c.viewDepth, c.count, layer, c.want)
			}
		}
	}
	// mlLightShadow routes each light to its slot's factor.
	for i, want := range []float64{0.25, 0.5, 1} {
		if got := run([3]float64{0.2, 0.5, 0.3}, 3, four, float64(i)); got[1] != want {
			t.Fatalf("light %d shadow = %v, want %v", i, got[1], want)
		}
	}
}

func evalProbe(t *testing.T, stage ir.Stage, env map[string]val, arrays map[string][]val, depth func(string, val, int) float64) val {
	t.Helper()
	m := machine{env: map[string]val{}, arrays: arrays, depth: depth}
	for k, v := range env {
		c := make(val, len(v))
		for i := range v {
			c[i] = f32(v[i])
		}
		m.env[k] = c
	}
	got, err := m.run(stage)
	if err != nil {
		t.Fatal(err)
	}
	return got
}
