package ir

import "testing"

// TestUsesDerivativesFindsCallInsideReturnCF is the regression test for the
// bug documented in spore.2026-07-27.selena-ir-cf-walker-convention:
// stmtMatches predated ir.ReturnCF and had no case for it, so a derivative
// call packed into an early-return value was invisible to UsesDerivatives.
// stmtMatches now dispatches through StmtCF.exprs/nestedStmts (implemented
// for every variant in this package), so this can no longer regress silently:
// a variant missing those methods fails to compile wherever it is used as a
// StmtCF, rather than falling through this walker unnoticed.
func TestUsesDerivativesFindsCallInsideReturnCF(t *testing.T) {
	m := Module{
		Fragment: Stage{
			Body: []Stmt{
				{CF: IfCF{
					Cond: Ref{Name: "cond"},
					Then: []Stmt{
						{CF: ReturnCF{Value: Call{Func: "fwidth", Args: []Expr{Ref{Name: "uv"}}}}},
					},
				}},
			},
			Output: Ref{Name: "fallback"},
		},
	}
	if !UsesDerivatives(m) {
		t.Fatal("UsesDerivatives = false, want true (fwidth is inside an if's ReturnCF)")
	}
}

// TestUsesDerivativesFindsCallInsideAssignCF exercises the AssignCF case of
// the same walker with a derivative nested one level deeper: inside a for
// loop's body.
func TestUsesDerivativesFindsCallInsideAssignCF(t *testing.T) {
	m := Module{
		Fragment: Stage{
			Body: []Stmt{
				{Target: "acc", Type: Float, Value: Lit{Value: 0}, Mutable: true},
				{CF: ForCF{
					InitTarget: "i", InitType: Int, InitValue: IntLit{Value: 0},
					Cond:       Binary{Op: "<", L: Ref{Name: "i"}, R: IntLit{Value: 4}},
					PostTarget: "i", PostValue: Binary{Op: "+", L: Ref{Name: "i"}, R: IntLit{Value: 1}},
					Body: []Stmt{
						{CF: AssignCF{Target: "acc", Value: Call{Func: "dpdx", Args: []Expr{Ref{Name: "acc"}}}}},
					},
				}},
			},
			Output: Ref{Name: "acc"},
		},
	}
	if !UsesDerivatives(m) {
		t.Fatal("UsesDerivatives = false, want true (dpdx is inside a for body's AssignCF)")
	}
}

// TestUsesDerivativesFalseWithoutDerivative is the negative control for the
// two tests above: a module with no derivative call anywhere must not report
// one.
func TestUsesDerivativesFalseWithoutDerivative(t *testing.T) {
	m := Module{
		Fragment: Stage{
			Body: []Stmt{
				{CF: IfCF{
					Cond: Ref{Name: "cond"},
					Then: []Stmt{
						{CF: ReturnCF{Value: Call{Func: "normalize", Args: []Expr{Ref{Name: "uv"}}}}},
					},
				}},
			},
			Output: Ref{Name: "fallback"},
		},
	}
	if UsesDerivatives(m) {
		t.Fatal("UsesDerivatives = true, want false (no derivative call anywhere)")
	}
}

// TestStageUsesVertexIndexBuiltinFindsRefInsideIf is the direct unit test for
// the walker lower/lower_vertex.go now uses in place of its former hand-rolled
// (and control-flow-blind) irStageUsesVertexIndex/irExprUsesVertexIndex. See
// lower/lower_test.go's TestLowerMeshAuthoredVertexUsesVertexIndexInsideControlFlow
// for the end-to-end regression test through the compiler, and
// validate/validate_test.go's
// TestPreFixVertexIndexInControlFlowWouldHaveFailedNagaValidation for the
// empirical proof of what the old behaviour emitted.
func TestStageUsesVertexIndexBuiltinFindsRefInsideIf(t *testing.T) {
	stage := Stage{
		Body: []Stmt{
			{Target: "fi", Type: Float, Value: Lit{Value: 0}, Mutable: true},
			{CF: IfCF{
				Cond: Ref{Name: "gridSize"},
				Then: []Stmt{
					{CF: AssignCF{Target: "fi", Value: Call{Func: "float", Args: []Expr{Ref{Name: "vertexIndex"}}}}},
				},
			}},
		},
		Output: Ref{Name: "fi"},
	}
	if !StageUsesVertexIndexBuiltin(stage) {
		t.Fatal("StageUsesVertexIndexBuiltin = false, want true (vertexIndex is read inside an if's AssignCF)")
	}
}

// TestStageUsesVertexIndexBuiltinFalseWithoutReference is the negative
// control: a stage that never reads vertexIndex must not report one.
func TestStageUsesVertexIndexBuiltinFalseWithoutReference(t *testing.T) {
	stage := Stage{
		Body: []Stmt{
			{Target: "fi", Type: Float, Value: Lit{Value: 1}},
		},
		Output: Ref{Name: "fi"},
	}
	if StageUsesVertexIndexBuiltin(stage) {
		t.Fatal("StageUsesVertexIndexBuiltin = true, want false (vertexIndex is never referenced)")
	}
}
