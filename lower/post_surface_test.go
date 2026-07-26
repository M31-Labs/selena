package lower

import (
	"errors"
	"strings"
	"testing"

	"m31labs.dev/selena/hir"
	"m31labs.dev/selena/ir"
)

// postMaterial builds a post-kind material whose surface body is stmts and
// whose result is result.
func postMaterial(params []hir.Param, stmts []hir.Stmt, result hir.Expr) hir.Material {
	return hir.Material{
		Name:    "P",
		Kind:    hir.KindPost,
		Params:  params,
		Surface: hir.Func{Geo: "post", Body: stmts, Result: result},
	}
}

func diagCode(t *testing.T, err error) string {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error")
	}
	var de *DiagnosticError
	if !errors.As(err, &de) {
		t.Fatalf("error is not a DiagnosticError: %v", err)
	}
	return de.Code
}

// TestUnknownCallIsUnknownName pins the diagnosis for a callee the language
// does not provide.
//
// The typer used to infer an unregistered call's type from its first argument
// and let the name pass straight through into every emitted backend. Authoring
// `sceneColorLevel(post.uv, 3.0)` before that builtin existed therefore reported
// "surface must return color/vec3/vec4, got vec2" — the type of `post.uv` — and
// a two-argument name that did resolve to a vec4 would have been emitted
// verbatim into WGSL, GLSL, GLES and Metal.
func TestUnknownCallIsUnknownName(t *testing.T) {
	cases := map[string][]hir.Expr{
		"two args": {
			hir.Member{E: hir.Ref{Name: "post"}, Field: "uv"},
			hir.Lit{Value: 3.0},
		},
		"no args": nil,
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			m := postMaterial(nil, nil, hir.Call{Func: "notABuiltin", Args: args})
			_, _, err := Lower(m)
			if code := diagCode(t, err); code != CodeUnknownName {
				t.Fatalf("code = %s, want %s (%v)", code, CodeUnknownName, err)
			}
			if !strings.Contains(err.Error(), `unknown function "notABuiltin"`) {
				t.Fatalf("message does not name the callee: %v", err)
			}
		})
	}
}

// TestSceneColorLevelLowersToSceneSampleLevel covers the backdrop LOD tap: post
// materials cannot declare texture2d params, so sampleLevel() can never name
// sceneColor and this is the only route to a pre-filtered backdrop.
func TestSceneColorLevelLowersToSceneSampleLevel(t *testing.T) {
	m := postMaterial(nil, nil, hir.Call{Func: "sceneColorLevel", Args: []hir.Expr{
		hir.Member{E: hir.Ref{Name: "post"}, Field: "uv"},
		hir.Lit{Value: 3.0},
	}})
	mod, layout, err := Lower(m)
	if err != nil {
		t.Fatal(err)
	}
	lvl, ok := mod.Fragment.Output.(ir.SceneSampleLevel)
	if !ok {
		t.Fatalf("fragment output = %T, want ir.SceneSampleLevel", mod.Fragment.Output)
	}
	if lvl.Name != "sceneColor" {
		t.Fatalf("sampled target = %q, want sceneColor", lvl.Name)
	}
	if !layout.Requires.SceneColorMips {
		t.Fatal("Requires.SceneColorMips = false; a host with no mip chain renders this unblurred")
	}
}

// TestSceneColorLevelArgumentTypes keeps the signature honest.
func TestSceneColorLevelArgumentTypes(t *testing.T) {
	uv := hir.Member{E: hir.Ref{Name: "post"}, Field: "uv"}
	cases := map[string][]hir.Expr{
		"lod must be float": {uv, hir.Member{E: hir.Ref{Name: "post"}, Field: "uv"}},
		"uv must be vec2":   {hir.Lit{Value: 1.0}, hir.Lit{Value: 3.0}},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			m := postMaterial(nil, nil, hir.Call{Func: "sceneColorLevel", Args: args})
			if _, _, err := Lower(m); err == nil {
				t.Fatal("expected a type error")
			}
		})
	}
	t.Run("arity", func(t *testing.T) {
		m := postMaterial(nil, nil, hir.Call{Func: "sceneColorLevel", Args: []hir.Expr{uv}})
		if code := diagCode(t, mustErr(t, m)); code != CodeInvalidCall {
			t.Fatalf("code = %s, want %s", code, CodeInvalidCall)
		}
	})
}

// TestSceneSizeLowersToSceneSize covers the backdrop-resolution query, and the
// GLSL ES 1.00 uniform it implies (that dialect has no textureSize).
func TestSceneSizeLowersToSceneSize(t *testing.T) {
	m := postMaterial(nil,
		[]hir.Stmt{hir.Let{Name: "s", Value: hir.Call{Func: "sceneSize"}}},
		hir.Call{Func: "rgb", Args: []hir.Expr{
			hir.Member{E: hir.Ref{Name: "s"}, Field: "x"},
			hir.Member{E: hir.Ref{Name: "s"}, Field: "y"},
			hir.Lit{Value: 0.0},
		}},
	)
	mod, layout, err := Lower(m)
	if err != nil {
		t.Fatal(err)
	}
	if len(mod.Fragment.Body) != 1 {
		t.Fatalf("fragment body = %+v", mod.Fragment.Body)
	}
	if _, ok := mod.Fragment.Body[0].Value.(ir.SceneSize); !ok {
		t.Fatalf("let value = %T, want ir.SceneSize", mod.Fragment.Body[0].Value)
	}
	if mod.Fragment.Body[0].Type != ir.Vec2 {
		t.Fatalf("sceneSize() type = %s, want vec2", mod.Fragment.Body[0].Type)
	}
	if layout.Requires.GLSceneSizeUniform != "_sceneSize" {
		t.Fatalf("Requires.GLSceneSizeUniform = %q, want _sceneSize", layout.Requires.GLSceneSizeUniform)
	}
}

// TestSceneSizeTakesNoArguments guards the arity message; a bare unknown name
// used to be reported as `call "sceneSize" has no arguments`.
func TestSceneSizeTakesNoArguments(t *testing.T) {
	m := postMaterial(nil,
		[]hir.Stmt{hir.Let{Name: "s", Value: hir.Call{Func: "sceneSize", Args: []hir.Expr{hir.Lit{Value: 1.0}}}}},
		hir.Call{Func: "rgb", Args: []hir.Expr{
			hir.Member{E: hir.Ref{Name: "s"}, Field: "x"},
			hir.Member{E: hir.Ref{Name: "s"}, Field: "y"},
			hir.Lit{Value: 0.0},
		}},
	)
	if code := diagCode(t, mustErr(t, m)); code != CodeInvalidCall {
		t.Fatalf("code = %s, want %s", code, CodeInvalidCall)
	}
}

// TestDerivativesDeclareGLExtension is the lowering half of the WebGL
// derivative contract: the emitted `#extension` directive only takes effect
// when the host has already called gl.getExtension for it, so the descriptor
// must name the extension.
func TestDerivativesDeclareGLExtension(t *testing.T) {
	m := postMaterial(nil,
		[]hir.Stmt{hir.Let{Name: "w", Value: hir.Call{Func: "fwidth", Args: []hir.Expr{
			hir.Member{E: hir.Member{E: hir.Ref{Name: "post"}, Field: "uv"}, Field: "x"},
		}}}},
		hir.Call{Func: "rgb", Args: []hir.Expr{
			hir.Ref{Name: "w"}, hir.Ref{Name: "w"}, hir.Ref{Name: "w"},
		}},
	)
	_, layout, err := Lower(m)
	if err != nil {
		t.Fatal(err)
	}
	if len(layout.Requires.GLExtensions) != 1 || layout.Requires.GLExtensions[0] != "OES_standard_derivatives" {
		t.Fatalf("Requires.GLExtensions = %v", layout.Requires.GLExtensions)
	}
}

// TestSampleLevelOnBackdropNamesTheRightBuiltin covers the other spelling an
// author reaches for. sampleLevel() only accepts a texture2d param and post
// materials cannot declare one, so `sampleLevel(sceneColor, uv, lod)` is a dead
// end; the message has to point at sceneColorLevel rather than at a param type
// the kind forbids.
func TestSampleLevelOnBackdropNamesTheRightBuiltin(t *testing.T) {
	m := postMaterial(nil, nil, hir.Call{Func: "sampleLevel", Args: []hir.Expr{
		hir.Ref{Name: "sceneColor"},
		hir.Member{E: hir.Ref{Name: "post"}, Field: "uv"},
		hir.Lit{Value: 3.0},
	}})
	err := mustErr(t, m)
	if !strings.Contains(err.Error(), "sceneColorLevel(uv, lod)") {
		t.Fatalf("message does not name sceneColorLevel: %v", err)
	}
}

// TestFloatArrayIndexNamesTheIntLiteralSuffix keeps the fix in the message. A
// bare `0` is a float literal, so `for (var i = 0; ...)` yields a float counter
// and `rects[i]` is ill-typed — with nothing in the old message to reveal that
// `0i` exists.
func TestFloatArrayIndexNamesTheIntLiteralSuffix(t *testing.T) {
	m := postMaterial(
		[]hir.Param{{Name: "rects", Type: hir.Vec4, IsArray: true, ArraySize: 4}},
		[]hir.Stmt{
			hir.VarDecl{Name: "i", Value: hir.Lit{Value: 0}},
			hir.Let{Name: "r", Value: hir.IndexExpr{Arr: hir.Ref{Name: "rects"}, Index: hir.Ref{Name: "i"}}},
		},
		hir.Call{Func: "rgb", Args: []hir.Expr{
			hir.Member{E: hir.Ref{Name: "r"}, Field: "x"},
			hir.Member{E: hir.Ref{Name: "r"}, Field: "y"},
			hir.Member{E: hir.Ref{Name: "r"}, Field: "z"},
		}},
	)
	err := mustErr(t, m)
	if !strings.Contains(err.Error(), "0i") {
		t.Fatalf("message does not name the int-literal suffix: %v", err)
	}
	if !strings.Contains(err.Error(), "int(i)") {
		t.Fatalf("message does not offer the conversion: %v", err)
	}
}

// TestIntLiteralCounterIndexesArrayDirectly is the positive control: the
// documented spelling works, on a post material, with break inside the loop.
func TestIntLiteralCounterIndexesArrayDirectly(t *testing.T) {
	m := postMaterial(
		[]hir.Param{{Name: "rects", Type: hir.Vec4, IsArray: true, ArraySize: 4}},
		[]hir.Stmt{
			hir.VarDecl{Name: "best", Value: hir.Lit{Value: 1.0}},
			hir.For{
				InitName:  "i",
				InitValue: hir.IntLit{Value: 0},
				Cond:      hir.Binary{Op: "<", L: hir.Ref{Name: "i"}, R: hir.IntLit{Value: 4}},
				PostName:  "i",
				PostValue: hir.Binary{Op: "+", L: hir.Ref{Name: "i"}, R: hir.IntLit{Value: 1}},
				Body: []hir.Stmt{
					hir.Let{Name: "r", Value: hir.IndexExpr{Arr: hir.Ref{Name: "rects"}, Index: hir.Ref{Name: "i"}}},
					hir.Assign{Name: "best", Value: hir.Call{Func: "min", Args: []hir.Expr{
						hir.Ref{Name: "best"},
						hir.Member{E: hir.Ref{Name: "r"}, Field: "x"},
					}}},
					hir.Break{},
				},
			},
		},
		hir.Call{Func: "rgb", Args: []hir.Expr{
			hir.Ref{Name: "best"}, hir.Ref{Name: "best"}, hir.Ref{Name: "best"},
		}},
	)
	if _, _, err := Lower(m); err != nil {
		t.Fatal(err)
	}
}

func mustErr(t *testing.T, m hir.Material) error {
	t.Helper()
	_, _, err := Lower(m)
	if err == nil {
		t.Fatal("expected an error")
	}
	return err
}
