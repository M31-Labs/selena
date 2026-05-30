package hir

// DirectionalDiffuse is the canonical sample authored at the high level. Compare
// its six declarative lines to the LIR + four hand-written shader stages it
// lowers to: no uniform/attribute/varying blocks, no stages, no binding numbers,
// no std140 layout. `geo.worldNormal` is a stdlib-computed interpolant (lowering
// synthesizes the vertex stage that produces it); `light` is a Sun record that
// expands into uniforms; the vec3 result is wrapped to vec4 at output.
//
//	material DirectionalDiffuse {
//	    param baseColor : color
//	    param light     : Sun
//	    surface(geo) -> color {
//	        let n = normalize(geo.worldNormal)
//	        return baseColor * (light.ambient + max(dot(n, light.dir), 0))
//	    }
//	}
func DirectionalDiffuse() Material {
	return Material{
		Name: "DirectionalDiffuse",
		Params: []Param{
			{Name: "baseColor", Type: Color},
			{Name: "light", Type: Sun},
		},
		Surface: Func{
			Geo: "geo",
			Body: []Let{
				{Name: "n", Value: Call{Func: "normalize", Args: []Expr{
					Member{E: Ref{Name: "geo"}, Field: "worldNormal"},
				}}},
			},
			Result: Binary{Op: "*",
				L: Ref{Name: "baseColor"},
				R: Binary{Op: "+",
					L: Member{E: Ref{Name: "light"}, Field: "ambient"},
					R: Call{Func: "max", Args: []Expr{
						Call{Func: "dot", Args: []Expr{
							Ref{Name: "n"},
							Member{E: Ref{Name: "light"}, Field: "dir"},
						}},
						Lit{Value: 0.0},
					}},
				},
			},
		},
	}
}
