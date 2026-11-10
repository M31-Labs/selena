package shadertest

import (
	"strings"
	"testing"
)

func TestDecide(t *testing.T) {
	cases := []struct {
		name          string
		tool          string
		found, req    bool
		wantFail      bool
		wantInMessage []string
	}{
		{"found", "naga", true, true, false, nil},
		{"found not required", "naga", true, false, false, nil},
		{"missing not required skips", "naga", false, false, false, []string{"naga", "skipped"}},
		{"missing required fails naga", "naga", false, true, true, []string{"naga", "cargo install naga-cli", EnvRequire}},
		{"missing required fails glslang", "glslangValidator", false, true, true, []string{"glslangValidator", "apt-get install", "glslang-tools"}},
		{"unknown tool", "other", false, true, true, []string{"other", EnvRequire}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fail, msg := Decide(c.tool, c.found, c.req)
			if fail != c.wantFail {
				t.Fatalf("fail = %v, want %v (msg %q)", fail, c.wantFail, msg)
			}
			if c.found && msg != "" {
				t.Fatalf("found tool gave message %q", msg)
			}
			for _, w := range c.wantInMessage {
				if !strings.Contains(msg, w) {
					t.Errorf("message %q missing %q", msg, w)
				}
			}
		})
	}
}

func TestRequired(t *testing.T) {
	t.Setenv(EnvRequire, "1")
	if !Required() {
		t.Fatal("want required when set to 1")
	}
	t.Setenv(EnvRequire, "")
	if Required() {
		t.Fatal("want not required when empty")
	}
}
