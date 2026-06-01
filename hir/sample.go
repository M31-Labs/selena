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

// Textured lights a sampled albedo texture with one directional light. It is the
// M2 sample that exercises the texture binding model (and swizzle + uv interpolant).
//
//	material Textured {
//	    param albedo : texture2d
//	    param light  : Sun
//	    surface(geo) -> color {
//	        let c = sample(albedo, geo.uv).rgb
//	        return c * (light.ambient + max(dot(normalize(geo.worldNormal), light.dir), 0))
//	    }
//	}
func Textured() Material {
	return Material{
		Name: "Textured",
		Params: []Param{
			{Name: "albedo", Type: Texture2D},
			{Name: "light", Type: Sun},
		},
		Surface: Func{
			Geo: "geo",
			Body: []Let{
				{Name: "c", Value: Member{
					E: Call{Func: "sample", Args: []Expr{
						Ref{Name: "albedo"},
						Member{E: Ref{Name: "geo"}, Field: "uv"},
					}},
					Field: "rgb",
				}},
			},
			Result: Binary{Op: "*",
				L: Ref{Name: "c"},
				R: Binary{Op: "+",
					L: Member{E: Ref{Name: "light"}, Field: "ambient"},
					R: Call{Func: "max", Args: []Expr{
						Call{Func: "dot", Args: []Expr{
							Call{Func: "normalize", Args: []Expr{Member{E: Ref{Name: "geo"}, Field: "worldNormal"}}},
							Member{E: Ref{Name: "light"}, Field: "dir"},
						}},
						Lit{Value: 0.0},
					}},
				},
			},
		},
	}
}

// Defaults is a small material that exercises descriptor-backed parameter
// defaults without requiring host-provided material uniforms.
//
//	material Defaults {
//	    param baseColor : color = rgb(0.78, 0.42, 0.98)
//	    param gain : float = 1.0
//	    surface(geo) -> color {
//	        return baseColor * gain
//	    }
//	}
func Defaults() Material {
	return Material{
		Name: "Defaults",
		Params: []Param{
			{Name: "baseColor", Type: Color, Default: Call{Func: "rgb", Args: []Expr{
				Lit{Value: 0.78},
				Lit{Value: 0.42},
				Lit{Value: 0.98},
			}}},
			{Name: "gain", Type: Float, Default: Lit{Value: 1.0}},
		},
		Surface: Func{
			Geo: "geo",
			Result: Binary{Op: "*",
				L: Ref{Name: "baseColor"},
				R: Ref{Name: "gain"},
			},
		},
	}
}
