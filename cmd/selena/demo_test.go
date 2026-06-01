package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDemoUsesDescriptorDefaults(t *testing.T) {
	out := filepath.Join(t.TempDir(), "defaults.html")
	if err := runDemo(out, "defaults"); err != nil {
		t.Fatal(err)
	}
	html, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	got := string(html)
	for _, want := range []string{
		`"name": "baseColor"`,
		`"name": "gain"`,
		`var MATERIAL = {};`,
		`var DEFAULTS = {};`,
		`DEFAULTS[d.name] = d.values;`,
		`uniformValue(values, f)`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("demo output missing %q", want)
		}
	}
}
