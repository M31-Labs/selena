package lower

import (
	"fmt"

	"m31labs.dev/selena/hir"
	"m31labs.dev/selena/ir"
)

func constDefault(e hir.Expr, want hir.Type) ([]float32, error) {
	got, values, err := constExpr(e)
	if err != nil {
		return nil, err
	}
	wantIR, ok := hirToIRType(want)
	if !ok {
		return nil, fmt.Errorf("unsupported default type %q", want)
	}
	if got != wantIR {
		return nil, fmt.Errorf("got %s, want %s", got, wantIR)
	}
	return values, nil
}

// sunDefault evaluates a constant Sun-record default of the form
// sun(dir, ambient), where dir is a constant vec3 and ambient a constant float.
// It returns the host-packable component values keyed by Sun field name, so the
// lowerer can attach one descriptor default per expanded Sun uniform.
func sunDefault(e hir.Expr) (map[string][]float32, error) {
	call, ok := e.(hir.Call)
	if !ok || call.Func != "sun" {
		return nil, fmt.Errorf("Sun default must be sun(dir, ambient)")
	}
	if len(call.Args) != 2 {
		return nil, fmt.Errorf("sun(dir, ambient) expects 2 arguments, got %d", len(call.Args))
	}
	dirType, dir, err := constExpr(call.Args[0])
	if err != nil {
		return nil, err
	}
	if dirType != ir.Vec3 {
		return nil, fmt.Errorf("sun: dir must be vec3, got %s", dirType)
	}
	ambType, amb, err := constExpr(call.Args[1])
	if err != nil {
		return nil, err
	}
	if ambType != ir.Float {
		return nil, fmt.Errorf("sun: ambient must be float, got %s", ambType)
	}
	return map[string][]float32{"dir": dir, "ambient": amb}, nil
}

func constExpr(e hir.Expr) (ir.Type, []float32, error) {
	switch x := e.(type) {
	case hir.Lit:
		return ir.Float, []float32{float32(x.Value)}, nil
	case hir.Call:
		switch x.Func {
		case "rgb":
			if len(x.Args) != 3 && len(x.Args) != 4 {
				return "", nil, fmt.Errorf("rgb default expects 3 or 4 arguments, got %d", len(x.Args))
			}
			values, err := constFloatArgs(x.Args)
			if err != nil {
				return "", nil, err
			}
			if len(values) == 4 {
				return ir.Vec4, values, nil
			}
			return ir.Vec3, values, nil
		case "vec2", "vec3", "vec4", "mat3", "mat4":
			want := map[string]int{"vec2": 2, "vec3": 3, "vec4": 4, "mat3": 9, "mat4": 16}[x.Func]
			if len(x.Args) != want {
				return "", nil, fmt.Errorf("%s default expects %d arguments, got %d", x.Func, want, len(x.Args))
			}
			values, err := constFloatArgs(x.Args)
			if err != nil {
				return "", nil, err
			}
			switch x.Func {
			case "vec2":
				return ir.Vec2, values, nil
			case "vec3":
				return ir.Vec3, values, nil
			case "vec4":
				return ir.Vec4, values, nil
			case "mat3":
				return ir.Mat3, values, nil
			default:
				return ir.Mat4, values, nil
			}
		default:
			return "", nil, fmt.Errorf("call %q is not allowed in defaults", x.Func)
		}
	case hir.Binary:
		t, values, err := constBinary(x)
		return t, values, err
	default:
		return "", nil, fmt.Errorf("%T is not a constant default expression", e)
	}
}

func constFloatArgs(args []hir.Expr) ([]float32, error) {
	values := make([]float32, len(args))
	for i, a := range args {
		t, v, err := constExpr(a)
		if err != nil {
			return nil, err
		}
		if t != ir.Float {
			return nil, fmt.Errorf("argument %d must be float, got %s", i+1, t)
		}
		values[i] = v[0]
	}
	return values, nil
}

func constBinary(b hir.Binary) (ir.Type, []float32, error) {
	lt, lv, err := constExpr(b.L)
	if err != nil {
		return "", nil, err
	}
	rt, rv, err := constExpr(b.R)
	if err != nil {
		return "", nil, err
	}
	if lt != ir.Float || rt != ir.Float {
		return "", nil, fmt.Errorf("operator %s defaults only support float operands", b.Op)
	}
	switch b.Op {
	case "+":
		return ir.Float, []float32{lv[0] + rv[0]}, nil
	case "-":
		return ir.Float, []float32{lv[0] - rv[0]}, nil
	case "*":
		return ir.Float, []float32{lv[0] * rv[0]}, nil
	case "/":
		if rv[0] == 0 {
			return "", nil, fmt.Errorf("division by zero")
		}
		return ir.Float, []float32{lv[0] / rv[0]}, nil
	default:
		return "", nil, fmt.Errorf("unsupported operator %q", b.Op)
	}
}
