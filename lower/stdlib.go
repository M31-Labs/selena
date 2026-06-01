package lower

import (
	"m31labs.dev/selena/ir"
)

type stdlibRegistry struct {
	records  map[string]recordSpec
	geometry map[string]geometrySpec
}

type recordSpec struct {
	fields map[string]ir.Type
}

type geometrySpec struct {
	typ     ir.Type
	varying string
	attrs   []string
	build   func() ir.Expr
}

var stdlib = stdlibRegistry{
	records: map[string]recordSpec{
		"Sun": {
			fields: map[string]ir.Type{
				"ambient": ir.Float,
				"dir":     ir.Vec3,
			},
		},
	},
	geometry: map[string]geometrySpec{
		"position": {
			typ:     ir.Vec3,
			varying: "vPosition",
			attrs:   []string{"position"},
			build:   func() ir.Expr { return ir.Ref{Name: "position"} },
		},
		"normal": {
			typ:     ir.Vec3,
			varying: "vNormal",
			attrs:   []string{"normal"},
			build:   func() ir.Expr { return ir.Ref{Name: "normal"} },
		},
		"uv": {
			typ:     ir.Vec2,
			varying: "vUv",
			attrs:   []string{"uv"},
			build:   func() ir.Expr { return ir.Ref{Name: "uv"} },
		},
		"worldNormal": {
			typ:     ir.Vec3,
			varying: "vWorldNormal",
			attrs:   []string{"normal"},
			build: func() ir.Expr {
				return ir.Call{Func: "normalize", Args: []ir.Expr{
					ir.Binary{Op: "*", L: ir.Ref{Name: "normalMatrix"}, R: ir.Ref{Name: "normal"}},
				}}
			},
		},
	},
}

func (r stdlibRegistry) recordField(record, field string) (ir.Type, bool) {
	spec, ok := r.records[record]
	if !ok {
		return "", false
	}
	t, ok := spec.fields[field]
	return t, ok
}

func (r stdlibRegistry) recordFields(record string) map[string]ir.Type {
	spec, ok := r.records[record]
	if !ok {
		return nil
	}
	return spec.fields
}

func (r stdlibRegistry) geometryField(name string) (geometrySpec, bool) {
	spec, ok := r.geometry[name]
	return spec, ok
}
