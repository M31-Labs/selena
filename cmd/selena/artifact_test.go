package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"m31labs.dev/selena/artifact"
	"m31labs.dev/selena/artifact/builder"
)

func TestArtifactCommandWritesAndChecks(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "tinted.sel")
	data, err := os.ReadFile(examplePath(t, "tinted.sel"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, data, 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := artifactCommand([]string{"-targets", "wgsl,gles", src}, &out); err != nil {
		t.Fatal(err)
	}
	written, err := os.ReadFile(src + ".json")
	if err != nil {
		t.Fatalf("default output path: %v", err)
	}
	file, err := artifact.Decode(written)
	if err != nil {
		t.Fatal(err)
	}
	if file.Source.Path != "tinted.sel" || len(file.Options.Targets) != 2 || file.Compiler != builder.Version() {
		t.Fatalf("path %q targets %v compiler %q", file.Source.Path, file.Options.Targets, file.Compiler)
	}

	out.Reset()
	if err := artifactCommand([]string{"-targets", "wgsl,gles", src}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "unchanged") {
		t.Fatalf("second write output %q, want unchanged", out.String())
	}

	out.Reset()
	if err := artifactCommand([]string{"-check", src}, &out); err != nil {
		t.Fatalf("fresh artifact failed -check: %v", err)
	}

	if err := os.WriteFile(src, append(data, []byte("\n// edit\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	err = artifactCommand([]string{"-check", src}, &out)
	if err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("-check after an edit = %v, want a stale error", err)
	}
}

func TestArtifactCommandStdoutAndErrors(t *testing.T) {
	var out bytes.Buffer
	if err := artifactCommand([]string{"-o", "-", "-materials", "Base", "-path", "x/tinted.sel", examplePath(t, "tinted.sel")}, &out); err != nil {
		t.Fatal(err)
	}
	file, err := artifact.Decode(out.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if file.Source.Path != "x/tinted.sel" || file.Default != "Base" {
		t.Fatalf("path %q default %q", file.Source.Path, file.Default)
	}
	for _, args := range [][]string{
		{},
		{"-targets", "hlsl", examplePath(t, "tinted.sel")},
		{"-check", "-o", "-", examplePath(t, "tinted.sel")},
		{"-o", "-", "-modules", "missing", examplePath(t, "tinted.sel")},
	} {
		if err := artifactCommand(args, &out); err == nil {
			t.Errorf("artifact %v succeeded", args)
		}
	}
}
