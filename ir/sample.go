package ir

// DirectionalDiffuse is a hand-built sample material for the first vertical
// slice. It lights a base color with one directional light plus ambient:
//
//	clip   = mvp * vec4(position, 1.0)
//	N      = normalize(normalMatrix * normal)          (vertex -> fragment)
//	diff   = max(dot(N, normalize(lightDir)), 0.0)
//	color  = vec4(baseColor * (ambient + diff), 1.0)
//
// It exercises uniforms of several types, two vertex attributes, one varying,
// matrix*vector, and the normalize/dot/max builtins — enough to pressure-test
// the IR and both emitters before any grammar/front-end exists.
func DirectionalDiffuse() Module {
	return Module{
		Name: "DirectionalDiffuse",
		Uniforms: []Binding{
			{Name: "mvp", Type: Mat4},
			{Name: "normalMatrix", Type: Mat3},
			{Name: "baseColor", Type: Vec3},
			{Name: "lightDir", Type: Vec3},
			{Name: "ambient", Type: Float},
		},
		Attributes: []Binding{
			{Name: "position", Type: Vec3},
			{Name: "normal", Type: Vec3},
		},
		Varyings: []Binding{
			{Name: "vNormal", Type: Vec3},
		},
		Vertex: Stage{
			Body: []Stmt{
				{Target: "vNormal", Type: Vec3, Value: Call{Func: "normalize", Args: []Expr{
					Binary{Op: "*", L: Ref{Name: "normalMatrix"}, R: Ref{Name: "normal"}},
				}}},
			},
			Output: Binary{Op: "*", L: Ref{Name: "mvp"}, R: Construct{Type: Vec4, Args: []Expr{
				Ref{Name: "position"}, Lit{Value: 1.0},
			}}},
		},
		Fragment: Stage{
			Body: []Stmt{
				{Target: "n", Type: Vec3, Value: Call{Func: "normalize", Args: []Expr{Ref{Name: "vNormal"}}}},
				{Target: "diff", Type: Float, Value: Call{Func: "max", Args: []Expr{
					Call{Func: "dot", Args: []Expr{
						Ref{Name: "n"},
						Call{Func: "normalize", Args: []Expr{Ref{Name: "lightDir"}}},
					}},
					Lit{Value: 0.0},
				}}},
				{Target: "lit", Type: Float, Value: Binary{Op: "+", L: Ref{Name: "ambient"}, R: Ref{Name: "diff"}}},
				{Target: "rgb", Type: Vec3, Value: Binary{Op: "*", L: Ref{Name: "baseColor"}, R: Ref{Name: "lit"}}},
			},
			Output: Construct{Type: Vec4, Args: []Expr{Ref{Name: "rgb"}, Lit{Value: 1.0}}},
		},
	}
}
