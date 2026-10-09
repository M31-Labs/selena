package selena

import (
	"m31labs.dev/selena/parse"
	"slices"
	"strings"
	"testing"
)

func TestCubeLevelCompilesEveryTarget(t *testing.T) {
	source := `material Prefiltered {
    param radiance : textureCube
    param roughness : float = 0.5
    surface(geo) -> color {
        return sampleCubeLevel(radiance, normalize(geo.worldNormal), roughness * 6.0).rgb
    }
}`
	r, err := Compile([]byte(source), CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for target, want := range map[Target]string{
		TargetWGSL:  "textureSampleLevel(radiance, radianceSampler,",
		TargetGLES:  "textureLod(radiance,",
		TargetGLSL:  "textureCubeLodEXT(radiance,",
		TargetMetal: "radiance.sample(radianceSampler,",
	} {
		a, ok := r.Artifact(target)
		if !ok || !strings.Contains(a.Source+a.Fragment, want) {
			t.Fatalf("%s missing explicit cube LOD: %s", target, a.Source+a.Fragment)
		}
	}
	glsl, _ := r.Artifact(TargetGLSL)
	if !strings.Contains(glsl.Fragment, "#extension GL_EXT_shader_texture_lod : enable") {
		t.Fatal("WebGL1 lacks required extension directive")
	}
	if !slices.Contains(r.Layout.Requires.GLExtensions, "EXT_shader_texture_lod") {
		t.Fatal("host descriptor omits cube LOD requirement")
	}
	if r.Layout.Requires.SceneColorMips {
		t.Fatal("environment sampling must not allocate backdrop mipmaps")
	}
	if len(r.Layout.Textures) != 1 || r.Layout.Textures[0].Dimension != "cube" {
		t.Fatal("lost cube binding")
	}
}

func TestCubeLevelLibraryKeepsTextureBindings(t *testing.T) {
	source := `fn environment(tex: textureCube, roughness: float) -> vec3 {
    let direction = vec3f(0.0, 1.0, 0.0)
    return sampleCubeLevel(tex, direction, roughness * 6.0).rgb
}
material LibraryProbe {
    param radiance : textureCube
    surface(geo) -> color { return environment(radiance, 0.8) }
}`
	p, err := parse.Program([]byte(source))
	if err != nil {
		t.Fatal(err)
	}
	for i := range p.Funcs {
		p.Funcs[i].BindLocals = true
	}
	r, err := CompileProgram(p, CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	a, _ := r.Artifact(TargetWGSL)
	if !strings.Contains(a.Source, "textureSampleLevel(radiance, radianceSampler,") {
		t.Fatal("library lost the source texture binding")
	}
	if len(r.Layout.Textures) != 1 || r.Layout.Textures[0].Name != "radiance" {
		t.Fatal("library duplicated texture bindings")
	}
}

func TestCubeLevelRejectsInvalidArguments(t *testing.T) {
	for _, expr := range []string{
		"sampleCubeLevel(radiance, vec3(0.0,1.0,0.0))",
		"sampleCubeLevel(image, vec3(0.0,1.0,0.0), 1.0)",
		"sampleCubeLevel(radiance, vec2(0.0,1.0), 1.0)",
		"sampleCubeLevel(radiance, vec3(0.0,1.0,0.0), vec2(0.0,1.0))",
	} {
		source := `material Invalid { param radiance : textureCube param image : texture2d surface(geo) -> color { return ` + expr + `.rgb } }`
		if _, err := Compile([]byte(source), CompileOptions{}); err == nil {
			t.Fatalf("accepted %s", expr)
		}
	}
}
