package materiallib

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	prismvalidate "m31labs.dev/prism/validate"
	"m31labs.dev/selena"
	"m31labs.dev/selena/parse"
)

func modules(t *testing.T) []Module {
	t.Helper()
	files, err := sources.ReadDir("modules")
	if err != nil {
		t.Fatal(err)
	}
	var out []Module
	for _, f := range files {
		out = append(out, Module(strings.TrimSuffix(f.Name(), ".sel")))
	}
	return out
}

// Every helper is called with dynamic parameters so validators cannot hide
// an invalid implementation behind constant folding or an unused branch.
func TestEveryHelperConformsAcrossTargets(t *testing.T) {
	for _, module := range modules(t) {
		functions, err := Functions(module)
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range functions {
			t.Run(string(module)+"/"+f.Name, func(t *testing.T) {
				var declarations, args []string
				for i, p := range f.Params {
					name := fmt.Sprintf("arg%d", i)
					declarations = append(declarations, fmt.Sprintf("param %s : %s", name, p.Type))
					args = append(args, name)
				}
				value := f.Name + "(" + strings.Join(args, ", ") + ")"
				switch f.Returns {
				case "float":
					value = "rgb(" + value + ", 0.0, 0.0)"
				case "vec2":
					value = "vec3f(" + value + ", 0.0)"
				}
				source := []byte("material Probe {\n" + strings.Join(declarations, "\n") + "\nsurface(geo) -> color { return " + value + " }\n}")
				res, err := Compile(source, selena.CompileOptions{}, module)
				if err != nil {
					t.Fatal(err)
				}
				validateArtifacts(t, res)
			})
		}
	}
}

func validateArtifacts(t *testing.T, res selena.Result) {
	t.Helper()
	if len(res.Artifacts) != 4 {
		t.Fatalf("artifacts: %d", len(res.Artifacts))
	}
	if res.Layout.SchemaVersion != "selena.descriptor.v1" || res.Layout.LanguageVersion != "selena.lang.v1" {
		t.Fatal("descriptor contract changed")
	}
	for _, a := range res.Artifacts {
		switch a.Target {
		case selena.TargetWGSL:
			prismvalidate.Shader(t, "naga", a.Source, ".wgsl", nil)
		case selena.TargetGLSL, selena.TargetGLES:
			prismvalidate.Shader(t, "glslangValidator", a.Vertex, ".vert", nil)
			prismvalidate.Shader(t, "glslangValidator", a.Fragment, ".frag", nil)
		case selena.TargetMetal:
			if !strings.Contains(a.Source, "fragmentMain") {
				t.Fatal("missing Metal fragment entry")
			}
		}
	}
}

func TestExamplesConformAcrossTargets(t *testing.T) {
	files, err := filepath.Glob("../examples/material-library/*.sel")
	if err != nil || len(files) == 0 {
		t.Fatal("missing library examples", err)
	}
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			source, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			res, err := Compile(source, selena.CompileOptions{}, modules(t)...)
			if err != nil {
				t.Fatal(err)
			}
			validateArtifacts(t, res)
		})
	}
}

func TestCompositionCompatibility(t *testing.T) {
	source := []byte("material Plain { param tint: color = rgb(0.3, 0.4, 0.5) surface(geo) -> color { return tint } }")
	want, err := selena.Compile(source, selena.CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	got, err := Compile(source, selena.CompileOptions{}, modules(t)...)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Layout, want.Layout) || !reflect.DeepEqual(got.Artifacts, want.Artifacts) {
		t.Fatal("unused modules change output")
	}
	program, _ := parse.Program(source)
	linked, err := Link(program, BRDF, BRDF)
	if err != nil {
		t.Fatal(err)
	}
	if len(program.Funcs) != 0 {
		t.Fatal("Link mutated input")
	}
	if _, err := Link(linked, BRDF); err == nil {
		t.Fatal("shadowing should fail")
	}
	if _, err := Functions(Module("../brdf")); err == nil {
		t.Fatal("unknown module should fail")
	}
}

func TestLibraryTypesAndCallSiteDiagnostics(t *testing.T) {
	for _, call := range []string{"mlPow5(vec2f(1.0, 2.0))", "mlPow5()"} {
		_, err := Compile([]byte("material Bad {\nsurface(geo) -> color {\nreturn rgb("+call+", 0.0, 0.0)\n}\n}"), selena.CompileOptions{}, BRDF)
		if err == nil {
			t.Fatal("invalid call accepted", call)
		}
		compileError, ok := err.(*selena.CompileError)
		if !ok || len(compileError.Diagnostics) == 0 || compileError.Diagnostics[0].Range.Start.Line != 3 {
			t.Fatalf("lost call-site diagnostic: %v", err)
		}
	}
}

func TestBindingsStayInsideBranchesAndLoopHeaders(t *testing.T) {
	source := []byte(`material Scope {
surface(geo) -> color {
    let selenaLib0 = 0.25
    var result = 0.0
    for (var i = 0.0; mlPow5(i) < 2.0; i = mlPow5(i) + 1.0) {
        if (i > 0.0) { result = result + mlPow5(selenaLib0) }
    }
    return rgb(result, 0.0, 0.0)
}}`)
	res, err := Compile(source, selena.CompileOptions{}, BRDF)
	if err != nil {
		t.Fatal(err)
	}
	validateArtifacts(t, res)
}
