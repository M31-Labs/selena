package selena

import (
	"flag"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

var updateConformanceGolden = flag.Bool(
	"update-conformance-golden",
	false,
	"rewrite conformance golden shader outputs",
)

func TestConformanceGoldenOutputs(t *testing.T) {
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
			base := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
			outputs := goldenOutputs(res)
			names := make([]string, 0, len(outputs))
			for name := range outputs {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				path := filepath.Join("testdata", "conformance", "golden", base+"."+name)
				assertGolden(t, path, outputs[name])
			}
		})
	}
}

func assertGolden(t *testing.T, path, got string) {
	t.Helper()
	got = normalizeGolden(got)
	if *updateConformanceGolden {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v; run go test -run TestConformanceGoldenOutputs -update-conformance-golden .", path, err)
	}
	if string(want) != got {
		t.Fatalf("golden %s changed; run go test -run TestConformanceGoldenOutputs -update-conformance-golden . to refresh", path)
	}
}

func goldenOutputs(res Result) map[string]string {
	out := map[string]string{}
	for _, target := range AllTargets() {
		artifact, ok := res.Artifact(target)
		if !ok {
			continue
		}
		switch target {
		case TargetWGSL:
			out["wgsl"] = artifact.Source
		case TargetMetal:
			out["metal"] = artifact.Source
		case TargetGLSL:
			out["glsl.vert"] = artifact.Vertex
			out["glsl.frag"] = artifact.Fragment
		case TargetGLES:
			out["gles.vert"] = artifact.Vertex
			out["gles.frag"] = artifact.Fragment
		}
	}
	return out
}

func normalizeGolden(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.TrimSpace(s)
	return s + "\n"
}
