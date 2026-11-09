package ir

import "testing"

func TestTextureAccessAndSizeTextures(t *testing.T) {
	m := Module{
		Textures: []Texture{{Name: "a"}, {Name: "b"}, {Name: "c"}},
		Vertex: Stage{Output: Construct{Type: Vec4, Args: []Expr{
			Load{Texture: "a", X: IntLit{Value: 0}, Y: IntLit{Value: 0}},
		}}},
		Fragment: Stage{
			Body: []Stmt{
				{Target: "s", Type: Vec2, Value: TextureSize{Texture: "c"}},
				{Target: "v", Type: Vec4, Value: Sample{Texture: "b", UV: Ref{Name: "uv"}}},
			},
			Output: Load{Texture: "b", X: IntLit{Value: 1}, Y: IntLit{Value: 1}},
		},
	}
	sampled, loaded := TextureAccess(m)
	if !sampled["b"] || sampled["a"] || sampled["c"] {
		t.Errorf("sampled = %v", sampled)
	}
	if !loaded["a"] || !loaded["b"] || loaded["c"] {
		t.Errorf("loaded = %v", loaded)
	}
	got := TexSizeTextures(m)
	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Errorf("TexSizeTextures = %v", got)
	}
	if v := StageTexSizeTextures(m, m.Vertex); len(v) != 1 || v[0] != "a" {
		t.Errorf("vertex texel textures = %v", v)
	}
	if v := VertexSampledTextures(m); len(v) != 1 || v[0] != "a" {
		t.Errorf("VertexSampledTextures = %v", v)
	}
}

type bareDialect struct{ Dialect }

func TestPrintLoadWithoutTexelDialectIsAnEmitError(t *testing.T) {
	defer func() {
		ee, ok := recover().(*EmitError)
		if !ok || ee.Code != CodeEmitUnwiredNode {
			t.Fatalf("recovered %v, want an unwired-node EmitError", ee)
		}
	}()
	Print(Load{Texture: "a", X: IntLit{Value: 0}, Y: IntLit{Value: 0}}, bareDialect{})
}
