package lower

import (
	"fmt"

	"m31labs.dev/selena/hir"
	"m31labs.dev/selena/ir"
)

func lowerSpecular(fn *hir.Func, reserved map[string]string, baseRS *resolver, baseTP *typer) (*ir.Stage, error) {
	if fn == nil {
		return nil, nil
	}
	rs, tp := *baseRS, *baseTP
	rs.geo, tp.geo = fn.Geo, fn.Geo
	rs.allowDiscard = false // only surface controls coverage of both attachments
	tp.locals, tp.mutableLocals = map[string]ir.Type{}, map[string]bool{}
	tp.arrayLocals = nil
	rs.tp = &tp
	lc := &lowerCtx{reserved: reserved, rs: &rs, tp: &tp, localKind: "specular local", allowReturn: true, requireRGB: true}
	body, err := lc.lowerStmts(fn.Body)
	if err != nil {
		return nil, fmt.Errorf("specular: %w", err)
	}
	typ, err := tp.typeOf(fn.Result)
	if err != nil {
		return nil, fmt.Errorf("specular result: %w", err)
	}
	if typ != ir.Vec3 {
		return nil, fmt.Errorf("specular must return color/vec3, got %s", typ)
	}
	out, err := rs.expr(fn.Result)
	if err != nil {
		return nil, fmt.Errorf("specular result: %w", err)
	}
	return &ir.Stage{Body: body, Output: ir.Construct{Type: ir.Vec4, Args: []ir.Expr{out, ir.Lit{Value: 1}}}}, nil
}
