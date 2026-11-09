package bindings

import (
	"os"
	"strings"
	"testing"
)

func TestEngineRegistryIsWellFormed(t *testing.T) {
	if EngineVersion == "" {
		t.Fatal("EngineVersion is empty")
	}
	seen := map[string]bool{}
	for _, in := range EngineInputs() {
		if in.Name == "" || in.Type == "" || in.Doc == "" || in.Space == "" {
			t.Errorf("incomplete registry entry %+v", in)
		}
		if in.Kind != EngineValue && in.Kind != EngineTexture {
			t.Errorf("%s: kind %q", in.Name, in.Kind)
		}
		if seen[in.Name] {
			t.Errorf("duplicate registry name %s", in.Name)
		}
		seen[in.Name] = true
		if got, ok := LookupEngineInput(in.Name); !ok || got != in {
			t.Errorf("Lookup(%s) = %+v, %v", in.Name, got, ok)
		}
	}
	for _, name := range []string{"cameraPos", "sunDir", "sunColor", "ambient", "fog", "exposure", "time",
		"environmentInfo", "skyBetaR", "skyBetaM", "skySun", "skyG", "planarReflection", "opaqueColor", "opaqueDepth"} {
		if !seen[name] {
			t.Errorf("registry lacks %s", name)
		}
	}
	if _, ok := LookupEngineInput("lightDir"); ok {
		t.Error("lightDir is a host-custom context name, not an engine input")
	}
}

func TestEngineInputsReturnsACopy(t *testing.T) {
	a := EngineInputs()
	a[0].Name = "mutated"
	if EngineInputs()[0].Name == "mutated" {
		t.Fatal("EngineInputs exposes the registry slice")
	}
}

func TestEngineInputsDocumentedInReference(t *testing.T) {
	doc, err := os.ReadFile("../docs/engine-inputs.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, in := range EngineInputs() {
		if !strings.Contains(string(doc), "| `"+in.Name+"` | `"+in.Type+"` |") {
			t.Errorf("docs/engine-inputs.md does not list %s as %s", in.Name, in.Type)
		}
	}
	if !strings.Contains(string(doc), EngineVersion) {
		t.Errorf("docs/engine-inputs.md does not name %s", EngineVersion)
	}
}
