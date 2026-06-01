package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckRendersDiagnosticSnippet(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "bad.sel")
	src := `material Bad {
    surface(geo) -> color {
        return missing
    }
}
`
	if err := os.WriteFile(file, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	err := run([]string{"check", file})
	if err == nil {
		t.Fatal("check succeeded, want diagnostic error")
	}
	var se *sourceError
	if !errors.As(err, &se) {
		t.Fatalf("error type = %T, want sourceError", err)
	}

	var out strings.Builder
	printCommandError(&out, err)
	got := out.String()
	for _, want := range []string{
		"selena: SEL2001 at 3:16: unknown name \"missing\"",
		"--> " + file + ":3:16",
		"3 |         return missing",
		"|                ^^^^^^^ unknown name \"missing\"",
		"hint: Declare a material param or let binding with this name, or correct the identifier.",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("diagnostic output missing %q\n--- got ---\n%s", want, got)
		}
	}
}
