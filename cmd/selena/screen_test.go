package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func screenSourcePath(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "testdata", "conformance", "screen-meter-strip.sel")
}

func TestInspectPrintsScreenPipeline(t *testing.T) {
	out := captureStdout(t, func() {
		if err := run([]string{"inspect", screenSourcePath(t)}); err != nil {
			t.Fatalf("inspect: %v", err)
		}
	})
	for _, want := range []string{
		"pipeline: blend additive, depth test false, depth write false",
		`"kind": "screen"`,
		`"pipeline": {`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("inspect output lacks %q:\n%s", want, out)
		}
	}
}

func TestInspectOmitsPipelineForOtherKinds(t *testing.T) {
	out := captureStdout(t, func() {
		if err := run([]string{"inspect", examplePath(t, "directional-diffuse.sel")}); err != nil {
			t.Fatalf("inspect: %v", err)
		}
	})
	if strings.Contains(out, "pipeline") {
		t.Errorf("a mesh material's inspect output mentions a pipeline:\n%s", out)
	}
}

func TestCheckAcceptsScreenMaterial(t *testing.T) {
	out := captureStdout(t, func() {
		if err := run([]string{"check", screenSourcePath(t)}); err != nil {
			t.Fatalf("check: %v", err)
		}
	})
	if !strings.Contains(out, "MeterStrip") {
		t.Errorf("check output = %q", out)
	}
}
