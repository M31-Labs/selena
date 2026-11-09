package lower

import (
	"sort"

	"m31labs.dev/selena/hir"
	"m31labs.dev/selena/ir"
)

// Builtins returns the name of every builtin function the lowerer accepts,
// sorted. It includes the statefield reads state(dx, dy) and stateAt(uv),
// which are checked outside the main builtin table. Tooling, such as the
// language server, uses it to keep its documentation complete.
func Builtins() []string {
	names := make([]string, 0, len(stdlib.builtins)+2)
	for name := range stdlib.builtins {
		names = append(names, name)
	}
	names = append(names, "state", "stateAt")
	sort.Strings(names)
	return names
}

// GeometryFields returns the fields surface(geo) can read in a material of
// the given kind, with their types. vertexStage selects the fields an
// authored vertex() reads instead; only mesh materials have one. The result
// is a fresh map.
func GeometryFields(kind hir.Kind, vertexStage bool) map[string]ir.Type {
	var src map[string]geometrySpec
	switch {
	case vertexStage:
		src = meshVertexGeometry
	case kind == hir.KindPoints:
		src = pointGeometry
	case kind == hir.KindPost:
		src = postGeometry
	case kind == hir.KindFeedback:
		src = map[string]geometrySpec{"uv": {typ: ir.Vec2}}
	default:
		src = stdlib.geometry
	}
	out := make(map[string]ir.Type, len(src))
	for name, spec := range src {
		out[name] = spec.typ
	}
	return out
}

// RecordFields returns the fields of a stdlib record param type such as Sun,
// or nil for a type that is not a record.
func RecordFields(record hir.Type) map[string]ir.Type {
	fields := stdlib.recordFields(string(record))
	if fields == nil {
		return nil
	}
	out := make(map[string]ir.Type, len(fields))
	for name, t := range fields {
		out[name] = t
	}
	return out
}
