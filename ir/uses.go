package ir

// This file holds the "does this stage use X?" queries the emitters need to
// decide what scaffolding to declare around a body: an extension directive, an
// engine uniform, a host capability requirement. Each emitter used to carry its
// own partial walker, which is how a fragment stage could call fwidth and still
// emit no GL_OES_standard_derivatives directive on some surface kinds. One
// walker in the IR keeps every emitter and the binding descriptor in agreement.

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
// explicit LOD (which requires a mipped scene-color target on the host).
func StageUsesSceneSampleLevel(stage Stage) bool {
	return stageMatches(stage, func(e Expr) bool { _, ok := e.(SceneSampleLevel); return ok })
}

// UsesSceneSize reports whether m's fragment stage reads the backdrop resolution.
func UsesSceneSize(m Module) bool { return StageUsesSceneSize(m.Fragment) }

// UsesSceneSampleLevel reports whether m's fragment stage samples the backdrop
// at an explicit LOD.
func UsesSceneSampleLevel(m Module) bool { return StageUsesSceneSampleLevel(m.Fragment) }

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

func stageMatches(stage Stage, pred func(Expr) bool) bool {
	for _, s := range stage.Body {
		if stmtMatches(s, pred) {
			return true
		}
	}
	return exprMatches(stage.Output, pred)
}

func stmtMatches(s Stmt, pred func(Expr) bool) bool {
	if s.CF == nil {
		return exprMatches(s.Value, pred)
	}
	switch cf := s.CF.(type) {
	case AssignCF:
		return exprMatches(cf.Value, pred)
	case IndexAssignCF:
		return exprMatches(cf.Index, pred) || exprMatches(cf.Value, pred)
	case ReturnCF:
		return exprMatches(cf.Value, pred)
	case IfCF:
		if exprMatches(cf.Cond, pred) {
			return true
		}
		return stmtsMatch(cf.Then, pred) || stmtsMatch(cf.Else, pred)
	case ForCF:
		if exprMatches(cf.Cond, pred) || exprMatches(cf.InitValue, pred) || exprMatches(cf.PostValue, pred) {
			return true
		}
		return stmtsMatch(cf.Body, pred)
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

func exprMatches(e Expr, pred func(Expr) bool) bool {
	if e == nil {
		return false
	}
	if pred(e) {
		return true
	}
	switch x := e.(type) {
	case Call:
		return anyMatch(x.Args, pred)
	case Construct:
		return anyMatch(x.Args, pred)
	case Binary:
		return exprMatches(x.L, pred) || exprMatches(x.R, pred)
	case Unary:
		return exprMatches(x.E, pred)
	case Swizzle:
		return exprMatches(x.E, pred)
	case Sample:
		return exprMatches(x.UV, pred)
	case SampleLevel:
		return exprMatches(x.UV, pred) || exprMatches(x.LOD, pred)
	case SampleCube:
		return exprMatches(x.Dir, pred)
	case SceneSample:
		return exprMatches(x.UV, pred)
	case SceneSampleLevel:
		return exprMatches(x.UV, pred) || exprMatches(x.LOD, pred)
	case StateSampleUV:
		return exprMatches(x.UV, pred)
	case Conditional:
		return exprMatches(x.Cond, pred) || exprMatches(x.Then, pred) || exprMatches(x.Alt, pred)
	case Index:
		return exprMatches(x.Arr, pred) || exprMatches(x.Idx, pred)
	}
	return false
}

func anyMatch(args []Expr, pred func(Expr) bool) bool {
	for _, a := range args {
		if exprMatches(a, pred) {
			return true
		}
	}
	return false
}
