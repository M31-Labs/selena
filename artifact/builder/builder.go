// Package builder compiles a .sel source into an artifact.File and checks a
// shipped artifact file against its source. It links the Selena compiler and
// the material library; programs that only load artifacts import package
// artifact instead.
package builder

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"runtime/debug"
	"strings"

	"m31labs.dev/selena"
	"m31labs.dev/selena/artifact"
	"m31labs.dev/selena/bindings"
	"m31labs.dev/selena/hir"
	"m31labs.dev/selena/materiallib"
	"m31labs.dev/selena/parse"
	"m31labs.dev/selena/result"
)

const modulePath = "m31labs.dev/selena"

// Options controls Build.
type Options struct {
	// Path labels the source in the file and in errors. Use a slash-separated
	// path relative to the project so the file does not depend on the machine
	// that built it.
	Path string
	// Materials lists the materials to compile, in output order. Nil compiles
	// every material in declaration order.
	Materials []string
	// Targets lists the shader targets. Nil emits every target
	// (selena.AllTargets). An empty non-nil slice is an error: an artifact
	// without shaders cannot be rendered.
	Targets []result.Target
	// SpecularMRT adds the color/specular fragment variant to mesh materials.
	SpecularMRT bool
	// Modules lists materiallib modules to link before compiling. Linking a
	// module the source does not call changes no output.
	Modules []materiallib.Module
	// Compiler overrides the recorded Selena version. Empty records Version().
	Compiler string
}

// Version returns the Selena module version linked into the running program:
// the dependency version when Selena is a dependency, the main module version
// when it is the main module, or "devel" when the build carries no version.
func Version() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "devel"
	}
	if info.Main.Path == modulePath {
		return versionOrDevel(info.Main.Version)
	}
	for _, dep := range info.Deps {
		if dep.Path != modulePath {
			continue
		}
		if dep.Replace != nil {
			if dep.Replace.Version != "" {
				return dep.Replace.Version
			}
			return "devel"
		}
		return versionOrDevel(dep.Version)
	}
	return "devel"
}

func versionOrDevel(v string) string {
	if v == "" || v == "(devel)" {
		return "devel"
	}
	return v
}

// Build compiles source into an artifact file. The result is deterministic for
// the same source, options and Selena version.
func Build(source []byte, opts Options) (artifact.File, error) {
	targets := opts.Targets
	if targets == nil {
		targets = selena.AllTargets()
	}
	if len(targets) == 0 {
		return artifact.File{}, fmt.Errorf("selena artifact %s: no targets", label(opts.Path))
	}
	targets = append([]result.Target(nil), targets...)
	program, err := parse.Program(source)
	if err != nil {
		// Recompile through the root API so syntax errors keep their
		// CompileError diagnostics and source ranges.
		if _, cerr := selena.Compile(source, selena.CompileOptions{Targets: []selena.Target{}}); cerr != nil {
			return artifact.File{}, cerr
		}
		return artifact.File{}, err
	}
	if len(opts.Modules) > 0 {
		program, err = materiallib.Link(program, opts.Modules...)
		if err != nil {
			return artifact.File{}, fmt.Errorf("selena artifact %s: %w", label(opts.Path), err)
		}
	}
	if len(program.Materials) == 0 {
		return artifact.File{}, fmt.Errorf("selena artifact %s: no material declared", label(opts.Path))
	}
	names := opts.Materials
	if names == nil {
		names = materialNames(program)
	}
	if len(names) == 0 {
		return artifact.File{}, fmt.Errorf("selena artifact %s: no materials selected", label(opts.Path))
	}
	file := artifact.File{
		Schema:          artifact.Schema,
		Compiler:        opts.Compiler,
		LanguageVersion: bindings.LanguageVersion,
		Source:          artifact.Source{Path: opts.Path, SHA256: artifact.Hash(source)},
		Options:         artifact.Options{Targets: targets, SpecularMRT: opts.SpecularMRT, Modules: moduleNames(opts.Modules)},
		Default:         defaultName(program, names),
	}
	if file.Compiler == "" {
		file.Compiler = Version()
	}
	seen := make(map[string]bool, len(names))
	for _, name := range names {
		if seen[name] {
			return artifact.File{}, fmt.Errorf("selena artifact %s: material %q listed twice", label(opts.Path), name)
		}
		seen[name] = true
		res, err := selena.CompileProgram(program, selena.CompileOptions{
			Material:    name,
			SpecularMRT: opts.SpecularMRT,
			Targets:     targets,
		})
		if err != nil {
			return artifact.File{}, fmt.Errorf("selena artifact %s: material %s: %w", label(opts.Path), name, err)
		}
		file.Materials = append(file.Materials, artifact.FromResult(res))
	}
	if err := file.Validate(); err != nil {
		return artifact.File{}, err
	}
	return file, nil
}

// StaleError reports a shipped artifact file that no longer matches its
// source or no longer matches what the linked compiler emits.
type StaleError struct {
	Path   string
	Reason string
}

func (e *StaleError) Error() string {
	return fmt.Sprintf("selena artifact %s is stale: %s; rebuild it", label(e.Path), e.Reason)
}

// ErrStale matches every *StaleError with errors.Is.
var ErrStale = errors.New("selena artifact is stale")

// Is reports whether target is ErrStale.
func (e *StaleError) Is(target error) bool { return target == ErrStale }

// Check rebuilds source with the options recorded in shipped and returns a
// *StaleError when the source hash differs or when the rebuilt materials,
// layouts or shaders differ. The recorded compiler version is not compared:
// a new Selena version that emits the same output does not make a file stale.
func Check(shipped artifact.File, source []byte) error {
	if err := shipped.Validate(); err != nil {
		return err
	}
	if !shipped.Matches(source) {
		return &StaleError{Path: shipped.Source.Path, Reason: fmt.Sprintf("source sha256 is %s, artifact records %s", artifact.Hash(source), shipped.Source.SHA256)}
	}
	modules := make([]materiallib.Module, len(shipped.Options.Modules))
	for i, m := range shipped.Options.Modules {
		modules[i] = materiallib.Module(m)
	}
	rebuilt, err := Build(source, Options{
		Path:        shipped.Source.Path,
		Materials:   shipped.Names(),
		Targets:     shipped.Options.Targets,
		SpecularMRT: shipped.Options.SpecularMRT,
		Modules:     modules,
		Compiler:    shipped.Compiler,
	})
	if err != nil {
		return &StaleError{Path: shipped.Source.Path, Reason: "the source no longer compiles: " + err.Error()}
	}
	want, err := artifact.Encode(rebuilt)
	if err != nil {
		return err
	}
	got, err := artifact.Encode(shipped)
	if err != nil {
		return err
	}
	if !bytes.Equal(want, got) {
		return &StaleError{Path: shipped.Source.Path, Reason: firstDifference(shipped, rebuilt)}
	}
	return nil
}

func firstDifference(shipped, rebuilt artifact.File) string {
	if shipped.LanguageVersion != rebuilt.LanguageVersion {
		return fmt.Sprintf("language version %s, compiler has %s", shipped.LanguageVersion, rebuilt.LanguageVersion)
	}
	if shipped.Default != rebuilt.Default {
		return fmt.Sprintf("default material %q, source selects %q", shipped.Default, rebuilt.Default)
	}
	for i, m := range rebuilt.Materials {
		s := shipped.Materials[i]
		if encoded(s.Layout) != encoded(m.Layout) {
			return fmt.Sprintf("material %s: binding layout differs from what the compiler emits", m.Name)
		}
		for j, shader := range m.Shaders {
			if j >= len(s.Shaders) || encoded(s.Shaders[j]) != encoded(shader) {
				return fmt.Sprintf("material %s: %s shader differs from what the compiler emits", m.Name, shader.Target)
			}
		}
		if len(s.Shaders) != len(m.Shaders) {
			return fmt.Sprintf("material %s: has %d shaders, compiler emits %d", m.Name, len(s.Shaders), len(m.Shaders))
		}
	}
	return "content differs from what the compiler emits"
}

// encoded returns the JSON encoding of v for comparison; values here always
// encode.
func encoded(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func materialNames(p hir.Program) []string {
	out := make([]string, len(p.Materials))
	for i, m := range p.Materials {
		out[i] = m.Name
	}
	return out
}

// defaultName follows selena.Compile: the last declared material is the
// default. When that material was not selected, the last selected one is.
func defaultName(p hir.Program, selected []string) string {
	last := p.Materials[len(p.Materials)-1].Name
	for _, name := range selected {
		if name == last {
			return last
		}
	}
	return selected[len(selected)-1]
}

func moduleNames(modules []materiallib.Module) []string {
	if len(modules) == 0 {
		return nil
	}
	out := make([]string, len(modules))
	for i, m := range modules {
		out[i] = string(m)
	}
	return out
}

func label(path string) string {
	if strings.TrimSpace(path) == "" {
		return "<source>"
	}
	return path
}
