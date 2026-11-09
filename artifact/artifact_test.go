package artifact_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"m31labs.dev/selena/artifact"
	"m31labs.dev/selena/bindings"
	"m31labs.dev/selena/result"
)

func golden(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("builder", "testdata", "tinted.sel.json"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestDecodeGolden(t *testing.T) {
	f, err := artifact.Decode(golden(t))
	if err != nil {
		t.Fatal(err)
	}
	if f.Schema != artifact.Schema || f.Default != "Tinted" || len(f.Materials) != 2 {
		t.Fatalf("decoded %q default %q with %d materials", f.Schema, f.Default, len(f.Materials))
	}
	res, err := f.Result("")
	if err != nil {
		t.Fatal(err)
	}
	if res.Material.Name != "Tinted" || res.Layout.Material != "Tinted" {
		t.Fatalf("default result is %q / %q", res.Material.Name, res.Layout.Material)
	}
	if a, ok := res.Artifact(result.TargetWGSL); !ok || !strings.Contains(a.Source, "fn fragmentMain") {
		t.Fatal("default result has no WGSL module")
	}
	if a, ok := res.Artifact(result.TargetGLES); !ok || a.Vertex == "" || a.Fragment == "" {
		t.Fatal("default result has no GLES program")
	}
	if _, err := f.Result("Missing"); err == nil || !strings.Contains(err.Error(), "Base, Tinted") {
		t.Fatalf("missing material error = %v", err)
	}
}

func TestMaterialRoundTripsThroughResult(t *testing.T) {
	f, err := artifact.Decode(golden(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range f.Materials {
		back := artifact.FromResult(m.Result())
		if !reflect.DeepEqual(back, m) {
			t.Fatalf("%s changed through Result/FromResult", m.Name)
		}
	}
	mrt := result.Result{Layout: f.Materials[0].Layout, Artifacts: []result.Artifact{{
		Target: result.TargetWGSL, Source: "a", SpecularMRT: &result.FragmentArtifact{Source: "b"},
	}}}
	m := artifact.FromResult(mrt)
	if m.Name != f.Materials[0].Name || m.Shaders[0].SpecularMRT == nil || m.Shaders[0].SpecularMRT.Source != "b" {
		t.Fatalf("FromResult lost the name or the specular variant: %+v", m)
	}
	if got := m.Artifacts()[0].SpecularMRT; got == nil || got.Source != "b" {
		t.Fatal("Artifacts lost the specular variant")
	}
}

// Data-texture fields (textures[].format, sampleMode, requires.glTexSizeUniforms)
// survive Decode, Result/FromResult and Encode, and an artifact without them
// decodes with all three empty.
func TestDataTextureFieldsRoundTrip(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("builder", "testdata", "data-texture.sel.json"))
	if err != nil {
		t.Fatal(err)
	}
	f, err := artifact.Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	layout := f.Materials[0].Layout
	want := []struct{ name, format, mode string }{
		{"spectrum", "r16float", "load"},
		{"gains", "rgba32float", "load"},
		{"palette", "rgba16float", "both"},
	}
	for i, w := range want {
		g := layout.Textures[i]
		if g.Name != w.name || g.Format != w.format || g.SampleMode != w.mode {
			t.Errorf("texture %d = %s/%s/%s, want %s/%s/%s", i, g.Name, g.Format, g.SampleMode, w.name, w.format, w.mode)
		}
	}
	if got := layout.Requires.GLTexSizeUniforms; len(got) != 3 || got[0] != "_texSize_spectrum" {
		t.Errorf("GLTexSizeUniforms = %v", got)
	}
	back := artifact.FromResult(f.Materials[0].Result())
	if !reflect.DeepEqual(back, f.Materials[0]) {
		t.Fatal("data-texture fields changed through Result/FromResult")
	}
	again, err := artifact.Encode(f)
	if err != nil {
		t.Fatal(err)
	}
	if string(again) != string(data) {
		t.Fatal("re-encoding the decoded file changed its bytes")
	}

	old, err := artifact.Decode(golden(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range old.Materials {
		for _, tex := range m.Layout.Textures {
			if tex.Format != "" || tex.SampleMode != "" {
				t.Errorf("%s/%s: unexpected data-texture fields %q %q", m.Name, tex.Name, tex.Format, tex.SampleMode)
			}
		}
		if len(m.Layout.Requires.GLTexSizeUniforms) != 0 {
			t.Errorf("%s: unexpected GLTexSizeUniforms", m.Name)
		}
	}
}

// The screen pipeline fields survive Decode, Result/FromResult and Encode, and
// an artifact written before they existed decodes with none of them and
// encodes back to the same bytes.
func TestScreenPipelineRoundTrips(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("builder", "testdata", "screen-overlay.sel.json"))
	if err != nil {
		t.Fatal(err)
	}
	f, err := artifact.Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	layout := f.Materials[0].Layout
	if layout.Kind != "screen" {
		t.Fatalf("kind = %q", layout.Kind)
	}
	want := bindings.PipelineState{Blend: "premultiplied", DepthTest: true}
	if layout.Pipeline == nil || *layout.Pipeline != want {
		t.Fatalf("pipeline = %+v, want %+v", layout.Pipeline, want)
	}
	if len(layout.Context) != 1 || layout.Context[0] != "screenSize" {
		t.Errorf("context = %v", layout.Context)
	}
	back := artifact.FromResult(f.Materials[0].Result())
	if !reflect.DeepEqual(back, f.Materials[0]) {
		t.Fatal("the pipeline changed through Result/FromResult")
	}
	again, err := artifact.Encode(f)
	if err != nil {
		t.Fatal(err)
	}
	if string(again) != string(data) {
		t.Fatal("re-encoding the decoded file changed its bytes")
	}

	for _, name := range []string{"tinted", "post-tint", "data-texture", "brdf-specular"} {
		old, err := os.ReadFile(filepath.Join("builder", "testdata", name+".sel.json"))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(old), `"pipeline"`) {
			t.Errorf("%s: an older artifact mentions a pipeline", name)
		}
		file, err := artifact.Decode(old)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for _, m := range file.Materials {
			if m.Layout.Pipeline != nil {
				t.Errorf("%s/%s: decoded a pipeline from an artifact without one", name, m.Name)
			}
		}
		enc, err := artifact.Encode(file)
		if err != nil {
			t.Fatal(err)
		}
		if string(enc) != string(old) {
			t.Errorf("%s: re-encoding changed the bytes", name)
		}
	}
}

func TestMatchesUsesTheSourceHash(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("builder", "testdata", "tinted.sel"))
	if err != nil {
		t.Fatal(err)
	}
	f, err := artifact.Decode(golden(t))
	if err != nil {
		t.Fatal(err)
	}
	if !f.Matches(src) {
		t.Fatal("golden does not match its source")
	}
	if f.Matches(append(src, ' ')) {
		t.Fatal("an edited source matched")
	}
	if got := artifact.PathFor("materials/rock.sel"); got != "materials/rock.sel.json" {
		t.Fatalf("PathFor = %q", got)
	}
}

func TestDecodeRejectsMalformedFiles(t *testing.T) {
	base, err := artifact.Decode(golden(t))
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(*artifact.File){
		"schema":       func(f *artifact.File) { f.Schema = "selena.artifact.v0" },
		"short hash":   func(f *artifact.File) { f.Source.SHA256 = "abc" },
		"upper hash":   func(f *artifact.File) { f.Source.SHA256 = strings.ToUpper(f.Source.SHA256) },
		"no materials": func(f *artifact.File) { f.Materials = nil },
		"unnamed":      func(f *artifact.File) { f.Materials[0].Name = "" },
		"duplicate": func(f *artifact.File) {
			f.Materials[1].Name = f.Materials[0].Name
			f.Materials[1].Layout.Material = f.Materials[0].Name
		},
		"layout name":       func(f *artifact.File) { f.Materials[0].Layout.Material = "Other" },
		"default":           func(f *artifact.File) { f.Default = "Missing" },
		"undeclared shader": func(f *artifact.File) { f.Options.Targets = f.Options.Targets[:1] },
	}
	for name, mutate := range cases {
		f := base
		f.Materials = append([]artifact.Material(nil), base.Materials...)
		f.Options.Targets = append([]result.Target(nil), base.Options.Targets...)
		mutate(&f)
		if _, err := artifact.Encode(f); err == nil {
			t.Errorf("%s: Encode accepted an invalid file", name)
		}
	}
	if _, err := artifact.Decode([]byte("{")); err == nil {
		t.Fatal("Decode accepted truncated JSON")
	}
}

// The artifact package exists so a browser WASM module can load compiled
// materials without the compiler. Fail if its import closure, built for
// js/wasm, reaches any Selena package outside the leaf set or any module
// outside Selena.
func TestImportClosureExcludesTheCompiler(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go tool unavailable")
	}
	cmd := exec.Command("go", "list", "-deps", "-f", "{{if not .Standard}}{{.ImportPath}}{{end}}", ".")
	cmd.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm", "CGO_ENABLED=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list: %v\n%s", err, out)
	}
	allowed := map[string]bool{
		"m31labs.dev/selena/artifact": true,
		"m31labs.dev/selena/bindings": true,
		"m31labs.dev/selena/hir":      true,
		"m31labs.dev/selena/ir":       true,
		"m31labs.dev/selena/result":   true,
	}
	for _, dep := range strings.Fields(string(out)) {
		if !allowed[dep] {
			t.Errorf("artifact links %s; only %v are allowed", dep, keys(allowed))
		}
	}
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
