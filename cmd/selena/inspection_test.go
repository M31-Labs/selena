package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGraphPrintsJSON(t *testing.T) {
	output := captureStdout(t, func() {
		if err := run([]string{"graph", examplePath(t, "directional-diffuse.sel")}); err != nil {
			t.Fatalf("graph: %v", err)
		}
	})
	var payload struct {
		GraphVersion int    `json:"graph_version"`
		Material     string `json:"material"`
		Counts       struct {
			Materials int `json:"materials"`
			Artifacts int `json:"artifacts"`
			Uniforms  int `json:"uniforms"`
		} `json:"counts"`
		IR struct {
			Name     string `json:"name"`
			Fragment struct {
				Output string `json:"output"`
			} `json:"fragment"`
		} `json:"ir"`
	}
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatalf("unmarshal graph: %v\n%s", err, output)
	}
	if payload.GraphVersion != 1 || payload.Material != "DirectionalDiffuse" || payload.Counts.Materials != 1 {
		t.Fatalf("unexpected graph identity: %+v", payload)
	}
	if payload.Counts.Artifacts != 4 || payload.Counts.Uniforms == 0 || payload.IR.Name == "" || payload.IR.Fragment.Output == "" {
		t.Fatalf("graph missing compiler structure: %+v", payload)
	}
}

func TestShadersExtractsSourcesWithValidation(t *testing.T) {
	outDir := filepath.Join(t.TempDir(), "shaders")
	output := captureStdout(t, func() {
		if err := run([]string{"shaders", "--target", "wgsl", "--validate", "--out", outDir, examplePath(t, "directional-diffuse.sel")}); err != nil {
			t.Fatalf("shaders: %v", err)
		}
	})
	if !strings.Contains(output, "wrote 1 shader source") {
		t.Fatalf("unexpected shaders output: %s", output)
	}
	data, err := os.ReadFile(filepath.Join(outDir, "manifest.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var manifest struct {
		SourceCount int `json:"source_count"`
		Sources     []struct {
			Target     string `json:"target"`
			SourceFile string `json:"source_file"`
			Validation *struct {
				EntryChecked bool     `json:"entry_checked"`
				Entries      []string `json:"entries"`
			} `json:"validation"`
		} `json:"sources"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("unmarshal manifest: %v\n%s", err, data)
	}
	if manifest.SourceCount != 1 || len(manifest.Sources) != 1 || manifest.Sources[0].Target != "wgsl" {
		t.Fatalf("unexpected manifest: %+v", manifest)
	}
	if manifest.Sources[0].Validation == nil || !manifest.Sources[0].Validation.EntryChecked || len(manifest.Sources[0].Validation.Entries) != 2 {
		t.Fatalf("validation metadata missing: %+v", manifest.Sources[0].Validation)
	}
	src, err := os.ReadFile(filepath.Join(outDir, manifest.Sources[0].SourceFile))
	if err != nil {
		t.Fatalf("read source: %v", err)
	}
	if !strings.Contains(string(src), "vertexMain") || !strings.Contains(string(src), "fragmentMain") {
		t.Fatalf("extracted WGSL missing stage entries:\n%s", src)
	}
}

func TestCompileBundleWritesInspectionArtifacts(t *testing.T) {
	srcPath := copyExample(t, "directional-diffuse.sel")
	bundleDir := filepath.Join(t.TempDir(), "bundle")
	captureStdout(t, func() {
		if err := run([]string{"compile", "--bundle", bundleDir, "--validate-shaders", srcPath}); err != nil {
			t.Fatalf("compile bundle: %v", err)
		}
	})
	for _, path := range []string{
		filepath.Join(bundleDir, "manifest.json"),
		filepath.Join(bundleDir, "source.sel"),
		filepath.Join(bundleDir, "graph.json"),
		filepath.Join(bundleDir, "shaders"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected bundle path %q: %v", path, err)
		}
	}
}

func TestDoctorReportsTools(t *testing.T) {
	output := captureStdout(t, func() {
		if err := run([]string{"doctor"}); err != nil {
			t.Fatalf("doctor: %v", err)
		}
	})
	for _, want := range []string{"selena: dev", "targets:", "tools:", "naga"} {
		if !strings.Contains(output, want) {
			t.Fatalf("doctor output missing %q\n%s", want, output)
		}
	}
}

func examplePath(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join("..", "..", "examples", name)
}

func copyExample(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(examplePath(t, name))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() {
		os.Stdout = old
	}()
	fn()
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
