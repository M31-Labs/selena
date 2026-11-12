package validate

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"m31labs.dev/selena"
	"m31labs.dev/selena/internal/shadertest"
	"m31labs.dev/selena/materiallib"
)

// A GLSL program is two stages linked together. Validating each stage alone
// accepts a pair that does not link: a uniform both stages declare must have
// the same type and precision in both. These tests link the pair.

// linkGLES links one vertex and one fragment source with glslangValidator -l
// and returns its output when the link fails.
func linkGLES(t *testing.T, vertex, fragment string) (string, bool) {
	t.Helper()
	bin, err := exec.LookPath("glslangValidator")
	if err != nil {
		shadertest.Missing(t, "glslangValidator")
	}
	dir := t.TempDir()
	vp, fp := filepath.Join(dir, "shader.vert"), filepath.Join(dir, "shader.frag")
	if err := os.WriteFile(vp, []byte(vertex), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fp, []byte(fragment), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(bin, "-l", vp, fp).CombinedOutput()
	return strings.ReplaceAll(string(out), dir+string(filepath.Separator), ""), err == nil
}

// TestGLESStagesLinkWhenBothReadOneTexture is the ordinary height-map case:
// vertex() displaces by a texture that surface() also samples.
func TestGLESStagesLinkWhenBothReadOneTexture(t *testing.T) {
	src := []byte(`material BothStageTexture {
    param heightMap : texture2d
    param scale : float = 0.1

    varying vUv : vec2

    vertex(geo) -> vec4 {
        let h = sampleLevel(heightMap, geo.uv, 0.0).r
        vUv = geo.uv
        return mvp * vec4f(geo.position + geo.normal * (h * scale), 1.0)
    }

    surface(geo) -> color {
        let c = sample(heightMap, geo.vUv)
        return rgb(c.r, c.g, c.b)
    }
}
`)
	res, err := selena.Compile(src, selena.CompileOptions{Targets: []selena.Target{selena.TargetGLES, selena.TargetGLSL}})
	if err != nil {
		t.Fatal(err)
	}
	gles, _ := res.Artifact(selena.TargetGLES)
	if out, ok := linkGLES(t, gles.Vertex, gles.Fragment); !ok {
		t.Errorf("gles stages do not link:\n%s", out)
	}
	// GLSL ES 1.00: the sampler must be highp in both stages too. (The whole
	// pair cannot be linked by glslang yet: see TestGLESCorpusStagesLink.)
	glsl, _ := res.Artifact(selena.TargetGLSL)
	for stage, source := range map[string]string{"vertex": glsl.Vertex, "fragment": glsl.Fragment} {
		if !strings.Contains(source, "uniform highp sampler2D heightMap;") {
			t.Errorf("glsl %s stage does not declare heightMap highp:\n%s", stage, source)
		}
	}
}

// TestGLESCorpusStagesLink links the GLSL ES 3.00 pair of every mesh material
// in the conformance corpus and the examples.
//
// The GLSL ES 1.00 target is not linked here. Its fragment stage declares
// every uniform at mediump and its vertex stage at highp, including uniforms
// only one stage uses; glslang reports each as a mismatch. WebGL 1 compares
// precision only for a uniform both stages use.
func TestGLESCorpusStagesLink(t *testing.T) {
	var files []string
	for _, pattern := range []string{"../testdata/conformance/*.sel", "../examples/*.sel", "../testdata/instancing/*.sel"} {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, matches...)
	}
	if len(files) == 0 {
		t.Fatal("no corpus files found")
	}
	linked := 0
	for _, file := range files {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		res, err := materiallib.Compile(src, selena.CompileOptions{Targets: []selena.Target{selena.TargetGLES}}, materiallib.Lighting)
		if err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		if kind := res.Layout.Kind; kind != "" && kind != "mesh" {
			continue // other kinds pair with a host or engine vertex stage
		}
		gles, ok := res.Artifact(selena.TargetGLES)
		if !ok || gles.Vertex == "" || gles.Fragment == "" {
			continue
		}
		if out, ok := linkGLES(t, gles.Vertex, gles.Fragment); !ok {
			t.Errorf("%s: gles stages do not link:\n%s", filepath.Base(file), out)
		}
		linked++
	}
	if linked < 20 {
		t.Fatalf("linked %d materials; the corpus has more mesh materials than that", linked)
	}
	t.Logf("linked %d mesh materials", linked)
}
