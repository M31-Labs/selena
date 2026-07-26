package lower

import (
	"errors"
	"strings"
	"testing"

	"m31labs.dev/selena/emit/gles"
	"m31labs.dev/selena/emit/glsl"
	"m31labs.dev/selena/emit/metal"
	"m31labs.dev/selena/emit/wgsl"
	"m31labs.dev/selena/hir"
	"m31labs.dev/selena/ir"
	"m31labs.dev/selena/parse"
)

func TestLowerDirectionalDiffuse(t *testing.T) {
	mod, layout, err := Lower(hir.DirectionalDiffuse())
	if err != nil {
		t.Fatal(err)
	}

	// --- binding layout: a correct std140 block, 144 bytes ---
	if layout.UniformBlock.Size != 144 {
		t.Errorf("uniform block size = %d, want 144", layout.UniformBlock.Size)
	}
	off := map[string]int{}
	for _, f := range layout.UniformBlock.Fields {
		off[f.Name] = f.Offset
	}
	for name, want := range map[string]int{"mvp": 0, "normalMatrix": 64, "baseColor": 112} {
		if off[name] != want {
			t.Errorf("uniform %s offset = %d, want %d", name, off[name], want)
		}
	}
	if _, ok := off["light_dir"]; !ok {
		t.Error("Sun param did not expand into a light_dir uniform")
	}
	if len(layout.Attributes) != 2 || layout.Attributes[0].Name != "position" || layout.Attributes[0].Location != 0 {
		t.Errorf("attributes = %+v, want position@0 + normal@1", layout.Attributes)
	}

	// --- interpolant inference: worldNormal became a synthesized varying ---
	if len(mod.Varyings) != 1 || mod.Varyings[0].Name != "vWorldNormal" {
		t.Errorf("varyings = %+v, want [vWorldNormal]", mod.Varyings)
	}
	if len(mod.Vertex.Body) != 1 {
		t.Fatalf("vertex body = %d stmts, want 1 (the synthesized worldNormal)", len(mod.Vertex.Body))
	}

	// --- emit WGSL from the lowered LIR and check the key constructs ---
	src, err := wgsl.Emit(mod)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"out.vWorldNormal = normalize((u.normalMatrix * in.normal));",
		"out.position = (u.mvp * vec4<f32>(in.position, 1.0));",
		"let n = normalize(in.vWorldNormal);",
		"u.light_dir",
		"u.light_ambient",
		"return vec4<f32>((u.baseColor * ",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("lowered WGSL missing %q\n--- got ---\n%s", want, src)
		}
	}
}

// TestLowerRejectsVertexHookOnPoints checks that the non-mesh kinds still reject
// author vertex() hooks (only mesh/general supports them as of B4).
func TestLowerRejectsVertexHookOnPoints(t *testing.T) {
	_, _, err := Lower(hir.Material{
		Name: "Bad",
		Kind: hir.KindPoints,
		Vertex: &hir.Func{
			Span:   hir.Span{Start: hir.Position{Line: 2, Column: 5}},
			Geo:    "geo",
			Result: hir.Call{Func: "vec4f", Args: []hir.Expr{hir.Lit{Value: 0}, hir.Lit{Value: 0}, hir.Lit{Value: 0}, hir.Lit{Value: 1}}},
		},
		Surface: hir.Func{
			Geo:    "geo",
			Result: hir.Call{Func: "rgb", Args: []hir.Expr{hir.Lit{Value: 1}, hir.Lit{Value: 1}, hir.Lit{Value: 1}}},
		},
	})
	if err == nil {
		t.Fatal("Lower succeeded, want unsupported feature diagnostic")
	}
	var de *DiagnosticError
	if !errors.As(err, &de) {
		t.Fatalf("error type = %T, want *DiagnosticError", err)
	}
	if de.Code != CodeUnsupportedFeat {
		t.Fatalf("diagnostic code = %s, want %s", de.Code, CodeUnsupportedFeat)
	}
	if !strings.Contains(de.Message, "vertex hooks are not supported in points") {
		t.Fatalf("diagnostic message = %q", de.Message)
	}
}

// TestLowerMeshAuthoredVertex checks that a mesh material authoring its own
// vertex() stage lowers (B4): procedural geometry from vertexIndex, an author
// varying written in vertex and read in surface, with the expected WGSL constructs.
func TestLowerMeshAuthoredVertex(t *testing.T) {
	p, err := parse.Program([]byte(`material Ripple {
    param gridSize : float = 16.0
    varying worldPos : vec3
    vertex() -> vec4 {
        let fi = float(vertexIndex)
        let gx = fract(fi / gridSize)
        let p = vec3f(gx, 0.0, fi)
        worldPos = p
        return mvp * vec4f(gx, 0.0, fi, 1.0)
    }
    surface(geo) -> color {
        return rgb(geo.worldPos.x, geo.worldPos.y, geo.worldPos.z)
    }
}`))
	if err != nil {
		t.Fatal(err)
	}
	mod, layout, err := LowerProgram(p, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !mod.VertexAuthored {
		t.Fatal("module VertexAuthored = false, want true")
	}
	if !mod.UsesVertexIndex {
		t.Fatal("module UsesVertexIndex = false, want true")
	}
	if len(mod.Attributes) != 0 {
		t.Fatalf("attributes = %+v, want none (procedural geometry)", mod.Attributes)
	}
	if len(mod.Varyings) != 1 || mod.Varyings[0].Name != "worldPos" || mod.Varyings[0].Type != ir.Vec3 {
		t.Fatalf("varyings = %+v, want [worldPos vec3]", mod.Varyings)
	}
	if layout.EntryPoints.Vertex != "vertexMain" || layout.EntryPoints.Fragment != "fragmentMain" {
		t.Fatalf("entry points = %+v", layout.EntryPoints)
	}
	src, err := wgsl.Emit(mod)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"@vertex\nfn vertexMain(@builtin(vertex_index) vertexIndex : u32) -> VertexOutput {",
		"let fi = f32(vertexIndex);",
		"out.worldPos = p;",
		"let p = vec3<f32>(gx, 0.0, fi);",
		"out.position = (u.mvp * vec4<f32>(gx, 0.0, fi, 1.0));",
		"@location(0) worldPos : vec3<f32>,",
		"return vec4<f32>(vec3<f32>(in.worldPos.x, in.worldPos.y, in.worldPos.z), 1.0);",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("WGSL missing %q\n--- got ---\n%s", want, src)
		}
	}
}

func TestLowerTextured(t *testing.T) {
	mod, layout, err := Lower(hir.Textured())
	if err != nil {
		t.Fatal(err)
	}

	// texture surfaced in the module and descriptor with per-backend bindings
	if len(mod.Textures) != 1 || mod.Textures[0].Name != "albedo" {
		t.Fatalf("module textures = %+v, want [albedo]", mod.Textures)
	}
	if len(layout.Textures) != 1 {
		t.Fatalf("descriptor textures = %d, want 1", len(layout.Textures))
	}
	tex := layout.Textures[0]
	if tex.WGSL.TextureBinding != 1 || tex.WGSL.SamplerBinding != 2 {
		t.Errorf("wgsl tex/sampler binding = %d/%d, want 1/2", tex.WGSL.TextureBinding, tex.WGSL.SamplerBinding)
	}
	if tex.GL.Uniform != "albedo" || tex.Metal.Texture != 0 || tex.Metal.Sampler != 0 {
		t.Errorf("gl/metal binding wrong: %+v", tex)
	}

	// geo.uv synthesized into a varying
	var hasUV bool
	for _, v := range mod.Varyings {
		if v.Name == "vUv" {
			hasUV = true
		}
	}
	if !hasUV {
		t.Errorf("uv interpolant not synthesized; varyings=%+v", mod.Varyings)
	}

	w, _ := wgsl.Emit(mod)
	for _, want := range []string{
		"@group(0) @binding(1) var albedo : texture_2d<f32>;",
		"@group(0) @binding(2) var albedoSampler : sampler;",
		"let c = textureSample(albedo, albedoSampler, in.vUv).rgb;",
	} {
		if !strings.Contains(w, want) {
			t.Errorf("WGSL missing %q", want)
		}
	}
	m, _ := metal.Emit(mod)
	for _, want := range []string{
		"texture2d<float> albedo [[texture(0)]], sampler albedoSampler [[sampler(0)]]",
		"float3 c = albedo.sample(albedoSampler, in.vUv).rgb;",
	} {
		if !strings.Contains(m, want) {
			t.Errorf("Metal missing %q", want)
		}
	}
}

func TestLowerParamDefaults(t *testing.T) {
	p, err := parse.Program([]byte(`material Defaults {
    param baseColor : color = rgb(0.25, 0.5, 0.75)
    param gain : float = 1.5
    param basis : mat3 = mat3(1, 0, 0, 0, 1, 0, 0, 0, 1)
    param tintMatrix : mat4 = mat4(1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0.1, 0.2, 0.3, 1)
    surface(geo) -> color { return baseColor * gain }
}`))
	if err != nil {
		t.Fatal(err)
	}
	_, layout, err := LowerProgram(p, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(layout.UniformBlock.Defaults) != 4 {
		t.Fatalf("defaults = %+v, want 4", layout.UniformBlock.Defaults)
	}
	if got := layout.UniformBlock.Defaults[0]; got.Name != "baseColor" || got.Type != "vec3" || got.Values[2] != 0.75 {
		t.Fatalf("baseColor default = %+v", got)
	}
	if got := layout.UniformBlock.Defaults[1]; got.Name != "gain" || got.Type != "float" || got.Values[0] != 1.5 {
		t.Fatalf("gain default = %+v", got)
	}
	if got := layout.UniformBlock.Defaults[2]; got.Name != "basis" || got.Type != "mat3" || len(got.Values) != 9 || got.Values[4] != 1 {
		t.Fatalf("basis default = %+v", got)
	}
	if got := layout.UniformBlock.Defaults[3]; got.Name != "tintMatrix" || got.Type != "mat4" || len(got.Values) != 16 || got.Values[12] != 0.1 {
		t.Fatalf("tintMatrix default = %+v", got)
	}
}

func TestLowerSunDefault(t *testing.T) {
	p, err := parse.Program([]byte(`material SunDefault {
    param light : Sun = sun(vec3(0.0, 1.0, 0.0), 0.25)
    surface(geo) -> color {
        let n = normalize(geo.worldNormal)
        return rgb(1, 1, 1) * (light.ambient + max(dot(n, light.dir), 0))
    }
}`))
	if err != nil {
		t.Fatal(err)
	}
	_, layout, err := LowerProgram(p, 0)
	if err != nil {
		t.Fatal(err)
	}
	// Sun expands to light_ambient (float) + light_dir (vec3), in sorted field
	// order, each carrying a host-packable default from sun(dir, ambient).
	if len(layout.UniformBlock.Defaults) != 2 {
		t.Fatalf("defaults = %+v, want 2", layout.UniformBlock.Defaults)
	}
	if got := layout.UniformBlock.Defaults[0]; got.Name != "light_ambient" || got.Type != "float" || got.Values[0] != 0.25 {
		t.Fatalf("light_ambient default = %+v", got)
	}
	if got := layout.UniformBlock.Defaults[1]; got.Name != "light_dir" || got.Type != "vec3" || len(got.Values) != 3 || got.Values[1] != 1.0 {
		t.Fatalf("light_dir default = %+v", got)
	}
}

func TestLowerUnaryMinus(t *testing.T) {
	p, err := parse.Program([]byte(`material Neg {
    param dir : vec3 = vec3(0.0, -1.0, 0.0)
    param k : float = -0.5
    surface(geo) -> color {
        let n = normalize(geo.worldNormal)
        return rgb(1, 1, 1) * (max(dot(n, -dir), 0) - k)
    }
}`))
	if err != nil {
		t.Fatal(err)
	}
	// Lowering succeeds (the surface uses -dir, exercising resolver/typer/
	// inliner), and negation in defaults carries the sign to the descriptor.
	_, layout, err := LowerProgram(p, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := layout.UniformBlock.Defaults[0]; got.Name != "dir" || got.Values[1] != -1.0 {
		t.Fatalf("dir default = %+v, want y = -1", got)
	}
	if got := layout.UniformBlock.Defaults[1]; got.Name != "k" || got.Values[0] != -0.5 {
		t.Fatalf("k default = %+v, want -0.5", got)
	}
}

func TestLowerRejectsInterfaceNameCollisions(t *testing.T) {
	cases := []struct {
		name string
		m    hir.Material
		want string
	}{
		{
			name: "duplicate param",
			m: hir.Material{
				Name: "Bad",
				Params: []hir.Param{
					{Name: "baseColor", Type: hir.Color},
					{Name: "baseColor", Type: hir.Vec3},
				},
				Surface: hir.Func{Geo: "geo", Result: hir.Ref{Name: "baseColor"}},
			},
			want: `duplicate param "baseColor"`,
		},
		{
			name: "reserved param",
			m: hir.Material{
				Name:   "Bad",
				Params: []hir.Param{{Name: "var", Type: hir.Float}},
				Surface: hir.Func{
					Geo:    "geo",
					Result: hir.Ref{Name: "var"},
				},
			},
			want: `param "var" is reserved for WGSL binding keyword`,
		},
		{
			name: "uniform collides with implicit attribute",
			m: hir.Material{
				Name:    "Bad",
				Params:  []hir.Param{{Name: "position", Type: hir.Color}},
				Surface: hir.Func{Geo: "geo", Result: hir.Ref{Name: "position"}},
			},
			want: `param "position" is reserved for generated vertex position attribute`,
		},
		{
			name: "reserved texture param",
			m: hir.Material{
				Name:   "Bad",
				Params: []hir.Param{{Name: "texture", Type: hir.Texture2D}},
				Surface: hir.Func{Geo: "geo", Result: hir.Member{E: hir.Call{Func: "sample", Args: []hir.Expr{
					hir.Ref{Name: "texture"},
					hir.Member{E: hir.Ref{Name: "geo"}, Field: "uv"},
				}}, Field: "rgb"}},
			},
			want: `param "texture" is reserved for GLES builtin function`,
		},
		{
			name: "texture sampler collides with uniform",
			m: hir.Material{
				Name: "Bad",
				Params: []hir.Param{
					{Name: "albedo", Type: hir.Texture2D},
					{Name: "albedoSampler", Type: hir.Color},
				},
				Surface: hir.Func{Geo: "geo", Result: hir.Member{E: hir.Call{Func: "sample", Args: []hir.Expr{hir.Ref{Name: "albedo"}, hir.Member{E: hir.Ref{Name: "geo"}, Field: "uv"}}}, Field: "rgb"}},
			},
			want: `texture sampler "albedoSampler" conflicts with uniform`,
		},
		{
			name: "local collides with uniform",
			m: hir.Material{
				Name:   "Bad",
				Params: []hir.Param{{Name: "baseColor", Type: hir.Color}},
				Surface: hir.Func{
					Geo:    "geo",
					Body:   []hir.Stmt{hir.Let{Name: "baseColor", Value: hir.Lit{Value: 1}}},
					Result: hir.Ref{Name: "baseColor"},
				},
			},
			want: `surface local "baseColor" conflicts with uniform`,
		},
		{
			name: "reserved local",
			m: hir.Material{
				Name: "Bad",
				Surface: hir.Func{
					Geo: "geo",
					Body: []hir.Stmt{
						hir.Let{Name: "fragColor", Value: hir.Call{Func: "rgb", Args: []hir.Expr{hir.Lit{Value: 1}, hir.Lit{Value: 0}, hir.Lit{Value: 0}}}},
					},
					Result: hir.Ref{Name: "fragColor"},
				},
			},
			want: `surface local "fragColor" is reserved for generated GLES fragment output`,
		},
		{
			name: "duplicate local",
			m: hir.Material{
				Name: "Bad",
				Surface: hir.Func{
					Geo: "geo",
					Body: []hir.Stmt{
						hir.Let{Name: "c", Value: hir.Call{Func: "rgb", Args: []hir.Expr{hir.Lit{Value: 1}, hir.Lit{Value: 0}, hir.Lit{Value: 0}}}},
						hir.Let{Name: "c", Value: hir.Call{Func: "rgb", Args: []hir.Expr{hir.Lit{Value: 0}, hir.Lit{Value: 1}, hir.Lit{Value: 0}}}},
					},
					Result: hir.Ref{Name: "c"},
				},
			},
			want: `duplicate surface local "c"`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := Lower(tc.m)
			if err == nil {
				t.Fatal("Lower succeeded, want error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want substring %q", err, tc.want)
			}
		})
	}
}

func TestLowerRejectsInvalidExpressions(t *testing.T) {
	cases := []struct {
		name string
		m    hir.Material
		want string
	}{
		{
			name: "invalid swizzle",
			m: hir.Material{
				Name:    "Bad",
				Params:  []hir.Param{{Name: "baseColor", Type: hir.Color}},
				Surface: hir.Func{Geo: "geo", Result: hir.Member{E: hir.Ref{Name: "baseColor"}, Field: "a"}},
			},
			want: "invalid swizzle .a for vec3",
		},
		{
			name: "binary type mismatch",
			m: hir.Material{
				Name:   "Bad",
				Params: []hir.Param{{Name: "baseColor", Type: hir.Color}},
				Surface: hir.Func{Geo: "geo", Result: hir.Binary{
					Op: "+",
					L:  hir.Ref{Name: "baseColor"},
					R:  hir.Call{Func: "rgb", Args: []hir.Expr{hir.Lit{Value: 1}, hir.Lit{Value: 0}, hir.Lit{Value: 0}, hir.Lit{Value: 1}}},
				}},
			},
			want: "operator + is not defined for vec3 and vec4",
		},
		{
			name: "sample uv type",
			m: hir.Material{
				Name:   "Bad",
				Params: []hir.Param{{Name: "albedo", Type: hir.Texture2D}},
				Surface: hir.Func{Geo: "geo", Result: hir.Member{E: hir.Call{Func: "sample", Args: []hir.Expr{
					hir.Ref{Name: "albedo"},
					hir.Member{E: hir.Ref{Name: "geo"}, Field: "worldNormal"},
				}}, Field: "rgb"}},
			},
			want: "sample: second argument must be vec2 uv, got vec3",
		},
		{
			name: "dot type mismatch",
			m: hir.Material{
				Name: "Bad",
				Surface: hir.Func{Geo: "geo", Result: hir.Call{Func: "rgb", Args: []hir.Expr{
					hir.Call{Func: "dot", Args: []hir.Expr{
						hir.Member{E: hir.Ref{Name: "geo"}, Field: "worldNormal"},
						hir.Member{E: hir.Ref{Name: "geo"}, Field: "uv"},
					}},
					hir.Lit{Value: 0},
					hir.Lit{Value: 0},
				}}},
			},
			want: "dot arguments must be matching vectors, got vec3 and vec2",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := Lower(tc.m)
			if err == nil {
				t.Fatal("Lower succeeded, want error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want substring %q", err, tc.want)
			}
		})
	}
}

func TestLowerRejectsInvalidParamDefaults(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "wrong type",
			src: `material Bad {
    param gain : float = rgb(1, 1, 1)
    surface(geo) -> color { return rgb(gain, gain, gain) }
}`,
			want: "param \"gain\" default: got vec3, want float",
		},
		{
			name: "non constant",
			src: `material Bad {
    param baseColor : color = missing
    surface(geo) -> color { return baseColor }
}`,
			want: "param \"baseColor\" default: hir.Ref is not a constant default expression",
		},
		{
			name: "texture default",
			src: `material Bad {
    param albedo : texture2d = rgb(1, 1, 1)
    surface(geo) -> color { return sample(albedo, geo.uv).rgb }
}`,
			want: "param \"albedo\": defaults for texture2d are not supported",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := parse.Program([]byte(tc.src))
			if err != nil {
				t.Fatal(err)
			}
			_, _, err = LowerProgram(p, 0)
			if err == nil {
				t.Fatal("LowerProgram succeeded, want error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want substring %q", err, tc.want)
			}
		})
	}
}

// TestLowerWorldPosGeometryField verifies geo.worldPos lowers to a vWorldPos
// varying that passes the (pre-baked world-space) position attribute through
// unchanged — the scene3d mesh convention — and types as vec3.
func TestLowerWorldPosGeometryField(t *testing.T) {
	src := `material WorldPosProbe {
    param tint : color = rgb(0.5, 0.5, 0.5)

    context {
        cameraPos : vec3
    }

    surface(geo) -> color {
        let v = normalize(cameraPos - geo.worldPos)
        let n = normalize(geo.worldNormal)
        let fres = pow(1.0 - max(dot(n, v), 0.0), 3.0)
        return tint * fres
    }
}`
	program, err := parse.Program([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	mod, layout, err := Lower(program.Materials[0])
	if err != nil {
		t.Fatal(err)
	}

	names := map[string]bool{}
	for _, v := range mod.Varyings {
		names[v.Name] = true
	}
	if !names["vWorldPos"] || !names["vWorldNormal"] {
		t.Fatalf("varyings = %+v, want vWorldPos + vWorldNormal", mod.Varyings)
	}
	foundPassthrough := false
	for _, stmt := range mod.Vertex.Body {
		if stmt.Target == "vWorldPos" {
			if ref, ok := stmt.Value.(ir.Ref); ok && ref.Name == "position" {
				foundPassthrough = true
			}
		}
	}
	if !foundPassthrough {
		t.Fatal("vWorldPos should pass the world-space position attribute through unchanged")
	}
	classByName := map[string]string{}
	for _, f := range layout.UniformBlock.Fields {
		classByName[f.Name] = f.Class
	}
	if classByName["cameraPos"] != "context" {
		t.Fatalf("cameraPos field class = %q, want context", classByName["cameraPos"])
	}
}

// TestLowerSampleLevel verifies sampleLevel(tex, uv, lod) types as vec4 (like
// sample) and lowers to each backend's explicit-LOD texture fetch: WGSL
// textureSampleLevel (valid inside non-uniform control flow, unlike
// textureSample), Metal tex.sample(...,level(lod)), GLES textureLod, and
// GLSL texture2DLod.
func TestLowerSampleLevel(t *testing.T) {
	src := `material SampleLevelProbe {
    param albedo : texture2d
    surface(geo) -> color {
        let c = sampleLevel(albedo, geo.uv, 0.0).rgb
        return c
    }
}`
	program, err := parse.Program([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	mod, _, err := Lower(program.Materials[0])
	if err != nil {
		t.Fatal(err)
	}

	w, err := wgsl.Emit(mod)
	if err != nil {
		t.Fatalf("wgsl emit: %v", err)
	}
	if want := "textureSampleLevel(albedo, albedoSampler, in.vUv, 0.0)"; !strings.Contains(w, want) {
		t.Errorf("WGSL missing %q\n--- got ---\n%s", want, w)
	}

	m, err := metal.Emit(mod)
	if err != nil {
		t.Fatalf("metal emit: %v", err)
	}
	if want := "albedo.sample(albedoSampler, in.vUv, level(0.0))"; !strings.Contains(m, want) {
		t.Errorf("Metal missing %q\n--- got ---\n%s", want, m)
	}

	_, gf, err := gles.Emit(mod)
	if err != nil {
		t.Fatalf("gles emit: %v", err)
	}
	if want := "textureLod(albedo, vUv, 0.0)"; !strings.Contains(gf, want) {
		t.Errorf("GLES missing %q\n--- got ---\n%s", want, gf)
	}

	_, sf, err := glsl.Emit(mod)
	if err != nil {
		t.Fatalf("glsl emit: %v", err)
	}
	if want := "texture2DLod(albedo, vUv, 0.0)"; !strings.Contains(sf, want) {
		t.Errorf("GLSL missing %q\n--- got ---\n%s", want, sf)
	}
}

// TestLowerRejectsSampleLevelBadArgs verifies sampleLevel's arity/type
// diagnostics mirror sample's (CodeInvalidCall/CodeTypeMismatch).
func TestLowerRejectsSampleLevelBadArgs(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "wrong arity",
			src: `material Bad {
    param albedo : texture2d
    surface(geo) -> color { return sampleLevel(albedo, geo.uv).rgb }
}`,
			want: "sampleLevel(texture, uv, lod) takes 3 arguments",
		},
		{
			name: "not a texture",
			src: `material Bad {
    param albedo : float
    surface(geo) -> color { return sampleLevel(albedo, geo.uv, 0.0).rgb }
}`,
			want: "sampleLevel: first argument must be a texture2d param",
		},
		{
			name: "lod not a float",
			src: `material Bad {
    param albedo : texture2d
    surface(geo) -> color { return sampleLevel(albedo, geo.uv, geo.uv).rgb }
}`,
			want: "sampleLevel: third argument must be a float lod",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := parse.Program([]byte(tc.src))
			if err != nil {
				t.Fatal(err)
			}
			_, _, err = LowerProgram(p, 0)
			if err == nil {
				t.Fatal("LowerProgram succeeded, want error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want substring %q", err, tc.want)
			}
		})
	}
}

// TestLowerUnknownFunctionNamesTheCall is the regression test for a
// fabricated type: callType used to fall back to the type of the first
// argument for any unknown function name, so color(1.0, 0.0, 0.0, 1.0) --
// "color" is not a registered builtin; the real constructor is rgb -- typed
// as float and surfaced the nonsense "surface must return color/vec3/vec4,
// got float" error. The diagnostic must name the unknown function instead of
// fabricating a type, and point at rgb for this specific mistake.
func TestLowerUnknownFunctionNamesTheCall(t *testing.T) {
	src := `material Bad {
    surface(geo) -> color { return color(1.0, 0.0, 0.0, 1.0) }
}`
	p, err := parse.Program([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = LowerProgram(p, 0)
	if err == nil {
		t.Fatal("LowerProgram succeeded on an unknown function, want a diagnostic error")
	}
	var de *DiagnosticError
	if !errors.As(err, &de) {
		t.Fatalf("error type = %T, want *DiagnosticError", err)
	}
	if !strings.Contains(de.Message, `unknown function "color"`) {
		t.Fatalf("diagnostic message = %q, want it to name the unknown function", de.Message)
	}
	if !strings.Contains(de.Message, "rgb(") {
		t.Fatalf("diagnostic message = %q, want it to point at rgb", de.Message)
	}
	if strings.Contains(err.Error(), "got float") {
		t.Fatalf("error = %q, must not fabricate a type from the first argument", err)
	}
}

// TestLowerUnknownFunctionNeverReachesEmittedOutput is the regression test
// for lower/resolver.go's unknown-call fallback: it used to pass an
// unresolved call straight through into the IR, so a material calling
// shadeMe(...) survived lowering and landed verbatim in emitted WGSL -- an
// invalid shader that only --validate-shaders would have caught. Lowering
// must reject the call before any emitter ever sees it.
func TestLowerUnknownFunctionNeverReachesEmittedOutput(t *testing.T) {
	src := `material Bad {
    surface(geo) -> color { return shadeMe(rgb(1.0, 0.0, 0.0), 0.5) }
}`
	p, err := parse.Program([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	mod, _, err := LowerProgram(p, 0)
	if err == nil {
		if out, emitErr := wgsl.Emit(mod); emitErr == nil && strings.Contains(out, "shadeMe(") {
			t.Fatalf("shadeMe reached emitted WGSL verbatim:\n%s", out)
		}
		t.Fatal("LowerProgram succeeded on an unknown function call, want a diagnostic error")
	}
	var de *DiagnosticError
	if !errors.As(err, &de) {
		t.Fatalf("error type = %T, want *DiagnosticError", err)
	}
	if !strings.Contains(de.Message, `unknown function "shadeMe"`) {
		t.Fatalf("diagnostic message = %q, want it to name the unknown function", de.Message)
	}
}

// TestLowerEarlyReturnCollectsGeoFieldUsage is the regression test for
// collectGeoStmt (lower_helpers.go): geo.uv is referenced ONLY inside a
// return nested in an if body, never in the trailing top-level return or any
// plain Let/VarDecl. buildInterfacePlan walks surface.Body through
// collectGeoStmt to decide which geometry fields need an attribute/varying;
// without a case for hir.Return, this usage would be invisible and geo.uv
// would fail to resolve inside the early return with "geo.uv is not
// available in the surface" — dropping the varying silently is exactly the
// failure mode the plan called out.
func TestLowerEarlyReturnCollectsGeoFieldUsage(t *testing.T) {
	p, err := parse.Program([]byte(`material EarlyReturnGeo {
    param cutoff : float = 0.5
    surface(geo) -> color {
        if (geo.uv.x < cutoff) {
            return rgb(1.0, 0.0, 0.0)
        }
        return rgb(0.0, 1.0, 0.0)
    }
}`))
	if err != nil {
		t.Fatal(err)
	}
	mod, _, err := LowerProgram(p, 0)
	if err != nil {
		t.Fatalf("geo field used only inside an early return should lower cleanly: %v", err)
	}
	found := false
	for _, v := range mod.Varyings {
		if v.Name == "vUv" {
			found = true
		}
	}
	if !found {
		t.Fatalf("module varyings = %+v, want vUv wired from the geo.uv usage inside the early return", mod.Varyings)
	}
	out, err := wgsl.Emit(mod)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "in.vUv.x") {
		t.Fatalf("WGSL fragment does not reference the uv varying inside the branch:\n%s", out)
	}
}
