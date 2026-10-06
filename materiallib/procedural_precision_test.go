package materiallib

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"testing"

	"m31labs.dev/selena"
	"m31labs.dev/selena/hir"
	"m31labs.dev/selena/parse"
)

// Model binary16 arithmetic after each expression, with optional truncation
// and flushed subnormals. A desktop GPU may execute mediump as float32, so a
// WebGL rendering alone cannot exercise the minimum fragment precision.
func mediumpRound(truncate bool) func(float64) float64 {
	return func(x float64) float64 {
		if math.Abs(x) < math.Exp2(-14) {
			return math.Copysign(0, x)
		}
		if math.Abs(x) > math.Exp2(14) {
			return math.Copysign(math.Inf(1), x)
		}
		_, exponent := math.Frexp(x)
		step := math.Exp2(float64(exponent - 11))
		if truncate {
			return math.Trunc(x/step) * step
		}
		return math.RoundToEven(x/step) * step
	}
}

var emittedVector = regexp.MustCompile(`\b(?:vec([234])(?:<f32>)?|float([234]))\(`)
var emittedLocal = regexp.MustCompile(`(?:let|bool|float[234]?|vec[234]) (selenaLib[0-9]+) = ([^;]+);`)
var emittedOutput = regexp.MustCompile(`(?m)^\s*(?:return|gl_FragColor =|fragColor =) ([^;]+);`)

// Read the actual emitted fragment's local expressions and output. Only type
// constructor spelling and the uniform prefix are normalized for the existing
// numerical interpreter; arithmetic operators and builtins remain unchanged.
// This checks target output, not just the common authored helper or IR. It is
// a numerical model, not a substitute for native shader execution/validation.
func emittedProbe(t *testing.T, result selena.Result, target selena.Target) hir.FuncDecl {
	return emittedFunction(t, result, target, "p:vec2")
}

func emittedFunction(t *testing.T, result selena.Result, target selena.Target, parameters string) hir.FuncDecl {
	t.Helper()
	a, ok := result.Artifact(target)
	if !ok {
		t.Fatal("missing artifact", target)
	}
	source := a.Source
	if source == "" {
		source = a.Fragment
	}
	normalize := func(expression string) string {
		expression = strings.ReplaceAll(expression, "u.", "")
		expression = strings.NewReplacer("dFdx(", "dpdx(", "dFdy(", "dpdy(", "dfdx(", "dpdx(", "dfdy(", "dpdy(").Replace(expression)
		return emittedVector.ReplaceAllStringFunc(expression, func(s string) string {
			m := emittedVector.FindStringSubmatch(s)
			return "vec" + m[1] + m[2] + "f("
		})
	}
	locals := emittedLocal.FindAllStringSubmatch(source, -1)
	if len(locals) != len(result.Module.Fragment.Body) {
		t.Fatalf("unrecognized %s locals: found %d, want %d", target, len(locals), len(result.Module.Fragment.Body))
	}
	var body strings.Builder
	for _, local := range locals {
		fmt.Fprintf(&body, "let %s = %s\n", local[1], normalize(local[2]))
	}
	outputs := emittedOutput.FindAllStringSubmatch(source, -1)
	if len(outputs) == 0 {
		t.Fatal("missing fragment output", target)
	}
	fmt.Fprintf(&body, "return %s\n", normalize(outputs[len(outputs)-1][1]))
	p, err := parse.Program([]byte("fn probe(" + parameters + ") -> vec4 {\n" + body.String() + "}\n"))
	if err != nil || len(p.Funcs) != 1 {
		t.Fatalf("parse %s numerical probe: %v", target, err)
	}
	return p.Funcs[0]
}

func compilePrecisionProbe(t *testing.T, name string) selena.Result {
	t.Helper()
	call := name + "(p)"
	value := "rgb(" + call + ",0.0,0.0)"
	if name == "mlHash11" {
		value = "rgb(mlHash11(p.x),0.0,0.0)"
	} else if name == "mlHash22" {
		value = "vec3f(" + call + ",0.0)"
	}
	r, err := Compile([]byte("material PrecisionProbe {param p:vec2 surface(geo)->color {return "+value+"}}"), selena.CompileOptions{}, Procedural)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func moments(values []float64) (mean, variance float64) {
	for _, v := range values {
		mean += v / float64(len(values))
	}
	for _, v := range values {
		variance += (v - mean) * (v - mean) / float64(len(values))
	}
	return
}

func correlation(a, b []float64) float64 {
	ma, va := moments(a)
	mb, vb := moments(b)
	covariance := 0.0
	for i := range a {
		covariance += (a[i] - ma) * (b[i] - mb) / float64(len(a))
	}
	return covariance / math.Sqrt(va*vb)
}

func TestProceduralStatisticsAcrossEmittedTargets(t *testing.T) {
	precision := []struct {
		name  string
		round func(float64) float64
	}{
		{"float32", func(x float64) float64 { return float64(float32(x)) }},
		{"mediump-nearest", mediumpRound(false)},
		{"mediump-truncate", mediumpRound(true)},
	}
	for _, name := range []string{"mlHash11", "mlHash21", "mlHash22", "mlValueNoise2", "mlGradientNoise2", "mlSimplexNoise2", "mlFBm4"} {
		result := compilePrecisionProbe(t, name)
		for _, artifact := range result.Artifacts {
			function := emittedProbe(t, result, artifact.Target)
			for _, mode := range precision {
				t.Run(name+"/"+string(artifact.Target)+"/"+mode.name, func(t *testing.T) {
					r := reference{functions: map[string]hir.FuncDecl{"probe": function}, round: mode.round}
					if strings.HasPrefix(name, "mlHash") {
						checkHashStatistics(t, r, name)
					} else {
						checkNoiseStatistics(t, r, name)
					}
				})
			}
		}
	}
}

func checkHashStatistics(t *testing.T, r reference, name string) {
	t.Helper()
	channels := 1
	if name == "mlHash22" {
		channels = 2
	}
	for _, start := range []float64{0, -32, 240, -512} {
		values := [2][]float64{}
		for y := 0; y < 32; y++ {
			for x := 0; x < 32; x++ {
				p := []float64{start + float64(x), start + float64(y)}
				if name == "mlHash11" {
					p[0] = start + float64(x+32*y)
				}
				v := r.call("probe", p)
				for c := 0; c < channels; c++ {
					if math.IsNaN(v[c]) || v[c] < 0 || v[c] >= 1 {
						t.Fatalf("hash out of range at %v: %v", p, v)
					}
					values[c] = append(values[c], v[c])
				}
			}
		}
		for c := 0; c < channels; c++ {
			mean, variance := moments(values[c])
			near(t, mean, 0.5, 0.02)
			near(t, variance, 1.0/12.0, 0.01)
			bins := [16]int{}
			unique := map[float64]bool{}
			zeros := 0
			for _, v := range values[c] {
				unique[v] = true
				bins[int(v*16)]++
				if v == 0 {
					zeros++
				}
			}
			for _, n := range bins {
				if n < 32 || n > 96 {
					t.Fatalf("biased hash bins at origin %g: %v", start, bins)
				}
			}
			if zeros > 16 {
				t.Fatalf("hash entropy collapsed: %d/1024 zeros", zeros)
			}
			if len(unique) < 128 {
				t.Fatalf("hash has only %d distinct values", len(unique))
			}
			if start == 0 {
				t.Logf("channel %d: mean %.6f variance %.6f zeros %d/1024 bins %v", c, mean, variance, zeros, bins)
			}
		}
		if channels == 2 && math.Abs(correlation(values[0], values[1])) > 0.15 {
			t.Fatal("hash vector channels correlated")
		}
	}
	if name == "mlHash21" {
		var values, horizontal, vertical, diagonal []float64
		for y := 0; y < 64; y++ {
			for x := 0; x < 64; x++ {
				values = append(values, r.call("probe", []float64{float64(x), float64(y)})[0])
				horizontal = append(horizontal, r.call("probe", []float64{float64(x + 1), float64(y)})[0])
				vertical = append(vertical, r.call("probe", []float64{float64(x), float64(y + 1)})[0])
				diagonal = append(diagonal, r.call("probe", []float64{float64(x + 1), float64(y + 1)})[0])
			}
		}
		for _, adjacent := range [][]float64{horizontal, vertical, diagonal} {
			if math.Abs(correlation(values, adjacent)) > 0.1 {
				t.Fatal("adjacent lattice hashes correlated")
			}
		}
		cell := []float64{}
		for _, p := range [][]float64{{4, 2}, {5, 2}, {4, 3}, {5, 3}} {
			cell = append(cell, r.call("probe", p)[0])
		}
		_, variance := moments(cell)
		if variance < 0.01 {
			t.Fatal("regression cell (4,2) has no variation", cell)
		}
	}
}

func TestStandaloneProceduralPatternsStayWithinShaderBudget(t *testing.T) {
	for _, call := range []string{
		"mlWoodGrainAA(p,28.0,8.0)",
		"mlMarbleVeinAA(p,9.0,12.0)",
		"mlFBm4AA(p)",
	} {
		r, err := Compile([]byte("material Budget {param p:vec2 surface(geo)->color {return rgb("+call+",0.0,0.0)}}"), selena.CompileOptions{}, Procedural)
		if err != nil {
			t.Fatal(err)
		}
		for _, a := range r.Artifacts {
			bytes := len(a.Source) + len(a.Vertex) + len(a.Fragment)
			if bytes > 64*1024 {
				t.Fatalf("%s %s emits %d bytes, exceeds 64-KiB budget", call, a.Target, bytes)
			}
			t.Logf("%s %s: %d bytes", call, a.Target, bytes)
		}
	}
}

func checkNoiseStatistics(t *testing.T, r reference, name string) {
	t.Helper()
	var values []float64
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			v := r.call("probe", []float64{float64(x) - 8 + 0.375, float64(y) - 8 + 0.625})[0]
			if math.IsNaN(v) || math.Abs(v) > 1.1 {
				t.Fatal("noise out of range", v)
			}
			values = append(values, v)
		}
	}
	mean, variance := moments(values)
	expectedMean, minimumVariance := 0.0, 0.01
	if name == "mlValueNoise2" {
		expectedMean = 0.5
	} else if name == "mlFBm4" {
		minimumVariance = 0.003
	}
	near(t, mean, expectedMean, 0.1)
	if variance < minimumVariance || variance > 0.3 {
		t.Fatalf("noise variance %g; expected [%g,0.3]", variance, minimumVariance)
	}
	t.Logf("mean %.6f variance %.6f", mean, variance)
}

func TestMediumpSimulationDetectsLegacyHashCollapse(t *testing.T) {
	p, err := parse.Program([]byte(`fn legacy(p:vec2)->float {
let q = fract(vec3f(p.x,p.y,p.x)*0.1031)
let d = dot(q,q.yzx+33.33)
let r = q+d
return fract((r.x+r.y)*r.z)
}`))
	if err != nil {
		t.Fatal(err)
	}
	r := reference{functions: map[string]hir.FuncDecl{"legacy": p.Funcs[0]}, round: mediumpRound(false)}
	zeros := 0
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			if r.call("legacy", []float64{float64(x), float64(y)})[0] == 0 {
				zeros++
			}
		}
	}
	if zeros < 3*1024/4 {
		t.Fatalf("simulation failed to reproduce legacy entropy loss: %d/1024 zeros", zeros)
	}
	t.Logf("legacy hash: %d/1024 zeros", zeros)
}
