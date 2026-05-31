package lower

import (
	"strings"
	"testing"

	"m31labs.dev/selena/emit/metal"
	"m31labs.dev/selena/emit/wgsl"
	"m31labs.dev/selena/hir"
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
			name: "uniform collides with implicit attribute",
			m: hir.Material{
				Name:    "Bad",
				Params:  []hir.Param{{Name: "position", Type: hir.Color}},
				Surface: hir.Func{Geo: "geo", Result: hir.Ref{Name: "position"}},
			},
			want: `attribute "position" conflicts with uniform`,
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
					Body:   []hir.Let{{Name: "baseColor", Value: hir.Lit{Value: 1}}},
					Result: hir.Ref{Name: "baseColor"},
				},
			},
			want: `surface local "baseColor" conflicts with uniform`,
		},
		{
			name: "duplicate local",
			m: hir.Material{
				Name: "Bad",
				Surface: hir.Func{
					Geo: "geo",
					Body: []hir.Let{
						{Name: "c", Value: hir.Call{Func: "rgb", Args: []hir.Expr{hir.Lit{Value: 1}, hir.Lit{Value: 0}, hir.Lit{Value: 0}}}},
						{Name: "c", Value: hir.Call{Func: "rgb", Args: []hir.Expr{hir.Lit{Value: 0}, hir.Lit{Value: 1}, hir.Lit{Value: 0}}}},
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
