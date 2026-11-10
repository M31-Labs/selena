package materiallib

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"m31labs.dev/selena"
	"m31labs.dev/selena/bindings"
	"m31labs.dev/selena/emit/gles"
	"m31labs.dev/selena/emit/wgsl"
	"m31labs.dev/selena/hir"
	"m31labs.dev/selena/internal/shadertest"
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

// moduleContext is the context block a probe declares for a module whose
// helpers read engine inputs by name.
var moduleContext = map[Module]string{
	Lighting: `context {
    cameraPos       : vec3
    cameraForward   : vec3
    fog             : vec4
    lightCount      : float
    lights          : array<vec4, 32>
    shadow0         : vec4
    shadow1         : vec4
    shadowSplits0   : vec4
    shadowSplits1   : vec4
    shadowMatrices0 : array<vec4, 16>
    shadowMatrices1 : array<vec4, 16>
    shadowMap0      : textureDepth2dArray
    shadowMap1      : textureDepth2dArray
}
`,
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
				surfaceProbe := f.Returns == hir.SurfaceT
				for i, p := range f.Params {
					name := fmt.Sprintf("arg%d", i)
					if p.Type == hir.SurfaceT {
						// A Surface argument is built from dynamic params.
						surfaceProbe = true
						declarations = append(declarations, fmt.Sprintf("param %sc : color\nparam %sr : float", name, name))
						args = append(args, fmt.Sprintf("pbr(%sc, %sr, %sr)", name, name, name))
						continue
					}
					declarations = append(declarations, fmt.Sprintf("param %s : %s", name, p.Type))
					args = append(args, name)
				}
				value := f.Name + "(" + strings.Join(args, ", ") + ")"
				if surfaceProbe {
					validateSurfaceProbe(t, module, declarations, value, f.Returns == hir.SurfaceT)
					return
				}
				switch f.Returns {
				case "int":
					value = "rgb(float(" + value + "), 0.0, 0.0)"
				case "float":
					value = "rgb(" + value + ", 0.0, 0.0)"
				case "vec2":
					value = "vec3f(" + value + ", 0.0)"
				}
				source := []byte("material Probe {\n" + moduleContext[module] + strings.Join(declarations, "\n") + "\nsurface(geo) -> color { return " + value + " }\n}")
				res, err := Compile(source, selena.CompileOptions{}, module)
				if err != nil {
					t.Fatal(err)
				}
				validateArtifacts(t, res)
			})
		}
	}
}

// validateSurfaceProbe compiles a surface-kind probe around a helper that
// takes or returns a Surface, then validates both hooks in their harnesses.
func validateSurfaceProbe(t *testing.T, module Module, declarations []string, value string, returnsSurface bool) {
	t.Helper()
	if !returnsSurface {
		value = "pbr(rgb(" + value + ", 0.0, 0.0), 0.5, 0.0)"
	}
	source := []byte("material Probe {\n" + strings.Join(declarations, "\n") + "\nsurface(geo) -> Surface { return " + value + " }\n}")
	res, err := Compile(source, selena.CompileOptions{}, module)
	if err != nil {
		t.Fatal(err)
	}
	w, ok := res.Artifact(selena.TargetWGSL)
	g, ok2 := res.Artifact(selena.TargetGLES)
	if !ok || !ok2 || len(res.Artifacts) != 2 {
		t.Fatalf("surface probe artifacts: %d", len(res.Artifacts))
	}
	shadertest.Shader(t, "naga", wgsl.SurfaceHarness(w.Source, len(res.Layout.Textures)), ".wgsl", nil)
	shadertest.Shader(t, "glslangValidator", gles.SurfaceHarness(g.Fragment, len(res.Layout.Textures)), ".frag", nil)
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
			shadertest.Shader(t, "naga", a.Source, ".wgsl", nil)
		case selena.TargetGLSL, selena.TargetGLES:
			shadertest.Shader(t, "glslangValidator", a.Vertex, ".vert", nil)
			shadertest.Shader(t, "glslangValidator", a.Fragment, ".frag", nil)
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
			if res.Layout.Kind == bindings.SurfaceKindSurface {
				w, _ := res.Artifact(selena.TargetWGSL)
				g, _ := res.Artifact(selena.TargetGLES)
				shadertest.Shader(t, "naga", wgsl.SurfaceHarness(w.Source, len(res.Layout.Textures)), ".wgsl", nil)
				shadertest.Shader(t, "glslangValidator", gles.SurfaceHarness(g.Fragment, len(res.Layout.Textures)), ".frag", nil)
				return
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
