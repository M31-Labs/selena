//go:build !js

package result_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const (
	modulePath = "m31labs.dev/selena"
	leafPath   = modulePath + "/result"
)

// allowedModulePackages lists every package outside the standard library that
// the leaf may reach, directly or through its imports. The compiler (parse,
// lower, emit/... and validate), the tree-sitter grammar and GoSX are absent on
// purpose: the leaf exists so a browser WASM module can read compile results
// without linking them.
var allowedModulePackages = map[string]bool{
	leafPath:                 true,
	modulePath + "/bindings": true,
	modulePath + "/hir":      true,
	modulePath + "/ir":       true,
}

// deniedStandardPackages lists standard-library packages that the compiler
// links and that the leaf must not. Their package initializers run in a
// browser module even when nothing calls them.
var deniedStandardPackages = map[string]bool{
	"encoding/gob": true,
	"go/parser":    true,
	"regexp":       true,
}

// disallowed explains why importPath may not appear in the leaf's imports, or
// returns "" when it may.
func disallowed(importPath string, standard bool) string {
	switch {
	case standard && deniedStandardPackages[importPath]:
		return "standard-library package that the compiler links and the leaf must not"
	case !standard && !allowedModulePackages[importPath]:
		return "not a leaf package (compiler, grammar or third-party code)"
	}
	return ""
}

// isStandard reports whether importPath names a standard-library package: its
// first element has no dot.
func isStandard(importPath string) bool {
	first, _, _ := strings.Cut(importPath, "/")
	return !strings.Contains(first, ".")
}

type listedPackage struct {
	ImportPath string
	Standard   bool
	Imports    []string
}

// listDeps runs go list -deps for the leaf package with extra environment
// settings and returns every package in its import closure, the leaf included.
func listDeps(t *testing.T, goTool string, env []string) []listedPackage {
	t.Helper()
	cmd := exec.Command(goTool, "list", "-deps", "-json=ImportPath,Standard,Imports", ".")
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			t.Fatalf("go list -deps: %v\n%s", err, exit.Stderr)
		}
		t.Fatalf("go list -deps: %v", err)
	}
	var pkgs []listedPackage
	dec := json.NewDecoder(bytes.NewReader(out))
	for dec.More() {
		var p listedPackage
		if err := dec.Decode(&p); err != nil {
			t.Fatalf("decode go list output: %v", err)
		}
		pkgs = append(pkgs, p)
	}
	return pkgs
}

// TestLeafImportClosureExcludesCompiler fails if the import closure of package
// result, on the host or for js/wasm, contains the compiler, the tree-sitter
// grammar, any other third-party package, or a denied standard-library package.
func TestLeafImportClosureExcludesCompiler(t *testing.T) {
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go tool not on PATH")
	}
	platforms := []struct {
		name string
		env  []string
	}{
		{"host", nil},
		{"js-wasm", []string{"GOOS=js", "GOARCH=wasm"}},
	}
	for _, platform := range platforms {
		t.Run(platform.name, func(t *testing.T) {
			pkgs := listDeps(t, goTool, platform.env)

			importedBy := map[string][]string{}
			sawLeaf := false
			for _, p := range pkgs {
				sawLeaf = sawLeaf || p.ImportPath == leafPath
				for _, imported := range p.Imports {
					importedBy[imported] = append(importedBy[imported], p.ImportPath)
				}
			}
			if !sawLeaf {
				t.Fatalf("go list -deps did not report %s, so this test is not checking the leaf package", leafPath)
			}

			for _, p := range pkgs {
				if why := disallowed(p.ImportPath, p.Standard); why != "" {
					importers := importedBy[p.ImportPath]
					sort.Strings(importers)
					t.Errorf("%s is in the import closure of %s: %s (imported by %s)",
						p.ImportPath, leafPath, why, strings.Join(importers, ", "))
				}
			}
		})
	}
}

// TestLeafSourcesImportOnlyLeafPackages checks the direct imports of the leaf
// and of the three packages it builds on. It parses every non-test source file
// in those directories and ignores build constraints, so a file behind an
// unusual tag or file-name suffix cannot import the compiler unseen, and it
// runs without the go tool.
func TestLeafSourcesImportOnlyLeafPackages(t *testing.T) {
	fset := token.NewFileSet()
	for _, dir := range []string{".", "../bindings", "../hir", "../ir"} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read %s: %v", dir, err)
		}
		for _, entry := range entries {
			if !isLibrarySource(entry) {
				continue
			}
			path := filepath.Join(dir, entry.Name())
			file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
			if err != nil {
				t.Fatalf("parse %s: %v", path, err)
			}
			// A file in package main cannot belong to a library package. It is
			// a go:generate helper that an "ignore" tag keeps out of every build.
			if file.Name.Name == "main" {
				continue
			}
			for _, spec := range file.Imports {
				imported, err := strconv.Unquote(spec.Path.Value)
				if err != nil {
					t.Fatalf("%s: import path %s: %v", path, spec.Path.Value, err)
				}
				if why := disallowed(imported, isStandard(imported)); why != "" {
					t.Errorf("%s imports %s: %s", path, imported, why)
				}
			}
		}
	}
}

// isLibrarySource reports whether entry is a Go file that the go tool could
// compile into a package under some build constraint: not a directory, not a
// test file, and not a name the go tool always skips (a leading "_" or ".").
func isLibrarySource(entry os.DirEntry) bool {
	name := entry.Name()
	return !entry.IsDir() &&
		strings.HasSuffix(name, ".go") &&
		!strings.HasSuffix(name, "_test.go") &&
		!strings.HasPrefix(name, "_") &&
		!strings.HasPrefix(name, ".")
}
