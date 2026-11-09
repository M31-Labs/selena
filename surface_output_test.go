package selena

import (
	"strings"
	"testing"

	"m31labs.dev/selena/bindings"
	"m31labs.dev/selena/ir"
)

const surfaceRockSource = `material Rock {
    param albedo : texture2d
    param tint : color = rgb(0.9, 0.85, 0.8)
    param wet : float = 0.0

    surface(geo) -> Surface {
        let c = sample(albedo, geo.uv).rgb * tint
        var s = pbr(c, 0.8, 0.0)
        if (wet > 0.5) {
            s = withRoughness(s, s.roughness * 0.3)
        }
        let moss = pbr(rgb(0.2, 0.4, 0.1), 0.9, 0.0)
        return withOcclusion(mixSurface(s, moss, clamp(geo.worldNormal.y, 0.0, 1.0)), 0.9)
    }
}`

func TestSurfaceKindLayoutContract(t *testing.T) {
	res, err := Compile([]byte(surfaceRockSource), CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	l := res.Layout
	if l.Kind != bindings.SurfaceKindSurface || res.Module.Kind != ir.KindSurface {
		t.Fatalf("kind = %q / %q, want surface", l.Kind, res.Module.Kind)
	}
	if l.Surface == nil || l.Surface.ABI != bindings.SurfaceABI || l.Surface.Function != "selenaSurface" || l.Surface.GLUniformPrefix != "selena_" {
		t.Fatalf("surface hook = %+v", l.Surface)
	}
	if got := strings.Join(l.Surface.TextureNames, ","); got != "selenaSurfaceTex0" {
		t.Fatalf("texture names = %q", got)
	}
	if len(l.Textures) != 1 || l.Textures[0].Name != "albedo" {
		t.Fatalf("textures = %+v", l.Textures)
	}
	if l.WGSL.Group != 2 || l.WGSL.Binding != 0 || l.EntryPoints.Fragment != "selenaSurface" || l.EntryPoints.Vertex != "" {
		t.Fatalf("binding/entry = %+v %+v", l.WGSL, l.EntryPoints)
	}
	if l.Attributes == nil || len(l.Attributes) != 0 {
		t.Fatalf("attributes = %#v, want empty", l.Attributes)
	}
	var names []string
	for _, f := range l.UniformBlock.Fields {
		names = append(names, f.Name)
	}
	if got := strings.Join(names, ","); got != "tint,wet" {
		t.Fatalf("uniform fields = %q, want tint,wet (no mvp/normalMatrix)", got)
	}
	var targets []string
	for _, a := range res.Artifacts {
		targets = append(targets, string(a.Target))
	}
	if got := strings.Join(targets, ","); got != "wgsl,gles" {
		t.Fatalf("default targets = %q, want wgsl,gles", got)
	}
}

func TestSurfaceKindHooks(t *testing.T) {
	res, err := Compile([]byte(surfaceRockSource), CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	w, _ := res.Artifact(TargetWGSL)
	for _, want := range []string{
		"struct SelenaSurface {",
		"@group(2) @binding(0) var<uniform> selenaParams : SelenaSurfaceParams;",
		"fn selenaSurface(selenaGeo : SelenaSurfaceGeo) -> SelenaSurface {",
		"textureSample(selenaSurfaceTex0, selenaSurfaceTex0Sampler, selenaGeo.uv)",
		"selenaParams.tint",
		"return SelenaSurface(selenaOut_baseColor, selenaOut_alpha, selenaOut_normal,",
	} {
		if !strings.Contains(w.Source, want) {
			t.Errorf("wgsl hook missing %q:\n%s", want, w.Source)
		}
	}
	for _, reject := range []string{"@vertex", "@fragment", "var albedo", "mvp"} {
		if strings.Contains(w.Source, reject) {
			t.Errorf("wgsl hook contains %q", reject)
		}
	}
	g, _ := res.Artifact(TargetGLES)
	if g.Vertex != "" {
		t.Errorf("gles hook has a vertex stage:\n%s", g.Vertex)
	}
	for _, want := range []string{
		"uniform vec3 selena_tint;",
		"uniform float selena_wet;",
		"SelenaSurface selenaSurface(SelenaSurfaceGeo selenaGeo) {",
		"texture(selenaSurfaceTex0, selenaGeo.uv)",
	} {
		if !strings.Contains(g.Fragment, want) {
			t.Errorf("gles hook missing %q:\n%s", want, g.Fragment)
		}
	}
	for _, reject := range []string{"#version", "void main", "uniform sampler2D"} {
		if strings.Contains(g.Fragment, reject) {
			t.Errorf("gles hook contains %q", reject)
		}
	}
}

func TestSurfaceKindRejectsUnsupportedTargets(t *testing.T) {
	for _, target := range []Target{TargetMetal, TargetGLSL} {
		if _, err := Compile([]byte(surfaceRockSource), CompileOptions{Targets: []Target{target}}); err == nil || !strings.Contains(err.Error(), "no surface hook") {
			t.Errorf("%s: err = %v, want no surface hook", target, err)
		}
	}
}

func TestSurfaceKindFunctionsTakeAndReturnSurface(t *testing.T) {
	src := `fn wetten(s: Surface, k: float) -> Surface {
    return withRoughness(s, s.roughness * k)
}
material Wet {
    surface(geo) -> Surface {
        let s = pbr(rgb(0.5, 0.5, 0.5), 0.8, 0.0)
        return wetten(withClearcoat(s, 1.0, 0.05), 0.25)
    }
}`
	res, err := Compile([]byte(src), CompileOptions{Targets: []Target{TargetWGSL}})
	if err != nil {
		t.Fatal(err)
	}
	w, _ := res.Artifact(TargetWGSL)
	for _, want := range []string{"let selenaOut_roughness = (s_roughness * 0.25);", "let selenaOut_clearcoat = 1.0;", "let selenaOut_clearcoatRoughness = 0.05;"} {
		if !strings.Contains(w.Source, want) {
			t.Errorf("missing %q:\n%s", want, w.Source)
		}
	}
}

func TestSurfaceKindAssignmentReadsOldRecord(t *testing.T) {
	src := `material Swap {
    surface(geo) -> Surface {
        var s = pbr(rgb(0.2, 0.2, 0.2), 0.9, 0.1)
        s = pbr(vec3f(s.metallic, s.metallic, s.metallic), s.metallic, s.roughness)
        return s
    }
}`
	res, err := Compile([]byte(src), CompileOptions{Targets: []Target{TargetWGSL}})
	if err != nil {
		t.Fatal(err)
	}
	w, _ := res.Artifact(TargetWGSL)
	// The new roughness reads the old metallic and the new metallic the old
	// roughness: both must be evaluated before either is written.
	iRough := strings.Index(w.Source, "s_roughness = ")
	iMetal := strings.Index(w.Source, "s_metallic = ")
	iBind := strings.Index(w.Source, "let selenaSurf")
	if iBind < 0 || iRough < iBind || iMetal < iBind {
		t.Fatalf("assignments are not staged through temporaries:\n%s", w.Source)
	}
}

func TestSurfaceKindDiagnostics(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{"early return", `material M { surface(geo) -> Surface {
    if (geo.uv.x > 0.5) { return pbr(rgb(1, 1, 1), 0.5, 0.0) }
    return pbr(rgb(0, 0, 0), 0.5, 0.0)
} }`, "early return"},
		{"surface as value", `material M { surface(geo) -> Surface {
    let s = pbr(rgb(1, 1, 1), 0.5, 0.0)
    let x = s * 2.0
    return s
} }`, "used as a value"},
		{"unknown field", `material M { surface(geo) -> Surface {
    let s = pbr(rgb(1, 1, 1), 0.5, 0.0)
    return withRoughness(s, s.shininess)
} }`, "no field \"shininess\""},
		{"field type", `material M { surface(geo) -> Surface {
    return withRoughness(pbr(rgb(1, 1, 1), 0.5, 0.0), rgb(1, 1, 1))
} }`, "Surface.roughness must be float"},
		{"color result", `material M { surface(geo) -> Surface { return rgb(1, 1, 1) } }`, "must return a Surface value"},
		{"kind needs Surface", `material M kind surface { surface(geo) -> color { return rgb(1, 1, 1) } }`, "must declare surface(geo) -> Surface"},
		{"vertex", `material M {
    vertex(geo) -> vec4 { return vec4f(geo.position, 1.0) }
    surface(geo) -> Surface { return pbr(rgb(1, 1, 1), 0.5, 0.0) }
}`, "vertex() is not supported"},
		{"cube texture", `material M { param env : textureCube
    surface(geo) -> Surface { return pbr(sampleCube(env, geo.worldNormal).rgb, 0.5, 0.0) } }`, "texture2d params only"},
		{"geo field", `material M { surface(geo) -> Surface { return pbr(geo.position, 0.5, 0.0) } }`, "geo.position"},
		{"too many textures", `material M {
    param a : texture2d
    param b : texture2d
    param c : texture2d
    param d : texture2d
    param e : texture2d
    param f : texture2d
    param g : texture2d
    surface(geo) -> Surface { return pbr(sample(a, geo.uv).rgb, 0.5, 0.0) } }`, "at most 6"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Compile([]byte(tc.src), CompileOptions{})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestSurfaceKindExplicitKindAndConditional(t *testing.T) {
	src := `material Pick kind surface {
    param wet : float = 0.0
    surface(geo) -> Surface {
        let s = wet > 0.5 ? pbr(rgb(0.1, 0.1, 0.1), 0.2, 0.0) : pbr(rgb(0.4, 0.4, 0.4), 0.9, 0.0)
        return s
    }
}`
	res, err := Compile([]byte(src), CompileOptions{Targets: []Target{TargetWGSL}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Layout.Kind != bindings.SurfaceKindSurface {
		t.Fatalf("kind = %q", res.Layout.Kind)
	}
	w, _ := res.Artifact(TargetWGSL)
	if !strings.Contains(w.Source, "let selenaSurf0 = (selenaParams.wet > 0.5);") || !strings.Contains(w.Source, "select(0.9, 0.2, selenaSurf0)") {
		t.Fatalf("conditional Surface not split per field:\n%s", w.Source)
	}
}

// Texel reads in a hook use the hook's texture names, and data formats are
// rejected: hosts bind surface textures in their image slots.
func TestSurfaceKindTexelReadsAndDataFormats(t *testing.T) {
	src := `material Texel {
    param albedo : texture2d
    surface(geo) -> Surface {
        let size = size(albedo)
        let texel = load(albedo, int(geo.uv.x * size.x), int(geo.uv.y * size.y))
        return pbr(texel.rgb, 0.5, 0.0)
    }
}`
	res, err := Compile([]byte(src), CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	w, _ := res.Artifact(TargetWGSL)
	g, _ := res.Artifact(TargetGLES)
	for _, hook := range []string{w.Source, g.Fragment} {
		if strings.Contains(hook, "(albedo") || !strings.Contains(hook, "selenaSurfaceTex0") {
			t.Fatalf("texel read kept the param name:\n%s", hook)
		}
	}
	bad := `material Data {
    param heights : texture2d format r16float
    surface(geo) -> Surface { return pbr(rgb(1, 1, 1), load(heights, 0i, 0i).r, 0.0) }
}`
	if _, err := Compile([]byte(bad), CompileOptions{}); err == nil || !strings.Contains(err.Error(), "data formats") {
		t.Fatalf("err = %v, want a data-format rejection", err)
	}
}
