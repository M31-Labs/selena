package lower

import (
	"errors"
	"strings"
	"testing"

	"m31labs.dev/selena/bindings"
	"m31labs.dev/selena/parse"
)

func lowerEngineSrc(t *testing.T, src string) (bindings.Layout, error) {
	t.Helper()
	program, err := parse.Program([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	_, layout, err := LowerProgram(program, 0)
	return layout, err
}

func TestEngineContextTypeMismatchIsADiagnostic(t *testing.T) {
	for name, src := range map[string]string{
		"wrong vector width": `material M { context { sunDir : vec4 } surface(geo) -> color { return rgb(sunDir.x, 0.0, 0.0) } }`,
		"wrong scalar":       `material M { context { sunDir : float } surface(geo) -> color { return rgb(sunDir, 0.0, 0.0) } }`,
		"array":              `material M { context { ambient : array<vec3, 2> } surface(geo) -> color { return rgb(1.0, 0.0, 0.0) } }`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := lowerEngineSrc(t, src)
			var de *DiagnosticError
			if !errors.As(err, &de) {
				t.Fatalf("err = %v, want a DiagnosticError", err)
			}
			if de.Code != CodeEngineInput {
				t.Fatalf("code = %s, want %s", de.Code, CodeEngineInput)
			}
			if !strings.Contains(de.Message, "engine input") {
				t.Fatalf("message = %q", de.Message)
			}
		})
	}
}

func TestEngineContextDescriptorSource(t *testing.T) {
	layout, err := lowerEngineSrc(t, `material M {
    context { sunDir : vec3  lightDir : vec3  exposure : float }
    surface(geo) -> color { return rgb(sunDir.x + lightDir.x, exposure, 0.0) }
}`)
	if err != nil {
		t.Fatal(err)
	}
	source := map[string]string{}
	class := map[string]string{}
	for _, f := range layout.UniformBlock.Fields {
		source[f.Name], class[f.Name] = f.Source, f.Class
	}
	if source["sunDir"] != "engine" || source["exposure"] != "engine" {
		t.Errorf("engine inputs not marked: %v", source)
	}
	if source["lightDir"] != "" || class["lightDir"] != "context" {
		t.Errorf("host-custom context field changed: source %q class %q", source["lightDir"], class["lightDir"])
	}
	if class["sunDir"] != "context" {
		t.Errorf("class = %q, want context kept", class["sunDir"])
	}
	if layout.EngineVersion != bindings.EngineVersion {
		t.Errorf("EngineVersion = %q, want %q", layout.EngineVersion, bindings.EngineVersion)
	}
}

func TestMaterialWithoutEngineInputsHasNoEngineVersion(t *testing.T) {
	layout, err := lowerEngineSrc(t, `material M {
    context { lightDir : vec3 }
    surface(geo) -> color { return rgb(lightDir.x, 0.0, 0.0) }
}`)
	if err != nil {
		t.Fatal(err)
	}
	if layout.EngineVersion != "" {
		t.Errorf("EngineVersion = %q, want empty", layout.EngineVersion)
	}
}

func TestEngineContextTypeCheckedInPointsMaterials(t *testing.T) {
	_, err := lowerEngineSrc(t, `material P kind points {
    context { exposure : vec3 }
    surface(pt) -> color { return rgb(pt.color.r, pt.color.g, pt.color.b, pt.alpha) }
}`)
	var de *DiagnosticError
	if !errors.As(err, &de) || de.Code != CodeEngineInput {
		t.Fatalf("err = %v, want SEL1007", err)
	}
}
