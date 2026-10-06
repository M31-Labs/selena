package materiallib

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"m31labs.dev/selena"
	"m31labs.dev/selena/hir"
)

type frameCase struct {
	name        string
	args        [][]float64
	derivatives map[int][2][]float64
	want        []float64
	unit        bool
	exact       bool
	mask        bool
	bounded     bool
}

// Evaluate the actual four emitters, including eager WGSL select operands.
// Observe every expression and rounded builtin intermediate, not just output:
// masking a NaN after a division cannot satisfy the finite-value contract.
func checkFrameCases(t *testing.T, name string, cases []frameCase) {
	t.Helper()
	t.Run(name, func(t *testing.T) { checkFrameHelper(t, name, cases) })
}

func checkFrameHelper(t *testing.T, name string, cases []frameCase) {
	t.Helper()
	functions, err := Functions(Normals, Effects)
	if err != nil {
		t.Fatal(err)
	}
	var f hir.FuncDecl
	for _, candidate := range functions {
		if candidate.Name == name {
			f = candidate
		}
	}
	if f.Name == "" {
		t.Fatal("missing helper", name)
	}
	var declarations, parameters, args []string
	for i, p := range f.Params {
		arg := fmt.Sprintf("arg%d", i)
		declarations = append(declarations, fmt.Sprintf("param %s:%s", arg, p.Type))
		parameters = append(parameters, fmt.Sprintf("%s:%s", arg, p.Type))
		args = append(args, arg)
	}
	value := name + "(" + strings.Join(args, ",") + ")"
	if f.Returns == "float" {
		value = "rgb(" + value + ",0.0,0.0)"
	} else if f.Returns == "vec2" {
		value = "vec3f(" + value + ",0.0)"
	}
	result, err := Compile([]byte("material FrameProbe {"+strings.Join(declarations, "\n")+"\nsurface(geo)->color {return "+value+"}}"), selena.CompileOptions{}, Normals, Effects)
	if err != nil {
		t.Fatal(err)
	}
	precision := []struct {
		name  string
		round func(float64) float64
	}{
		{"float64", nil},
		{"float32", func(x float64) float64 { return float64(float32(x)) }},
		{"mediump-nearest", mediumpRound(false)},
		{"mediump-truncate", mediumpRound(true)},
	}
	for _, artifact := range result.Artifacts {
		function := emittedFunction(t, result, artifact.Target, strings.Join(parameters, ","))
		for _, mode := range precision {
			t.Run(string(artifact.Target)+"/"+mode.name, func(t *testing.T) {
				for _, c := range cases {
					t.Run(c.name, func(t *testing.T) {
						r := reference{functions: map[string]hir.FuncDecl{"probe": function}, round: mode.round,
							derivatives: map[string][2][]float64{}, observe: func(v []float64) {
								for _, x := range v {
									if math.IsNaN(x) || math.IsInf(x, 0) {
										t.Fatalf("nonfinite intermediate: %v", v)
									}
								}
							}}
						for i, a := range c.args {
							d := [2][]float64{make([]float64, len(a)), make([]float64, len(a))}
							if supplied, ok := c.derivatives[i]; ok {
								d = supplied
							}
							r.derivatives[fmt.Sprintf("arg%d", i)] = d
						}
						v := r.call("probe", c.args...)
						for i, want := range c.want {
							tolerance := 0.012
							if c.exact {
								tolerance = 0
							}
							near(t, v[i], want, tolerance)
						}
						if c.unit {
							near(t, math.Sqrt(v[0]*v[0]+v[1]*v[1]+v[2]*v[2]), 1, 0.012)
						}
						if c.mask && (v[0] < 0 || v[0] > 1.012) {
							t.Fatalf("mask outside [0,1]: %v", v)
						}
						if c.bounded && math.Hypot(v[0]-c.args[0][0], v[1]-c.args[0][1]) > math.Max(c.args[4][0], 0)+0.001 {
							t.Fatalf("parallax exceeded offset budget: %v", v)
						}
					})
				}
			})
		}
	}
}

func TestTangentFramesAcrossResolutionAndPrecision(t *testing.T) {
	var cases []frameCase
	for _, resolution := range []float64{128, 512, 2048, 4096, 8192} {
		for _, angle := range []float64{0, math.Pi / 4, math.Pi / 2} {
			for _, mirror := range []float64{-1, 1} {
				d := 1 / resolution
				c, s := math.Cos(angle), math.Sin(angle)
				cases = append(cases, frameCase{
					name: fmt.Sprintf("pixels-%g/rotation-%g/mirror-%g", resolution, angle, mirror),
					args: [][]float64{{0, 0, 1}, {0, 0, 0}, {0, 0}},
					derivatives: map[int][2][]float64{
						1: {{d, 0, 0}, {0, d, 0}}, 2: {{c * d, s * d}, {-mirror * s * d, mirror * c * d}},
					}, want: []float64{c, -mirror * s, 0}, unit: true,
				})
			}
		}
	}
	for _, scale := range []float64{0.25, 1, 4} {
		cases = append(cases, frameCase{name: fmt.Sprintf("mesh-scale-%g/anisotropic-UV", scale),
			args:        [][]float64{{0, 0, 2}, {0, 0, 0}, {0, 0}},
			derivatives: map[int][2][]float64{1: {{scale / 2048, 0, 0}, {0, scale / 512, 0}}, 2: {{0, 1.0 / 2048}, {-2.0 / 512, 0}}},
			want:        []float64{0, -1, 0}, unit: true})
	}
	cases = append(cases, frameCase{name: "degenerate-UV", args: [][]float64{{0, 0, 1}, {0, 0, 0}, {0, 0}}, want: []float64{1, 0, 0}, unit: true})
	// Nearly parallel UV rows are rejected relative to their lengths.
	cases = append(cases, frameCase{name: "relative-degeneracy", args: [][]float64{{0, 0, 1}, {0, 0, 0}, {0, 0}},
		derivatives: map[int][2][]float64{1: {{0.001, 0, 0}, {0, 0.001, 0}}, 2: {{0, 0.001}, {0.0000001, 0.001}}}, want: []float64{1, 0, 0}, unit: true})
	checkFrameCases(t, "mlTangentFromUV", cases)
}

func TestBumpNormalsAcrossResolutionAndPrecision(t *testing.T) {
	var cases []frameCase
	for _, resolution := range []float64{128, 512, 2048, 4096} {
		for _, orientation := range []float64{-1, 1} {
			for _, slope := range []float64{0, 0.5, 2} {
				d := 1 / resolution
				n := math.Sqrt(1 + 2*slope*slope)
				cases = append(cases, frameCase{name: fmt.Sprintf("pixels-%g/orientation-%g/slope-%g", resolution, orientation, slope),
					args:        [][]float64{scalar(0.5), {0, 0, 1}, {0, 0, 0}},
					derivatives: map[int][2][]float64{0: {{orientation * slope * d}, {-slope * d}}, 2: {{orientation * d, 0, 0}, {0, d, 0}}},
					want:        []float64{-slope / n, slope / n, 1 / n}, unit: true})
			}
		}
	}
	cases = append(cases, frameCase{name: "zero-derivatives", args: [][]float64{scalar(0), {0, 1, 0}, {0, 0, 0}}, want: []float64{0, 1, 0}, unit: true})
	checkFrameCases(t, "mlBumpNormal", cases)
}

func TestNormalAndParallaxFiniteAcrossEmittedTargets(t *testing.T) {
	for _, name := range []string{"mlMaxAbs3", "mlSafeNormalize", "mlNormalToWorld"} {
		var cases []frameCase
		for _, v := range [][]float64{{0, 0, 0}, {0.000001, 0, 0}, {1000, -2000, 500}, {0, 0, 1}} {
			c := frameCase{name: fmt.Sprint(v), args: [][]float64{v}}
			if name == "mlSafeNormalize" {
				c.args = append(c.args, []float64{0, 1, 0})
				c.unit = true
				if v[0] == 0 && v[1] == 0 && v[2] == 0 {
					c.want = []float64{0, 1, 0}
				}
			} else if name == "mlNormalToWorld" {
				c.args = append(c.args, []float64{0, 1, 0}, []float64{0, 1, 0}, scalar(-1))
				c.unit = true
				if v[0] == 0 && v[1] == 0 && v[2] == 0 {
					c.want = []float64{0, 1, 0}
				}
			}
			cases = append(cases, c)
		}
		checkFrameCases(t, name, cases)
	}
	for _, name := range []string{"mlPipWellHeight", "mlPipWellGradient", "mlPipWellNormal", "mlPipWellNormalAA"} {
		var cases []frameCase
		for _, radius := range []float64{-1, 0, 0.000001, 0.0001, 0.2} {
			for _, depth := range []float64{0, 0.04, 1} {
				for _, p := range [][]float64{{0, 0}, {radius * 0.5, 0}, {1, -1}} {
					for _, width := range []float64{0, 1.0 / 512} {
						c := frameCase{name: fmt.Sprintf("radius-%g/depth-%g/p-%v/width-%g", radius, depth, p, width),
							args: [][]float64{p, scalar(radius), scalar(depth)}, derivatives: map[int][2][]float64{0: {{width, 0}, {0, width}}},
							unit: strings.Contains(name, "Normal")}
						if radius <= 0 || depth == 0 {
							c.want = []float64{0}
							if c.unit {
								c.want = []float64{0, 0, 1}
							}
							c.exact = true
						}
						cases = append(cases, c)
					}
				}
			}
		}
		checkFrameCases(t, name, cases)
	}
	var cases []frameCase
	for _, height := range []float64{0, 0.000001, 1} {
		for _, limit := range []float64{-1, 0, 0.04} {
			for _, view := range [][]float64{{0, 0, 0}, {1, -1, 0}, {0, 0, 1}} {
				c := frameCase{name: fmt.Sprintf("height-%g/limit-%g/view-%v", height, limit, view), args: [][]float64{{0.5, 0.5}, view, scalar(height), scalar(0.5), scalar(limit)}, bounded: true}
				if height == 0 || limit <= 0 || view[0] == 0 {
					c.want, c.exact = []float64{0.5, 0.5}, true
				}
				cases = append(cases, c)
			}
		}
	}
	checkFrameCases(t, "mlParallaxUV", cases)
	checkFrameCases(t, "mlParallaxUVPhone", []frameCase{{name: "identity", args: [][]float64{{0.5, 0.25}}, want: []float64{0.5, 0.25}, exact: true}})
}

func TestEventMasksFiniteAcrossEmittedTargets(t *testing.T) {
	checkFrameCases(t, "mlSafeLength2", []frameCase{
		{name: "zero", args: [][]float64{{0, 0}}, want: []float64{0}, exact: true},
		{name: "phone-pixel-derivative", args: [][]float64{{1.0 / 512, 0}}, want: []float64{1.0 / 512}, exact: true},
	})
	for _, name := range []string{"mlEventEnvelope", "mlEventGaussian", "mlRadialRippleWidth", "mlRadialRippleAA", "mlDustWidth", "mlDustAA", "mlImpactFlashWidth", "mlImpactFlashAA"} {
		var cases []frameCase
		for _, duration := range []float64{-1, 0, 0.000001, 0.0001, 1} {
			for _, age := range []float64{-1, 0, 0.5, 1, 1000} {
				for _, radius := range []float64{-1, 0, 0.000001, 0.0001, 0.1} {
					for _, width := range []float64{0, 1.0 / 512} {
						if name == "mlEventEnvelope" && (radius != 0.1 || width != 0) {
							continue
						}
						if name == "mlEventGaussian" && (duration != 1 || age != 0) {
							continue
						}
						c := frameCase{name: fmt.Sprintf("duration-%g/age-%g/radius-%g/width-%g", duration, age, radius, width), mask: true}
						p, center := []float64{0.5, 0.5}, []float64{0.5, 0.5}
						switch name {
						case "mlEventEnvelope":
							c.args = [][]float64{scalar(age), scalar(duration), scalar(1)}
						case "mlEventGaussian":
							c.args = [][]float64{{0, 0}, scalar(radius), scalar(width), scalar(4)}
						default:
							c.args = [][]float64{p, center, scalar(age), scalar(1), scalar(duration)}
							if strings.Contains(name, "ImpactFlash") {
								c.args = append(c.args, scalar(radius))
								if radius <= 0 {
									c.want, c.exact = []float64{0}, true
								}
							} else {
								c.args = append(c.args, scalar(0.5), scalar(radius))
								if strings.Contains(name, "Dust") {
									c.args = append(c.args, scalar(32))
								}
							}
							if strings.HasSuffix(name, "Width") {
								c.args = append(c.args, scalar(width))
							} else {
								c.derivatives = map[int][2][]float64{0: {{width, 0}, {0, width}}}
							}
						}
						if name != "mlEventGaussian" && (duration <= 0 || age < 0 || age >= duration) {
							c.want, c.exact = []float64{0}, true
						}
						if name == "mlEventGaussian" && radius <= 0 {
							c.want, c.exact = []float64{0}, true
						}
						cases = append(cases, c)
					}
				}
			}
		}
		if name != "mlEventEnvelope" && name != "mlEventGaussian" {
			for _, intensity := range []float64{-1, 0, 1} {
				for _, p := range [][]float64{{0.5, 0.5}, {0.51, 0.49}, {4, -3}} {
					c := frameCase{name: fmt.Sprintf("translated-p-%v/intensity-%g", p, intensity), mask: true,
						args: [][]float64{p, {0.5, 0.5}, scalar(0.125), scalar(intensity), scalar(0.5)}}
					if strings.Contains(name, "ImpactFlash") {
						c.args = append(c.args, scalar(0.0001))
					} else {
						c.args = append(c.args, scalar(0.5), scalar(0.0001))
						if strings.Contains(name, "Dust") {
							c.args = append(c.args, scalar(32))
						}
					}
					if strings.HasSuffix(name, "Width") {
						c.args = append(c.args, scalar(1.0/512))
					} else {
						c.derivatives = map[int][2][]float64{0: {{1.0 / 512, 0}, {0, 1.0 / 512}}}
					}
					if intensity <= 0 {
						c.want, c.exact = []float64{0}, true
					}
					cases = append(cases, c)
				}
			}
		}
		checkFrameCases(t, name, cases)
	}
}
