package builder_test

import (
	"bytes"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"m31labs.dev/selena"
	"m31labs.dev/selena/artifact"
	"m31labs.dev/selena/artifact/builder"
	"m31labs.dev/selena/materiallib"
	"m31labs.dev/selena/result"
)

var update = flag.Bool("update", false, "rewrite golden artifact files")

// goldenCases build each testdata source with fixed options. The golden file
// is testdata/<source>.json, the path artifact.PathFor gives.
var goldenCases = []struct {
	source string
	opts   builder.Options
}{
	{"tinted.sel", builder.Options{}},
	{"post-tint.sel", builder.Options{Targets: []result.Target{result.TargetWGSL, result.TargetGLES}}},
	{"brdf-specular.sel", builder.Options{SpecularMRT: true, Modules: []materiallib.Module{materiallib.BRDF}}},
	{"data-texture.sel", builder.Options{}},
}

func TestBuildMatchesGolden(t *testing.T) {
	for _, tc := range goldenCases {
		t.Run(tc.source, func(t *testing.T) {
			src := readSource(t, tc.source)
			opts := tc.opts
			opts.Path = tc.source
			opts.Compiler = "golden"
			file, err := builder.Build(src, opts)
			if err != nil {
				t.Fatal(err)
			}
			got, err := artifact.Encode(file)
			if err != nil {
				t.Fatal(err)
			}
			golden := filepath.Join("testdata", artifact.PathFor(tc.source))
			if *update {
				if err := os.WriteFile(golden, got, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("%v (run go test ./artifact/builder -update)", err)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("%s differs from a fresh build; run go test ./artifact/builder -update and review the diff", golden)
			}
			// The golden file must also pass Check, the drift test a host runs.
			shipped, err := artifact.Decode(want)
			if err != nil {
				t.Fatal(err)
			}
			if err := builder.Check(shipped, src); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestBuildIsDeterministic(t *testing.T) {
	src := readSource(t, "brdf-specular.sel")
	opts := builder.Options{Path: "brdf-specular.sel", SpecularMRT: true, Modules: []materiallib.Module{materiallib.BRDF}, Compiler: "x"}
	var first []byte
	for i := 0; i < 3; i++ {
		file, err := builder.Build(src, opts)
		if err != nil {
			t.Fatal(err)
		}
		data, err := artifact.Encode(file)
		if err != nil {
			t.Fatal(err)
		}
		if first == nil {
			first = data
			continue
		}
		if !bytes.Equal(first, data) {
			t.Fatalf("build %d encoded differently", i)
		}
	}
	decoded, err := artifact.Decode(first)
	if err != nil {
		t.Fatal(err)
	}
	again, err := artifact.Encode(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, again) {
		t.Fatal("decode then encode changed the bytes")
	}
}

func TestBuildMatchesCompilerOutput(t *testing.T) {
	src := readSource(t, "tinted.sel")
	file, err := builder.Build(src, builder.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got := file.Names(); !reflect.DeepEqual(got, []string{"Base", "Tinted"}) {
		t.Fatalf("materials = %v", got)
	}
	if file.Default != "Tinted" {
		t.Fatalf("default = %q, want the last material", file.Default)
	}
	if !reflect.DeepEqual(file.Options.Targets, selena.AllTargets()) {
		t.Fatalf("targets = %v", file.Options.Targets)
	}
	if file.Compiler == "" || file.LanguageVersion == "" {
		t.Fatalf("missing provenance: compiler %q language %q", file.Compiler, file.LanguageVersion)
	}
	for _, name := range file.Names() {
		want, err := selena.Compile(src, selena.CompileOptions{Material: name})
		if err != nil {
			t.Fatal(err)
		}
		got, err := file.Result(name)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got.Layout, want.Layout) {
			t.Fatalf("%s layout differs from selena.Compile", name)
		}
		if !reflect.DeepEqual(got.Artifacts, want.Artifacts) {
			t.Fatalf("%s artifacts differ from selena.Compile", name)
		}
	}
}

func TestBuildSelectsMaterials(t *testing.T) {
	src := readSource(t, "tinted.sel")
	file, err := builder.Build(src, builder.Options{Materials: []string{"Base"}, Targets: []result.Target{result.TargetWGSL}})
	if err != nil {
		t.Fatal(err)
	}
	if got := file.Names(); !reflect.DeepEqual(got, []string{"Base"}) || file.Default != "Base" {
		t.Fatalf("names %v default %q", got, file.Default)
	}
	for _, bad := range []builder.Options{
		{Materials: []string{"Missing"}},
		{Materials: []string{"Base", "Base"}},
		{Targets: []result.Target{}},
	} {
		if _, err := builder.Build(src, bad); err == nil {
			t.Fatalf("Build accepted %+v", bad)
		}
	}
}

func TestBuildLinksModulesOnRequest(t *testing.T) {
	src := readSource(t, "brdf-specular.sel")
	if _, err := builder.Build(src, builder.Options{SpecularMRT: true}); err == nil {
		t.Fatal("a source that calls a materiallib helper compiled without the module")
	}
	file, err := builder.Build(src, builder.Options{SpecularMRT: true, Modules: []materiallib.Module{materiallib.BRDF}})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(file.Options.Modules, []string{"brdf"}) {
		t.Fatalf("modules = %v", file.Options.Modules)
	}
	m, _ := file.Material("")
	for _, s := range m.Shaders {
		if s.SpecularMRT == nil {
			t.Fatalf("%s shader has no specular variant", s.Target)
		}
	}
}

func TestBuildKeepsCompileDiagnostics(t *testing.T) {
	_, err := builder.Build([]byte("material Broken {\n  surface(geo) -> color { return missing }\n}\n"), builder.Options{Path: "broken.sel"})
	var ce *selena.CompileError
	if !errors.As(err, &ce) || len(ce.Diagnostics) == 0 {
		t.Fatalf("err = %v, want a CompileError with diagnostics", err)
	}
	_, err = builder.Build([]byte("material {"), builder.Options{Path: "syntax.sel"})
	if !errors.As(err, &ce) {
		t.Fatalf("syntax err = %v, want a CompileError", err)
	}
}

func TestCheckDetectsDrift(t *testing.T) {
	src := readSource(t, "tinted.sel")
	shipped, err := builder.Build(src, builder.Options{Path: "tinted.sel"})
	if err != nil {
		t.Fatal(err)
	}
	if err := builder.Check(shipped, src); err != nil {
		t.Fatalf("fresh artifact reported stale: %v", err)
	}

	// A different recorded compiler version alone is not drift.
	other := shipped
	other.Compiler = "v9.9.9"
	if err := builder.Check(other, src); err != nil {
		t.Fatalf("version-only change reported stale: %v", err)
	}

	edited := bytes.Replace(src, []byte("* tint"), []byte("* tint * 0.5"), 1)
	assertStale(t, builder.Check(shipped, edited), "sha256")

	tampered := cloneFile(t, shipped)
	tampered.Materials[1].Shaders[0].Source += "\n// edited by hand\n"
	assertStale(t, builder.Check(tampered, src), "wgsl shader differs")

	relaid := cloneFile(t, shipped)
	relaid.Materials[0].Layout.UniformBlock.Size += 16
	assertStale(t, builder.Check(relaid, src), "binding layout differs")

	broken := []byte("material Base {")
	brokenFile := cloneFile(t, shipped)
	brokenFile.Source.SHA256 = artifact.Hash(broken)
	assertStale(t, builder.Check(brokenFile, broken), "no longer compiles")
}

func assertStale(t *testing.T, err error, want string) {
	t.Helper()
	var stale *builder.StaleError
	if !errors.As(err, &stale) || !errors.Is(err, builder.ErrStale) {
		t.Fatalf("err = %v, want a StaleError", err)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("err = %q, want it to mention %q", err, want)
	}
}

func TestBuildReportsLibrarySources(t *testing.T) {
	_, err := builder.Build([]byte("fn half(x: float) -> float {\n    return x * 0.5\n}\n"), builder.Options{Path: "lib.sel"})
	if !errors.Is(err, builder.ErrNoMaterial) {
		t.Fatalf("err = %v, want ErrNoMaterial", err)
	}
}

func TestVersionIsNeverEmpty(t *testing.T) {
	if builder.Version() == "" {
		t.Fatal("Version is empty")
	}
}

func cloneFile(t *testing.T, f artifact.File) artifact.File {
	t.Helper()
	data, err := artifact.Encode(f)
	if err != nil {
		t.Fatal(err)
	}
	out, err := artifact.Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func readSource(t *testing.T, name string) []byte {
	t.Helper()
	src, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return src
}
