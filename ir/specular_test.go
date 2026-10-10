package ir

import (
	"reflect"
	"testing"
)

func TestFusedSpecularSharesTypedMathWithoutCapturingStageLocals(t *testing.T) {
	surface := Stage{Body: []Stmt{
		{Target: "coord", Type: Vec2, Value: Ref{Name: "vUv"}},
		{Target: "a", Type: Vec3, Value: Call{Func: "normalize", Args: []Expr{Ref{Name: "coord"}}}},
		{Target: "privateValue", Type: Vec3, Value: Lit{Value: 2}},
	}, Output: Ref{Name: "privateValue"}}
	spec := Stage{Body: []Stmt{
		{Target: "uv", Type: Vec2, Value: Ref{Name: "vUv"}},
		{Target: "renamed", Type: Vec3, Value: Call{Func: "normalize", Args: []Expr{Ref{Name: "uv"}}}},
		{Target: "privateValue", Type: Vec3, Value: Binary{Op: "*", L: Ref{Name: "renamed"}, R: Lit{Value: 3}}},
	}, Output: Ref{Name: "privateValue"}}
	m := Module{Fragment: surface, Specular: &spec, Uniforms: []Binding{{Name: "selenaMRTLocal0", Type: Float}}}
	body, color, companion, ok := FusedSpecular(m)
	if !ok || len(body) != 4 {
		t.Fatalf("fused %v body %d", ok, len(body))
	}
	if color != (Ref{Name: "privateValue"}) || companion != (Ref{Name: "selenaMRTLocal1"}) {
		t.Fatalf("stage-local result capture: %v / %v", color, companion)
	}
	if !reflect.DeepEqual(body[3].Value, Binary{Op: "*", L: Ref{Name: "a"}, R: Lit{Value: 3}}) {
		t.Fatal(body[3])
	}
	if !reflect.DeepEqual(m.Fragment, surface) || !reflect.DeepEqual(*m.Specular, spec) {
		t.Fatal("input stage mutated")
	}
}

func TestFusedSpecularRefusesCoverageOrMutableStages(t *testing.T) {
	for _, stmt := range []Stmt{{Target: "x", Type: Float, Value: Lit{Value: 1}, Mutable: true}, {CF: DiscardCF{}}, {CF: ReturnCF{Value: Lit{Value: 1}}}, {CF: IfCF{Cond: Lit{Value: 1}}}} {
		plain := Stage{Output: Lit{Value: 0}}
		stage := Stage{Body: []Stmt{stmt}, Output: Lit{Value: 1}}
		for _, m := range []Module{{Fragment: stage, Specular: &plain}, {Fragment: plain, Specular: &stage}} {
			if _, _, _, ok := FusedSpecular(m); ok {
				t.Fatalf("unsafe fusion: %+v", stmt)
			}
		}
	}
}
