package materiallib

import (
	"fmt"
	"math"
	"testing"

	"m31labs.dev/selena/hir"
)

// Evaluate the authored functions for numerical invariants. Shader compilation
// is checked separately; this interpreter does not replace GPU validation.
type reference struct {
	functions map[string]hir.FuncDecl
	// Optional per-expression rounding models shader arithmetic precision.
	round func(float64) float64
}

func referenceLibrary(t *testing.T) reference {
	t.Helper()
	functions, err := Functions(modules(t)...)
	if err != nil {
		t.Fatal(err)
	}
	r := reference{functions: map[string]hir.FuncDecl{}}
	for _, f := range functions {
		r.functions[f.Name] = f
	}
	return r
}

func (r reference) call(name string, args ...[]float64) []float64 {
	f, ok := r.functions[name]
	if !ok {
		panic("unknown reference helper " + name)
	}
	env := map[string][]float64{}
	for i, p := range f.Params {
		env[p.Name] = args[i]
	}
	for _, l := range f.Body {
		env[l.Name] = r.eval(l.Value, env)
	}
	return r.eval(f.Result, env)
}

func scalar(x float64) []float64 { return []float64{x} }
func lane(v []float64, i int) float64 {
	if len(v) == 1 {
		return v[0]
	}
	return v[i]
}
func truth(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func (r reference) rounded(x float64) float64 {
	if r.round != nil {
		return r.round(x)
	}
	return x
}

func (r reference) eval(e hir.Expr, env map[string][]float64) (result []float64) {
	defer func() {
		if r.round != nil {
			result = append([]float64(nil), result...)
			for i, v := range result {
				result[i] = r.round(v)
			}
		}
	}()
	switch x := e.(type) {
	case hir.Lit:
		return scalar(x.Value)
	case hir.Ref:
		v, ok := env[x.Name]
		if !ok {
			panic("unknown ref " + x.Name)
		}
		return v
	case hir.Member:
		v := r.eval(x.E, env)
		out := make([]float64, len(x.Field))
		for i, c := range x.Field {
			for j, k := range "xyzw" {
				if c == k {
					out[i] = v[j]
				}
			}
			for j, k := range "rgba" {
				if c == k {
					out[i] = v[j]
				}
			}
		}
		return out
	case hir.Unary:
		v := r.eval(x.E, env)
		out := make([]float64, len(v))
		for i, n := range v {
			if x.Op == "-" {
				out[i] = -n
			} else {
				out[i] = truth(n == 0)
			}
		}
		return out
	case hir.Conditional:
		if r.eval(x.Cond, env)[0] != 0 {
			return r.eval(x.Then, env)
		}
		return r.eval(x.Alt, env)
	case hir.Binary:
		a, b := r.eval(x.L, env), r.eval(x.R, env)
		out := make([]float64, max(len(a), len(b)))
		for i := range out {
			u, v := lane(a, i), lane(b, i)
			switch x.Op {
			case "+":
				out[i] = u + v
			case "-":
				out[i] = u - v
			case "*":
				out[i] = u * v
			case "/":
				out[i] = u / v
			case ">":
				out[i] = truth(u > v)
			case "<":
				out[i] = truth(u < v)
			case ">=":
				out[i] = truth(u >= v)
			case "<=":
				out[i] = truth(u <= v)
			case "==":
				out[i] = truth(u == v)
			case "!=":
				out[i] = truth(u != v)
			case "&&":
				out[i] = truth(u != 0 && v != 0)
			case "||":
				out[i] = truth(u != 0 || v != 0)
			default:
				panic(x.Op)
			}
		}
		return out
	case hir.Call:
		args := make([][]float64, len(x.Args))
		width := 1
		for i, a := range x.Args {
			args[i] = r.eval(a, env)
			width = max(width, len(args[i]))
		}
		if _, ok := r.functions[x.Func]; ok {
			return r.call(x.Func, args...)
		}
		switch x.Func {
		case "rgb", "vec2f", "vec3f", "vec4f":
			var out []float64
			for _, a := range args {
				out = append(out, a...)
			}
			if len(out) == 1 && x.Func != "rgb" {
				out = make([]float64, int(x.Func[3]-'0'))
				for i := range out {
					out[i] = args[0][0]
				}
			}
			return out
		case "dot":
			total := 0.0
			for i, a := range args[0] {
				total = r.rounded(total + r.rounded(a*args[1][i]))
			}
			return scalar(total)
		case "length", "normalize":
			total := 0.0
			for _, a := range args[0] {
				total += a * a
			}
			n := math.Sqrt(total)
			if x.Func == "length" {
				return scalar(n)
			}
			out := make([]float64, len(args[0]))
			for i, a := range args[0] {
				out[i] = a / n
			}
			return out
		case "cross":
			a, b := args[0], args[1]
			return []float64{a[1]*b[2] - a[2]*b[1], a[2]*b[0] - a[0]*b[2], a[0]*b[1] - a[1]*b[0]}
		}
		out := make([]float64, width)
		for i := range out {
			a := lane(args[0], i)
			b, c := 0.0, 0.0
			if len(args) > 1 {
				b = lane(args[1], i)
			}
			if len(args) > 2 {
				c = lane(args[2], i)
			}
			switch x.Func {
			case "abs":
				out[i] = math.Abs(a)
			case "sqrt":
				out[i] = math.Sqrt(a)
			case "sin":
				out[i] = math.Sin(a)
			case "cos":
				out[i] = math.Cos(a)
			case "tan":
				out[i] = math.Tan(a)
			case "floor":
				out[i] = math.Floor(a)
			case "fract":
				out[i] = a - math.Floor(a)
			case "exp":
				out[i] = math.Exp(a)
			case "min":
				out[i] = math.Min(a, b)
			case "max":
				out[i] = math.Max(a, b)
			case "pow":
				out[i] = math.Pow(a, b)
			case "clamp":
				out[i] = math.Max(b, math.Min(c, a))
			case "mix":
				out[i] = a*(1-c) + b*c
			case "select":
				out[i] = a
				if c != 0 {
					out[i] = b
				}
			case "step":
				out[i] = truth(b >= a)
			case "smoothstep":
				q := math.Max(0, math.Min(1, (c-a)/(b-a)))
				out[i] = q * q * (3 - 2*q)
			case "atan2":
				out[i] = math.Atan2(a, b)
			case "mod":
				out[i] = a - b*math.Floor(a/b)
			case "sign":
				out[i] = truth(a > 0) - truth(a < 0)
			default:
				panic(fmt.Sprintf("reference builtin %s", x.Func))
			}
		}
		return out
	}
	panic(fmt.Sprintf("reference expression %T", e))
}

func near(t *testing.T, got, want, tolerance float64) {
	t.Helper()
	if math.IsNaN(got) || math.IsInf(got, 0) || math.Abs(got-want) > tolerance {
		t.Fatalf("got %g, want %g ± %g", got, want, tolerance)
	}
}

func TestBRDFNumericalInvariants(t *testing.T) {
	r := referenceLibrary(t)
	f0 := []float64{0.04, 0.3, 1}
	for i, v := range r.call("mlFresnelSchlick", scalar(1), f0) {
		near(t, v, f0[i], 1e-12)
	}
	for _, v := range r.call("mlFresnelSchlick", scalar(0), f0) {
		near(t, v, 1, 1e-12)
	}
	for i, v := range r.call("mlGGXEnergyCompensation", f0, scalar(0.5)) {
		near(t, v, 1+f0[i], 1e-12)
	}
	for _, rough := range []float64{0, 0.05, 0.3, 1} {
		for _, nv := range []float64{0, 0.000001, 0.5, 1} {
			for _, v := range r.call("mlGGX", scalar(nv), scalar(0), scalar(1), scalar(1), scalar(rough), f0, scalar(0.5)) {
				near(t, v, 0, 1e-12)
			}
		}
	}
	base := []float64{0.2, 0.4, 0.6}
	for i, v := range r.call("mlClearCoat", base, scalar(0.5), scalar(0.8), scalar(0.7), scalar(0.9), scalar(0.3), scalar(0)) {
		near(t, v, base[i], 1e-12)
	}
	for _, rough := range []float64{0.3, 0.6, 1} {
		const count = 20000
		for _, name := range []string{"mlGGXD", "mlCharlieD"} {
			total := 0.0
			for i := 0; i < count; i++ {
				c := (float64(i) + 0.5) / count
				total += r.call(name, scalar(c), scalar(rough))[0] * c * 2 * math.Pi / count
			}
			near(t, total, 1, 0.0001)
		}
		h := []float64{0.6, 0, 0.8}
		near(t, r.call("mlAnisotropicGGXD", h, scalar(rough*rough), scalar(rough*rough))[0], r.call("mlGGXD", scalar(h[2]), scalar(rough))[0], 1e-10)
	}
	near(t, r.call("mlThinTransmission", scalar(-1), scalar(1), scalar(1))[0], 0, 1e-12)
}
