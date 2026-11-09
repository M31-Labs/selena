package materiallib

import (
	"fmt"
	"math"

	"m31labs.dev/selena/ir"
)

// A tiny evaluator for the float subset of ir.Expr that a straight-line
// fragment stage uses: scalars and vec2..vec4 held as []float64, every result
// rounded to float32 like a GPU register. It exists so a test can run the IR
// Selena emits for all four backends and compare it with a reference.

type val []float64

func f32(x float64) float64 { return float64(float32(x)) }

func sc32(x float64) val { return val{f32(x)} }

func (v val) at(i int) float64 {
	if len(v) == 1 {
		return v[0]
	}
	return v[i]
}

func width(vs ...val) int {
	n := 1
	for _, v := range vs {
		if len(v) > n {
			n = len(v)
		}
	}
	return n
}

func zip(a, b val, f func(x, y float64) float64) val {
	n := width(a, b)
	out := make(val, n)
	for i := range out {
		out[i] = f32(f(a.at(i), b.at(i)))
	}
	return out
}

func mapv(a val, f func(x float64) float64) val {
	out := make(val, len(a))
	for i, x := range a {
		out[i] = f32(f(x))
	}
	return out
}

func clamp1(x, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, x)) }

// machine evaluates a stage. env holds scalars and vectors; arrays holds
// uniform arrays element by element; depth, when set, answers sampleDepth.
type machine struct {
	env    map[string]val
	arrays map[string][]val
	depth  func(tex string, uv val, layer int) float64
}

func (m machine) run(stage ir.Stage) (val, error) {
	for _, s := range stage.Body {
		if s.CF != nil {
			return nil, fmt.Errorf("control flow is not supported by the test evaluator")
		}
		v, err := m.eval(s.Value)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", s.Target, err)
		}
		m.env[s.Target] = v
	}
	return m.eval(stage.Output)
}

func (m machine) eval(e ir.Expr) (val, error) {
	switch x := e.(type) {
	case ir.Lit:
		return sc32(x.Value), nil
	case ir.IntLit:
		return val{float64(x.Value)}, nil
	case ir.Index:
		ref, ok := x.Arr.(ir.Ref)
		if !ok {
			return nil, fmt.Errorf("index of %T", x.Arr)
		}
		idx, err := m.eval(x.Idx)
		if err != nil {
			return nil, err
		}
		arr, ok := m.arrays[ref.Name]
		i := int(idx[0])
		if !ok || i < 0 || i >= len(arr) {
			return nil, fmt.Errorf("index %s[%d] out of range", ref.Name, i)
		}
		return arr[i], nil
	case ir.SampleDepth:
		if m.depth == nil {
			return nil, fmt.Errorf("sampleDepth without a depth function")
		}
		uv, err := m.eval(x.UV)
		if err != nil {
			return nil, err
		}
		layer := 0
		if x.Layer != nil {
			l, err := m.eval(x.Layer)
			if err != nil {
				return nil, err
			}
			layer = int(l[0])
		}
		return sc32(m.depth(x.Texture, uv, layer)), nil
	case ir.Ref:
		v, ok := m.env[x.Name]
		if !ok {
			return nil, fmt.Errorf("unbound %q", x.Name)
		}
		return v, nil
	case ir.Construct:
		var out val
		for _, a := range x.Args {
			v, err := m.eval(a)
			if err != nil {
				return nil, err
			}
			out = append(out, v...)
		}
		switch x.Type {
		case ir.Int:
			return val{math.Trunc(out[0])}, nil
		case ir.Float:
			return val{out[0]}, nil
		}
		want := map[ir.Type]int{ir.Vec2: 2, ir.Vec3: 3, ir.Vec4: 4}[x.Type]
		if want == 0 {
			return nil, fmt.Errorf("construct %s", x.Type)
		}
		if len(out) == 1 {
			for len(out) < want {
				out = append(out, out[0])
			}
		}
		if len(out) != want {
			return nil, fmt.Errorf("construct %s from %d components", x.Type, len(out))
		}
		return out, nil
	case ir.Swizzle:
		v, err := m.eval(x.E)
		if err != nil {
			return nil, err
		}
		out := make(val, len(x.Field))
		for i, c := range x.Field {
			idx := map[rune]int{'x': 0, 'y': 1, 'z': 2, 'w': 3, 'r': 0, 'g': 1, 'b': 2, 'a': 3}[c]
			out[i] = v[idx]
		}
		return out, nil
	case ir.Unary:
		v, err := m.eval(x.E)
		if err != nil {
			return nil, err
		}
		if x.Op != "-" {
			return nil, fmt.Errorf("unary %q", x.Op)
		}
		return mapv(v, func(a float64) float64 { return -a }), nil
	case ir.Binary:
		l, err := m.eval(x.L)
		if err != nil {
			return nil, err
		}
		r, err := m.eval(x.R)
		if err != nil {
			return nil, err
		}
		switch x.Op {
		case "+":
			return zip(l, r, func(a, b float64) float64 { return a + b }), nil
		case "-":
			return zip(l, r, func(a, b float64) float64 { return a - b }), nil
		case "*":
			return zip(l, r, func(a, b float64) float64 { return a * b }), nil
		case "/":
			return zip(l, r, func(a, b float64) float64 { return a / b }), nil
		case "<=":
			return zip(l, r, func(a, b float64) float64 { return truth(a <= b) }), nil
		case "<":
			return zip(l, r, func(a, b float64) float64 { return truth(a < b) }), nil
		case ">=":
			return zip(l, r, func(a, b float64) float64 { return truth(a >= b) }), nil
		case ">":
			return zip(l, r, func(a, b float64) float64 { return truth(a > b) }), nil
		case "==":
			return zip(l, r, func(a, b float64) float64 { return truth(a == b) }), nil
		case "!=":
			return zip(l, r, func(a, b float64) float64 { return truth(a != b) }), nil
		case "&&":
			return zip(l, r, func(a, b float64) float64 { return truth(a != 0 && b != 0) }), nil
		case "||":
			return zip(l, r, func(a, b float64) float64 { return truth(a != 0 || b != 0) }), nil
		}
		return nil, fmt.Errorf("binary %q", x.Op)
	case ir.Conditional:
		c, err := m.eval(x.Cond)
		if err != nil {
			return nil, err
		}
		if c[0] != 0 {
			return m.eval(x.Then)
		}
		return m.eval(x.Alt)
	case ir.Call:
		args := make([]val, len(x.Args))
		for i, a := range x.Args {
			v, err := m.eval(a)
			if err != nil {
				return nil, err
			}
			args[i] = v
		}
		return call(x.Func, args)
	}
	return nil, fmt.Errorf("unsupported expression %T", e)
}

func call(name string, a []val) (val, error) {
	switch name {
	case "float":
		return a[0], nil
	case "int":
		return mapv(a[0], math.Trunc), nil
	case "acos":
		return mapv(a[0], math.Acos), nil
	case "cos":
		return mapv(a[0], math.Cos), nil
	case "sin":
		return mapv(a[0], math.Sin), nil
	case "abs":
		return mapv(a[0], math.Abs), nil
	case "floor":
		return mapv(a[0], math.Floor), nil
	case "fract":
		return mapv(a[0], func(x float64) float64 { return x - math.Floor(x) }), nil
	case "step":
		return zip(a[0], a[1], func(edge, x float64) float64 {
			if x < edge {
				return 0
			}
			return 1
		}), nil
	case "exp":
		return mapv(a[0], math.Exp), nil
	case "sqrt":
		return mapv(a[0], math.Sqrt), nil
	case "dot":
		s := 0.0
		for i := range a[0] {
			s += a[0][i] * a[1][i]
		}
		return sc32(s), nil
	case "length":
		s := 0.0
		for _, x := range a[0] {
			s += x * x
		}
		return sc32(math.Sqrt(s)), nil
	case "normalize":
		s := 0.0
		for _, x := range a[0] {
			s += x * x
		}
		n := math.Sqrt(s)
		return mapv(a[0], func(x float64) float64 { return x / n }), nil
	case "pow":
		return zip(a[0], a[1], math.Pow), nil
	case "max":
		return zip(a[0], a[1], math.Max), nil
	case "min":
		return zip(a[0], a[1], math.Min), nil
	case "clamp":
		n := width(a[0], a[1], a[2])
		out := make(val, n)
		for i := range out {
			out[i] = f32(clamp1(a[0].at(i), a[1].at(i), a[2].at(i)))
		}
		return out, nil
	case "mix":
		n := width(a[0], a[1], a[2])
		out := make(val, n)
		for i := range out {
			t := a[2].at(i)
			out[i] = f32(a[0].at(i)*(1-t) + a[1].at(i)*t)
		}
		return out, nil
	case "smoothstep":
		n := width(a[0], a[1], a[2])
		out := make(val, n)
		for i := range out {
			t := clamp1((a[2].at(i)-a[0].at(i))/(a[1].at(i)-a[0].at(i)), 0, 1)
			out[i] = f32(t * t * (3 - 2*t))
		}
		return out, nil
	}
	return nil, fmt.Errorf("unsupported call %s", name)
}
