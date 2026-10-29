package result_test

import (
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"m31labs.dev/selena"
	"m31labs.dev/selena/result"
)

// Each assignment compiles only if the two types are identical. These types
// would not be assignable if the root declared its own defined types.
var (
	_ result.Target           = selena.Target("")
	_ result.CompileOptions   = selena.CompileOptions{}
	_ result.FragmentArtifact = selena.FragmentArtifact{}
	_ result.Artifact         = selena.Artifact{}
	_ result.Result           = selena.Result{}
	_ selena.Target           = result.TargetWGSL
)

func TestRootNamesAreAliases(t *testing.T) {
	pairs := []struct {
		name       string
		root, leaf reflect.Type
	}{
		{"Target", reflect.TypeOf(selena.Target("")), reflect.TypeOf(result.Target(""))},
		{"CompileOptions", reflect.TypeOf(selena.CompileOptions{}), reflect.TypeOf(result.CompileOptions{})},
		{"FragmentArtifact", reflect.TypeOf(selena.FragmentArtifact{}), reflect.TypeOf(result.FragmentArtifact{})},
		{"Artifact", reflect.TypeOf(selena.Artifact{}), reflect.TypeOf(result.Artifact{})},
		{"Result", reflect.TypeOf(selena.Result{}), reflect.TypeOf(result.Result{})},
	}
	for _, p := range pairs {
		if p.root != p.leaf {
			t.Errorf("selena.%s is %v, want the identical type %v", p.name, p.root, p.leaf)
		}
	}
}

func TestRootConstantsMatchResult(t *testing.T) {
	pairs := []struct {
		name       string
		root, leaf result.Target
	}{
		{"TargetWGSL", selena.TargetWGSL, result.TargetWGSL},
		{"TargetGLSL", selena.TargetGLSL, result.TargetGLSL},
		{"TargetMetal", selena.TargetMetal, result.TargetMetal},
		{"TargetGLES", selena.TargetGLES, result.TargetGLES},
	}
	for _, p := range pairs {
		if p.root != p.leaf {
			t.Errorf("selena.%s = %q, want %q", p.name, p.root, p.leaf)
		}
	}
}

func TestRootAllTargetsForwardsToResult(t *testing.T) {
	root, leaf := selena.AllTargets(), result.AllTargets()
	if !reflect.DeepEqual(root, leaf) {
		t.Fatalf("selena.AllTargets() = %v, want %v", root, leaf)
	}
	root[0] = result.Target("mutated")
	if again := result.AllTargets(); again[0] != result.TargetWGSL {
		t.Fatalf("result.AllTargets()[0] = %q after editing the slice from selena.AllTargets()", again[0])
	}
}

// A host that imports only result can build options for the compiler and read
// what it returns, with no conversion at the boundary.
func TestCompileAcceptsAndReturnsResultTypes(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "examples", "textured.sel"))
	if err != nil {
		t.Fatal(err)
	}
	opts := result.CompileOptions{Targets: []result.Target{result.TargetWGSL, result.TargetGLES}}

	var res result.Result
	res, err = selena.Compile(src, opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Artifacts) != 2 {
		t.Fatalf("artifacts = %d, want the 2 requested targets", len(res.Artifacts))
	}
	wgsl, ok := res.Artifact(result.TargetWGSL)
	if !ok || wgsl.Source == "" {
		t.Fatalf("WGSL artifact = %+v, %v; want emitted source", wgsl, ok)
	}
	gles, ok := res.Artifact(result.TargetGLES)
	if !ok || gles.Vertex == "" || gles.Fragment == "" {
		t.Fatalf("GLES artifact = %+v, %v; want vertex and fragment source", gles, ok)
	}
	if _, ok := res.Artifact(result.TargetGLSL); ok {
		t.Fatal("GLSL artifact present although it was not requested")
	}
}

// The package documentation promises that package selena re-exports every name
// in result. Check the promise against the sources so a name added to result
// without a matching root declaration fails here.
func TestRootReexportsEveryResultName(t *testing.T) {
	leaf := exportedNames(t, ".")
	for _, want := range []string{
		"AllTargets", "Artifact", "CompileOptions", "FragmentArtifact", "Result",
		"Target", "TargetGLES", "TargetGLSL", "TargetMetal", "TargetWGSL",
	} {
		if !leaf[want] {
			t.Errorf("result does not declare %s; update this test if the move was intended", want)
		}
	}

	root := exportedNames(t, "..")
	for name := range leaf {
		if !root[name] {
			t.Errorf("package selena does not re-export result.%s", name)
		}
	}
}

// exportedNames returns the exported package-level names (types, constants,
// variables and functions, but not methods) declared in dir's non-test files.
func exportedNames(t *testing.T, dir string) map[string]bool {
	t.Helper()
	pkg, err := build.Default.ImportDir(dir, 0)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	names := map[string]bool{}
	fset := token.NewFileSet()
	for _, file := range pkg.GoFiles {
		f, err := parser.ParseFile(fset, filepath.Join(dir, file), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range f.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Recv == nil && d.Name.IsExported() {
					names[d.Name.Name] = true
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						if s.Name.IsExported() {
							names[s.Name.Name] = true
						}
					case *ast.ValueSpec:
						for _, n := range s.Names {
							if n.IsExported() {
								names[n.Name] = true
							}
						}
					}
				}
			}
		}
	}
	return names
}
