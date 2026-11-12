package validate

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"m31labs.dev/selena"
	"m31labs.dev/selena/internal/shadertest"
	"m31labs.dev/selena/materiallib"
)

// A GLSL program is two stages linked together. Validating each stage alone
// accepts a pair that does not link: a uniform both stages declare must have
// the same type and precision in both. These tests link the GLSL ES 3.00
// (gles) pair with glslang and compare precision qualifiers in the GLSL ES 1.00
// (glsl) pair, which glslang cannot link without false mismatches (see
// TestGLSLCorpusSharedUniformPrecision).

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
	// GLSL ES 1.00: the sampler must be highp in both stages too.
	glsl, _ := res.Artifact(selena.TargetGLSL)
	for stage, source := range map[string]string{"vertex": glsl.Vertex, "fragment": glsl.Fragment} {
		if !strings.Contains(source, "uniform highp sampler2D heightMap;") {
			t.Errorf("glsl %s stage does not declare heightMap highp:\n%s", stage, source)
		}
	}
}

// A texture only the vertex stage reads is left out of the GLSL ES 1.00
// fragment stage: a highp declaration there would not compile on a device
// without fragment highp, though nothing in the fragment uses it.
func TestGLSLFragmentOmitsVertexOnlyTexture(t *testing.T) {
	src := []byte(`material VertexOnlyTexture {
    param heightMap : texture2d
    param albedo : texture2d

    varying vUv : vec2

    vertex(geo) -> vec4 {
        let h = sampleLevel(heightMap, geo.uv, 0.0).r
        vUv = geo.uv
        return mvp * vec4f(geo.position + geo.normal * h, 1.0)
    }

    surface(geo) -> color {
        let c = sample(albedo, geo.vUv)
        return rgb(c.r, c.g, c.b)
    }
}
`)
	res, err := selena.Compile(src, selena.CompileOptions{Targets: []selena.Target{selena.TargetGLSL}})
	if err != nil {
		t.Fatal(err)
	}
	glsl, _ := res.Artifact(selena.TargetGLSL)
	if !strings.Contains(glsl.Vertex, "uniform highp sampler2D heightMap;") {
		t.Errorf("vertex stage does not declare heightMap highp:\n%s", glsl.Vertex)
	}
	if strings.Contains(glsl.Fragment, "heightMap") {
		t.Errorf("fragment stage declares heightMap, which only the vertex stage reads:\n%s", glsl.Fragment)
	}
	if !strings.Contains(glsl.Fragment, "uniform sampler2D albedo;") {
		t.Errorf("fragment stage lost albedo:\n%s", glsl.Fragment)
	}
}

// An int param read in both stages, with no integer varying, must have one
// precision. A fragment int defaults to mediump, so the fragment stage has to
// say highp itself.
func TestGLESStagesLinkWhenBothReadOneInt(t *testing.T) {
	src := []byte(`material BothStageInt {
    param n : int
    param scale : float = 0.1

    varying vUv : vec2

    vertex(geo) -> vec4 {
        vUv = geo.uv
        return mvp * vec4f(geo.position * (scale * float(n)), 1.0)
    }

    surface(geo) -> color {
        let k = float(n)
        return rgb(k * 0.25, geo.vUv.x, geo.vUv.y)
    }
}
`)
	res, err := selena.Compile(src, selena.CompileOptions{Targets: []selena.Target{selena.TargetGLES}})
	if err != nil {
		t.Fatal(err)
	}
	gles, _ := res.Artifact(selena.TargetGLES)
	if !strings.Contains(gles.Vertex, "uniform int n;") || !strings.Contains(gles.Fragment, "uniform int n;") {
		t.Fatalf("both stages should declare n:\nvertex:\n%s\nfragment:\n%s", gles.Vertex, gles.Fragment)
	}
	if out, ok := linkGLES(t, gles.Vertex, gles.Fragment); !ok {
		t.Errorf("gles stages do not link:\n%s", out)
	}
}

// linkSweepFile is one source file of the sweeps and the options it compiles with.
type linkSweepFile struct {
	path    string
	modules []materiallib.Module
}

// linkSweepFiles lists the sources both sweeps cover: the conformance corpus,
// instancing corpus and examples compile with the lighting module, and
// examples/material-library with every bundled module, which includes the game module.
func linkSweepFiles(t *testing.T) []linkSweepFile {
	t.Helper()
	var files []linkSweepFile
	for _, set := range []struct {
		pattern string
		modules []materiallib.Module
	}{
		{"../testdata/conformance/*.sel", []materiallib.Module{materiallib.Lighting}},
		{"../examples/*.sel", []materiallib.Module{materiallib.Lighting}},
		{"../testdata/instancing/*.sel", []materiallib.Module{materiallib.Lighting}},
		{"../examples/material-library/*.sel", materiallib.Modules()}, // includes materiallib.Game
	} {
		matches, err := filepath.Glob(set.pattern)
		if err != nil {
			t.Fatal(err)
		}
		if len(matches) == 0 {
			t.Fatalf("no files match %s", set.pattern)
		}
		for _, m := range matches {
			files = append(files, linkSweepFile{path: m, modules: set.modules})
		}
	}
	return files
}

// TestGLESCorpusStagesLink links the GLSL ES 3.00 pair of every material in
// the sweep sets (see linkSweepFiles) for which Selena emits both stages. It
// skips only a material with an empty vertex or fragment stage: a surface
// hook has no vertex stage because the host's runs.
func TestGLESCorpusStagesLink(t *testing.T) {
	linked := 0
	for _, file := range linkSweepFiles(t) {
		src, err := os.ReadFile(file.path)
		if err != nil {
			t.Fatal(err)
		}
		res, err := materiallib.Compile(src, selena.CompileOptions{Targets: []selena.Target{selena.TargetGLES}}, file.modules...)
		if err != nil {
			t.Fatalf("%s: %v", file.path, err)
		}
		gles, ok := res.Artifact(selena.TargetGLES)
		if !ok || gles.Vertex == "" || gles.Fragment == "" {
			continue
		}
		if out, ok := linkGLES(t, gles.Vertex, gles.Fragment); !ok {
			t.Errorf("%s: gles stages do not link:\n%s", filepath.Base(file.path), out)
		}
		linked++
	}
	if linked < 30 {
		t.Fatalf("linked %d materials; the sweep sets have more than that", linked)
	}
	t.Logf("linked %d gles stage pairs", linked)
}

// uniformPrecisions maps each uniform declared in a GLSL ES 1.00 source to its
// effective precision. An explicit qualifier wins. Without one the default
// applies: a sampler is lowp in both stages, and any other uniform is highp in
// the vertex stage and takes the fragment stage's "precision mediump float;".
func uniformPrecisions(source string, fragment bool) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(source, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "uniform ") {
			continue
		}
		fields := strings.Fields(strings.TrimSuffix(line, ";"))
		precision := ""
		if len(fields) > 1 && (fields[1] == "highp" || fields[1] == "mediump" || fields[1] == "lowp") {
			precision = fields[1]
			fields = append(fields[:1], fields[2:]...)
		}
		// fields: uniform <type> <name>[N]
		if len(fields) < 3 {
			continue
		}
		typ, name := fields[1], fields[2]
		if i := strings.IndexByte(name, '['); i >= 0 {
			name = name[:i]
		}
		switch {
		case typ == "bool":
			precision = "none" // a bool takes no precision
		case precision != "":
		case strings.HasPrefix(typ, "sampler"):
			precision = "lowp"
		case fragment:
			precision = "mediump"
		default:
			precision = "highp"
		}
		out[name] = precision
	}
	return out
}

// withoutUniformDecls drops the uniform declaration lines of a GLSL source.
func withoutUniformDecls(source string) string {
	var kept []string
	for _, line := range strings.Split(source, "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "uniform ") {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n")
}

// identifierIn reports whether name appears in source as a whole identifier.
func identifierIn(source, name string) bool {
	return regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\b`).MatchString(source)
}

// TestGLSLCorpusSharedUniformPrecision checks the GLSL ES 1.00 (glsl) pair of
// every material in the sweep sets. glslang cannot link this pair: the
// fragment stage declares every uniform at mediump and the vertex stage at
// highp, including uniforms only one stage uses, and glslang reports each as
// a mismatch though WebGL 1 compares precision only for a uniform both stages
// use. So the test compares the declared precision of each uniform whose name
// both stages use outside a declaration (plain, array and texture uniforms).
// It cannot tell a local that shadows a uniform's name from a read, so it may
// over-report but not miss a shared uniform.
func TestGLSLCorpusSharedUniformPrecision(t *testing.T) {
	checked, shared, skipped := 0, 0, 0
	for _, file := range linkSweepFiles(t) {
		src, err := os.ReadFile(file.path)
		if err != nil {
			t.Fatal(err)
		}
		res, err := materiallib.Compile(src, selena.CompileOptions{Targets: []selena.Target{selena.TargetGLSL}}, file.modules...)
		if err != nil {
			msg := err.Error()
			if strings.Contains(msg, "cannot target GLSL ES 1.00") || strings.Contains(msg, "has no surface hook") {
				skipped++ // no glsl artifact exists: WebGL 1 lacks a feature it needs
				continue
			}
			t.Fatalf("%s: %v", file.path, err)
		}
		glsl, ok := res.Artifact(selena.TargetGLSL)
		if !ok || glsl.Vertex == "" || glsl.Fragment == "" {
			continue
		}
		checked++
		vertex, fragment := uniformPrecisions(glsl.Vertex, false), uniformPrecisions(glsl.Fragment, true)
		// A uniform both stages read (the name appears outside a uniform
		// declaration in both) must have one precision.
		vbody, fbody := withoutUniformDecls(glsl.Vertex), withoutUniformDecls(glsl.Fragment)
		names := make([]string, 0, len(vertex))
		for name := range vertex {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			fp, declared := fragment[name]
			if !declared || !identifierIn(vbody, name) || !identifierIn(fbody, name) {
				continue
			}
			shared++
			if vp := vertex[name]; vp != fp {
				t.Errorf("%s: uniform %s is %s in the vertex stage and %s in the fragment stage", filepath.Base(file.path), name, vp, fp)
			}
		}
	}
	if checked < 30 {
		t.Fatalf("checked %d materials; the sweep sets have more than that", checked)
	}
	t.Logf("checked %d glsl stage pairs, %d shared uniforms; %d materials have no glsl artifact", checked, shared, skipped)
}
