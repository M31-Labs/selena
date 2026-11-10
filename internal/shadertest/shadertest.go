// Package shadertest wraps the external shader validators (naga for WGSL,
// glslangValidator for GLSL) for tests. By default a missing tool skips the
// test. When SELENA_REQUIRE_VALIDATORS=1 is set, as in CI, a missing tool
// fails the test instead, so emitted shaders are never checked as golden
// text alone.
package shadertest

import (
	"fmt"
	"os"
	"os/exec"
	"testing"

	prismvalidate "m31labs.dev/prism/validate"
)

// EnvRequire is the environment variable that turns a missing tool into a
// test failure.
const EnvRequire = "SELENA_REQUIRE_VALIDATORS"

var installHint = map[string]string{
	"naga":             "cargo install naga-cli --version 29.0.3 --locked",
	"glslangValidator": "sudo apt-get install -y glslang-tools",
}

// Required reports whether missing validators must fail tests.
func Required() bool { return os.Getenv(EnvRequire) == "1" }

// Decide says what a test should do about a tool lookup result. found is
// whether the tool is on PATH and require is whether SELENA_REQUIRE_VALIDATORS
// is set. It returns fail=true with a message that names the tool and how to
// install it when the tool is missing and required. When the tool is missing
// and not required it returns fail=false with a skip message. When the tool
// is found it returns fail=false and an empty message.
func Decide(tool string, found, require bool) (fail bool, msg string) {
	if found {
		return false, ""
	}
	if !require {
		return false, fmt.Sprintf("%s: skipped (not on PATH)", tool)
	}
	hint := installHint[tool]
	if hint == "" {
		hint = "see CONTRIBUTING.md"
	}
	return true, fmt.Sprintf("%s not found on PATH but %s=1; install it (%s)", tool, EnvRequire, hint)
}

// Missing applies Decide to the current test: it fails or skips.
func Missing(t *testing.T, tool string) {
	t.Helper()
	fail, msg := Decide(tool, false, Required())
	if fail {
		t.Fatal(msg)
	}
	t.Skip(msg)
}

// Shader validates source with tool. A missing tool fails or skips as
// Decide describes; otherwise it defers to the prism helper.
func Shader(t *testing.T, tool, source, ext string, argv func(path string) []string) {
	t.Helper()
	_, err := exec.LookPath(tool)
	if fail, msg := Decide(tool, err == nil, Required()); fail {
		t.Fatal(msg)
	} else if msg != "" {
		t.Skip(msg)
	}
	prismvalidate.Shader(t, tool, source, ext, argv)
}
