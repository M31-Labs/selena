package ir

// This file holds the "does this stage use X?" queries the emitters need to
// decide what scaffolding to declare around a body: an extension directive, an
// engine uniform, a host capability requirement. Each emitter used to carry its
// own partial walker, which is how a fragment stage could call fwidth and still
// emit no GL_OES_standard_derivatives directive on some surface kinds. One
// walker in the IR keeps every emitter and the binding descriptor in agreement.
//
// stmtMatches and exprMatches dispatch through the StmtCF.exprs/nestedStmts and
// Expr.children methods (implemented per variant below) instead of a type
// switch. A type switch silently returns false for a case it forgot — the
// defect that shipped ir.ReturnCF with no case here and made an early-return
// derivative invisible to UsesDerivatives (see
// spore.2026-07-27.selena-ir-cf-walker-convention). Dispatching through an
// interface method makes that class of omission a compile error instead: a new
// StmtCF or Expr implementation that does not provide these methods cannot be
// assigned to a Stmt.CF or Expr field at all.

// DerivativeBuiltins is the set of screen-space derivative builtins. GLSL ES
// 1.00 (WebGL 1) provides them only through GL_OES_standard_derivatives; GLSL
// ES 3.00, WGSL and Metal have them in core.
var DerivativeBuiltins = map[string]bool{
	"dpdx":   true,
	"dpdy":   true,
	"fwidth": true,
}

// StageCalls reports whether stage's body or output expression contains a call
// to any builtin named in names.
func StageCalls(stage Stage, names map[string]bool) bool {
	for _, s := range stage.Body {
		if stmtCalls(s, names) {
			return true
		}
	}
	return exprCalls(stage.Output, names)
}

// UsesDerivatives reports whether the fragment stage of m calls dpdx, dpdy or
// fwidth. Derivatives are illegal in a vertex shader on every backend — not
// merely unflagged — so lower/resolver.go and lower/typer.go reject them at
// compile time before an ir.Module can ever carry one in Vertex.Body; the
// vertex stage is not scanned here because a derivative can never reach it.
func UsesDerivatives(m Module) bool {
	return StageCalls(m.Fragment, DerivativeBuiltins)
}

// StageUsesSceneSize reports whether stage reads the backdrop resolution.
func StageUsesSceneSize(stage Stage) bool {
	return stageMatches(stage, func(e Expr) bool { _, ok := e.(SceneSize); return ok })
}

// StageUsesSceneSampleLevel reports whether stage samples the backdrop at an
// explicit LOD. WebGL still needs its explicit-LOD extension for level-zero
// taps, even when the host does not need to build scene-color mips.
func StageUsesSceneSampleLevel(stage Stage) bool {
	return stageMatches(stage, func(e Expr) bool { _, ok := e.(SceneSampleLevel); return ok })
}

// StageRequiresSceneColorMips reports whether a stage samples the backdrop at
// an explicit, non-zero or dynamic LOD. A literal level-zero tap emits
// textureSampleLevel, which is valid in non-uniform WGSL control flow, but does
// not need a host-side mip chain.
func StageRequiresSceneColorMips(stage Stage) bool {
	return stageMatches(stage, func(e Expr) bool {
		lvl, ok := e.(SceneSampleLevel)
		return ok && !isLiteralZero(lvl.LOD)
	})
}

// UsesSceneSize reports whether m's fragment stage reads the backdrop resolution.
func UsesSceneSize(m Module) bool { return StageUsesSceneSize(m.Fragment) }

// UsesSceneSampleLevel reports whether m's fragment stage samples the backdrop
// at an explicit LOD.
func UsesSceneSampleLevel(m Module) bool { return StageUsesSceneSampleLevel(m.Fragment) }

// RequiresSceneColorMips reports whether m's fragment stage needs the host to
// provide a scene-color mip chain.
func RequiresSceneColorMips(m Module) bool { return StageRequiresSceneColorMips(m.Fragment) }

// StageUsesVertexIndexBuiltin reports whether stage's body or output
// expression references the vertexIndex builtin, including a reference nested
// inside an if/for/assign/return — anywhere stmtMatches recurses.
//
// This replaces a hand-rolled walker that used to live in
// lower/lower_vertex.go (irStageUsesVertexIndex / irExprUsesVertexIndex). That
// walker only scanned CF-less statements' Value field, so a vertexIndex read
// inside an authored vertex() reassignment, if, or for body was invisible to
// it: UsesVertexIndex came back false, the backend omitted the
// @builtin(vertex_index)/[[vertex_id]]/gl_VertexID wiring the reference
// needed, and the emitted shader referenced vertexIndex as an undeclared
// identifier — naga rejects it outright. Every conformance material happens to
// read vertexIndex from a top-level `let`, so the corpus never exercised the
// gap. Routing through the shared, exhaustive stmtMatches/exprMatches walker
// closes it the same way UsesDerivatives already does for dpdx/dpdy/fwidth.
func StageUsesVertexIndexBuiltin(stage Stage) bool {
	return stageMatches(stage, func(e Expr) bool {
		r, ok := e.(Ref)
		return ok && r.Name == "vertexIndex"
	})
}

func stmtCalls(s Stmt, names map[string]bool) bool {
	return stmtMatches(s, func(e Expr) bool {
		c, ok := e.(Call)
		return ok && names[c.Func]
	})
}

func exprCalls(e Expr, names map[string]bool) bool {
	return exprMatches(e, func(x Expr) bool {
		c, ok := x.(Call)
		return ok && names[c.Func]
	})
}

func isLiteralZero(e Expr) bool {
	lit, ok := e.(Lit)
	return ok && lit.Value == 0
}

func stageMatches(stage Stage, pred func(Expr) bool) bool {
	for _, s := range stage.Body {
		if stmtMatches(s, pred) {
			return true
		}
	}
	return exprMatches(stage.Output, pred)
}

// stmtMatches reports whether s (or, for a control-flow statement, any
// expression it evaluates directly or in a nested statement block) matches
// pred. It dispatches through StmtCF.exprs/nestedStmts (see ir.go) rather than
// a type switch, so a new StmtCF variant is exhaustively handled by
// construction — see the file doc comment.
func stmtMatches(s Stmt, pred func(Expr) bool) bool {
	if s.CF == nil {
		return exprMatches(s.Value, pred)
	}
	for _, e := range s.CF.exprs() {
		if exprMatches(e, pred) {
			return true
		}
	}
	for _, block := range s.CF.nestedStmts() {
		if stmtsMatch(block, pred) {
			return true
		}
	}
	return false
}

func stmtsMatch(stmts []Stmt, pred func(Expr) bool) bool {
	for _, s := range stmts {
		if stmtMatches(s, pred) {
			return true
		}
	}
	return false
}

// exprMatches reports whether e or any expression reachable from e (via
// Expr.children, see ir.go) matches pred. It dispatches through children
// rather than a type switch so a new Expr variant is exhaustively handled by
// construction — see the file doc comment.
func exprMatches(e Expr, pred func(Expr) bool) bool {
	if e == nil {
		return false
	}
	if pred(e) {
		return true
	}
	for _, c := range e.children() {
		if exprMatches(c, pred) {
			return true
		}
	}
	return false
}

// --- StmtCF.exprs / StmtCF.nestedStmts -------------------------------------
//
// Every StmtCF implementation in ir.go must provide both methods (the StmtCF
// interface requires them), even when the answer is "none": that requirement
// is what makes a missing case a compile error instead of a silently-false
// walker result.

func (cf AssignCF) exprs() []Expr         { return []Expr{cf.Value} }
func (cf AssignCF) nestedStmts() [][]Stmt { return nil }

func (cf IndexAssignCF) exprs() []Expr         { return []Expr{cf.Index, cf.Value} }
func (cf IndexAssignCF) nestedStmts() [][]Stmt { return nil }

func (cf ReturnCF) exprs() []Expr         { return []Expr{cf.Value} }
func (cf ReturnCF) nestedStmts() [][]Stmt { return nil }

func (cf IfCF) exprs() []Expr { return []Expr{cf.Cond} }
func (cf IfCF) nestedStmts() [][]Stmt {
	return [][]Stmt{cf.Then, cf.Else}
}

func (cf ForCF) exprs() []Expr {
	return []Expr{cf.InitValue, cf.Cond, cf.PostValue}
}
func (cf ForCF) nestedStmts() [][]Stmt { return [][]Stmt{cf.Body} }

// VarArrayCF declares a local array with no initializer — ElemType and Size
// carry no expression, and it introduces no nested statement block.
func (cf VarArrayCF) exprs() []Expr         { return nil }
func (cf VarArrayCF) nestedStmts() [][]Stmt { return nil }

// DiscardCF carries no payload.
func (cf DiscardCF) exprs() []Expr         { return nil }
func (cf DiscardCF) nestedStmts() [][]Stmt { return nil }

// BreakCF carries no payload.
func (cf BreakCF) exprs() []Expr         { return nil }
func (cf BreakCF) nestedStmts() [][]Stmt { return nil }

// --- Expr.children -----------------------------------------------------
//
// Every Expr implementation in ir.go must provide children (the Expr
// interface requires it), even when the answer is nil (a leaf expression):
// see the note on StmtCF above.

func (Ref) children() []Expr     { return nil }
func (Lit) children() []Expr     { return nil }
func (IntLit) children() []Expr  { return nil }
func (UintLit) children() []Expr { return nil }

func (x Construct) children() []Expr { return x.Args }
func (x Call) children() []Expr      { return x.Args }

func (x Binary) children() []Expr  { return []Expr{x.L, x.R} }
func (x Unary) children() []Expr   { return []Expr{x.E} }
func (x Swizzle) children() []Expr { return []Expr{x.E} }

func (x Sample) children() []Expr      { return []Expr{x.UV} }
func (x SampleLevel) children() []Expr { return []Expr{x.UV, x.LOD} }
func (x SampleCube) children() []Expr  { return []Expr{x.Dir} }

func (x SceneSample) children() []Expr      { return []Expr{x.UV} }
func (x SceneSampleLevel) children() []Expr { return []Expr{x.UV, x.LOD} }
func (SceneSize) children() []Expr          { return nil }

func (x Conditional) children() []Expr { return []Expr{x.Cond, x.Then, x.Alt} }

func (StateSample) children() []Expr     { return nil }
func (x StateSampleUV) children() []Expr { return []Expr{x.UV} }
func (CellUV) children() []Expr          { return nil }

func (x Index) children() []Expr { return []Expr{x.Arr, x.Idx} }
