package selena

import (
	"os"
	"path/filepath"
	"testing"

	"m31labs.dev/selena/bindings"
)

func TestConformanceCorpusCompilesAllTargets(t *testing.T) {
	for _, file := range conformanceFiles(t) {
		t.Run(filepath.Base(file), func(t *testing.T) {
			src, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			res, err := Compile(src, CompileOptions{})
			if err != nil {
				t.Fatalf("compile %s: %v", file, err)
			}
			assertConformanceResult(t, res)
		})
	}
}

func conformanceFiles(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob("testdata/conformance/*.sel")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no conformance corpus files found")
	}
	return files
}

func assertConformanceResult(t *testing.T, res Result) {
	t.Helper()
	if res.Layout.SchemaVersion != bindings.DescriptorSchemaVersion {
		t.Fatalf("schema version = %q, want %q", res.Layout.SchemaVersion, bindings.DescriptorSchemaVersion)
	}
	if res.Layout.LanguageVersion != bindings.LanguageVersion {
		t.Fatalf("language version = %q, want %q", res.Layout.LanguageVersion, bindings.LanguageVersion)
	}
	if _, err := res.Layout.JSON(); err != nil {
		t.Fatalf("descriptor JSON: %v", err)
	}
	if _, err := bindings.PackUniforms(res.Layout, uniformValues(res.Layout, false)); err != nil {
		t.Fatalf("pack uniforms: %v", err)
	}
	if _, err := bindings.PackUniformsWithDefaults(res.Layout, uniformValues(res.Layout, true)); err != nil {
		t.Fatalf("pack uniforms with defaults: %v", err)
	}

	targets := AllTargets()
	if len(res.Artifacts) != len(targets) {
		t.Fatalf("artifacts = %d, want %d", len(res.Artifacts), len(targets))
	}
	for _, target := range targets {
		artifact, ok := res.Artifact(target)
		if !ok {
			t.Fatalf("missing artifact for %s", target)
		}
		switch target {
		case TargetWGSL, TargetMetal:
			if artifact.Source == "" || artifact.Vertex != "" || artifact.Fragment != "" {
				t.Fatalf("%s artifact shape = %+v, want single source", target, artifact)
			}
		case TargetGLSL, TargetGLES:
			if artifact.Source != "" || artifact.Vertex == "" || artifact.Fragment == "" {
				t.Fatalf("%s artifact shape = %+v, want split vertex/fragment", target, artifact)
			}
		default:
			t.Fatalf("unexpected target %s", target)
		}
	}
}

func uniformValues(layout bindings.Layout, omitDefaults bool) map[string]any {
	defaults := map[string]bool{}
	if omitDefaults {
		for _, d := range layout.UniformBlock.Defaults {
			defaults[d.Name] = true
		}
	}
	out := map[string]any{}
	for _, field := range layout.UniformBlock.Fields {
		if defaults[field.Name] {
			continue
		}
		out[field.Name] = zeroValueField(field)
	}
	return out
}

// zeroValueField returns a zero-value host representation for the given
// uniform field. For array fields (Count > 1), it returns a flat []float32
// slice with Count * componentsPerElement zeros.
func zeroValueField(f bindings.Field) any {
	if f.Count > 1 {
		comps := componentCountForType(f.Type)
		return make([]float32, comps*f.Count)
	}
	return zeroValue(f.Type)
}

func zeroValue(t string) any {
	switch t {
	case "float":
		return float32(0)
	case "vec2":
		return []float32{0, 0}
	case "vec3":
		return []float32{0, 0, 0}
	case "vec4":
		return []float32{0, 0, 0, 0}
	case "mat3":
		return []float32{0, 0, 0, 0, 0, 0, 0, 0, 0}
	case "mat4":
		return []float32{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	default:
		return float32(0)
	}
}

// componentCountForType returns the number of float32 components per element
// for the given type string.
func componentCountForType(t string) int {
	switch t {
	case "float":
		return 1
	case "vec2":
		return 2
	case "vec3":
		return 3
	case "vec4":
		return 4
	case "mat3":
		return 9
	case "mat4":
		return 16
	default:
		return 1
	}
}
