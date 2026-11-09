// Package artifact defines the Selena artifact file: the compiled form of one
// .sel source, written at build time and loaded at run time without the
// compiler.
//
// An artifact file holds, for every material it records, the host binding
// layout (the selena.descriptor.v1 JSON a renderer binds from) and the emitted
// shader source for each target. It also records the SHA-256 of the source
// bytes, the build options, the language version and the Selena version that
// wrote it, so a build or a test can tell when the file no longer matches its
// source.
//
// This is a leaf package. It imports only bindings, hir, ir and result, never
// the parser, the lowerer, the emitters or the tree-sitter grammar, so a browser
// WASM module can decode an artifact and hand its layout and shaders to a
// renderer without linking the compiler. Package artifact/builder writes and
// checks artifact files; it links the compiler.
//
// Encode is deterministic: the same File always encodes to the same bytes, and
// builder.Build produces the same File for the same source, options and Selena
// version.
package artifact

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"m31labs.dev/selena/bindings"
	"m31labs.dev/selena/hir"
	"m31labs.dev/selena/result"
)

// Schema identifies the artifact file format. Decode rejects any other value.
// Adding an optional field keeps the schema; renaming, removing or changing the
// meaning of a field bumps it.
const Schema = "selena.artifact.v1"

// Extension is the suffix PathFor appends to a source path.
const Extension = ".json"

// File is one decoded artifact file.
type File struct {
	// Schema is always Schema.
	Schema string `json:"schema"`
	// Compiler is the Selena module version that wrote the file, such as
	// "v0.6.0" or a pseudo-version, or "devel" for an unversioned build. It
	// records provenance only: builder.Check compares content, not this field.
	Compiler string `json:"compiler"`
	// LanguageVersion is bindings.LanguageVersion of the compiler that wrote the
	// file.
	LanguageVersion string `json:"languageVersion"`
	// Source identifies the .sel source the file was compiled from.
	Source Source `json:"source"`
	// Options are the build options, so a checker can rebuild the same file.
	Options Options `json:"options"`
	// Default names the material a host gets when it asks for none: the last
	// material in the source, which follows Selena's convention that a derived
	// material follows its base.
	Default string `json:"default"`
	// Materials lists the compiled materials in source declaration order.
	Materials []Material `json:"materials"`
}

// Source identifies the bytes a file was compiled from.
type Source struct {
	// Path is the source path the builder was given, slash-separated. It is a
	// label for diagnostics; checks use SHA256.
	Path string `json:"path,omitempty"`
	// SHA256 is the lowercase hex SHA-256 of the source bytes (see Hash).
	SHA256 string `json:"sha256"`
}

// Options records how the file was built.
type Options struct {
	// Targets lists the emitted shader targets in emission order.
	Targets []result.Target `json:"targets"`
	// SpecularMRT is true when mesh materials carry the additive color/specular
	// fragment variant (see selena.CompileOptions).
	SpecularMRT bool `json:"specularMRT,omitempty"`
	// Modules lists the materiallib modules linked into the program before
	// compiling, in the order given.
	Modules []string `json:"modules,omitempty"`
}

// Material is one compiled material.
type Material struct {
	Name string `json:"name"`
	// Layout is the host binding descriptor (selena.descriptor.v1).
	Layout bindings.Layout `json:"layout"`
	// Shaders holds one entry per target in Options.Targets order.
	Shaders []Shader `json:"shaders"`
}

// Shader is the emitted source for one target. WGSL and Metal use Source;
// GLSL and GLES use Vertex and Fragment.
type Shader struct {
	Target   result.Target `json:"target"`
	Source   string        `json:"source,omitempty"`
	Vertex   string        `json:"vertex,omitempty"`
	Fragment string        `json:"fragment,omitempty"`
	// SpecularMRT is the additive color/specular fragment variant, present only
	// when Options.SpecularMRT is set and the material is a mesh material.
	SpecularMRT *Variant `json:"specularMRT,omitempty"`
}

// Variant is an additive fragment variant. WGSL and Metal carry the full module
// in Source; GLSL and GLES carry Fragment and reuse the Shader's Vertex.
type Variant struct {
	Source   string `json:"source,omitempty"`
	Fragment string `json:"fragment,omitempty"`
}

// Hash returns the lowercase hex SHA-256 of source, the value File.Source.SHA256
// records.
func Hash(source []byte) string {
	sum := sha256.Sum256(source)
	return hex.EncodeToString(sum[:])
}

// PathFor returns the conventional artifact path for a source path:
// "materials/rock.sel" becomes "materials/rock.sel.json".
func PathFor(sourcePath string) string { return sourcePath + Extension }

// Matches reports whether f was compiled from exactly these source bytes. It
// does not prove that the current compiler would emit the same shaders; use
// builder.Check for that.
func (f File) Matches(source []byte) bool { return f.Source.SHA256 == Hash(source) }

// Material returns the material called name, or the Default material when name
// is empty.
func (f File) Material(name string) (Material, bool) {
	if name == "" {
		name = f.Default
	}
	for _, m := range f.Materials {
		if m.Name == name {
			return m, true
		}
	}
	return Material{}, false
}

// Names returns the material names in declaration order.
func (f File) Names() []string {
	out := make([]string, len(f.Materials))
	for i, m := range f.Materials {
		out[i] = m.Name
	}
	return out
}

// Result returns the material called name (the Default when empty) as a
// result.Result that carries only the material name, the layout and the
// artifacts. The parsed program and the IR are not stored in an artifact file.
func (f File) Result(name string) (result.Result, error) {
	m, ok := f.Material(name)
	if !ok {
		if name == "" {
			name = f.Default
		}
		return result.Result{}, fmt.Errorf("selena artifact %s: no material %q (have %s)", f.label(), name, strings.Join(f.Names(), ", "))
	}
	return m.Result(), nil
}

// Result returns m as a result.Result holding the material name, the layout and
// the artifacts.
func (m Material) Result() result.Result {
	return result.Result{
		Material:  hir.Material{Name: m.Name},
		Layout:    m.Layout,
		Artifacts: m.Artifacts(),
	}
}

// Artifacts converts m's shaders to result.Artifact values in Shaders order.
func (m Material) Artifacts() []result.Artifact {
	out := make([]result.Artifact, len(m.Shaders))
	for i, s := range m.Shaders {
		out[i] = result.Artifact{Target: s.Target, Source: s.Source, Vertex: s.Vertex, Fragment: s.Fragment}
		if s.SpecularMRT != nil {
			out[i].SpecularMRT = &result.FragmentArtifact{Source: s.SpecularMRT.Source, Fragment: s.SpecularMRT.Fragment}
		}
	}
	return out
}

// FromResult converts a compile result to a Material. It copies the layout and
// the artifacts and drops the parsed program and the IR.
func FromResult(r result.Result) Material {
	name := r.Material.Name
	if name == "" {
		name = r.Layout.Material
	}
	m := Material{Name: name, Layout: r.Layout, Shaders: make([]Shader, len(r.Artifacts))}
	for i, a := range r.Artifacts {
		m.Shaders[i] = Shader{Target: a.Target, Source: a.Source, Vertex: a.Vertex, Fragment: a.Fragment}
		if a.SpecularMRT != nil {
			m.Shaders[i].SpecularMRT = &Variant{Source: a.SpecularMRT.Source, Fragment: a.SpecularMRT.Fragment}
		}
	}
	return m
}

// Validate reports the first structural problem in f: a wrong schema, a
// malformed hash, no materials, an unnamed or duplicate material, a layout
// whose material name differs, a Default that names no material, or a shader
// for a target outside Options.Targets.
func (f File) Validate() error {
	if f.Schema != Schema {
		return fmt.Errorf("selena artifact %s: schema %q, want %q", f.label(), f.Schema, Schema)
	}
	if len(f.Source.SHA256) != sha256.Size*2 || strings.ToLower(f.Source.SHA256) != f.Source.SHA256 {
		return fmt.Errorf("selena artifact %s: source sha256 %q is not 64 lowercase hex digits", f.label(), f.Source.SHA256)
	}
	if _, err := hex.DecodeString(f.Source.SHA256); err != nil {
		return fmt.Errorf("selena artifact %s: source sha256: %w", f.label(), err)
	}
	if len(f.Materials) == 0 {
		return fmt.Errorf("selena artifact %s: no materials", f.label())
	}
	targets := make(map[result.Target]bool, len(f.Options.Targets))
	for _, t := range f.Options.Targets {
		targets[t] = true
	}
	seen := make(map[string]bool, len(f.Materials))
	for _, m := range f.Materials {
		switch {
		case m.Name == "":
			return fmt.Errorf("selena artifact %s: material with no name", f.label())
		case seen[m.Name]:
			return fmt.Errorf("selena artifact %s: duplicate material %q", f.label(), m.Name)
		case m.Layout.Material != m.Name:
			return fmt.Errorf("selena artifact %s: material %q has layout for %q", f.label(), m.Name, m.Layout.Material)
		}
		seen[m.Name] = true
		for _, s := range m.Shaders {
			if !targets[s.Target] {
				return fmt.Errorf("selena artifact %s: material %q has a %q shader, but options list targets %v", f.label(), m.Name, s.Target, f.Options.Targets)
			}
		}
	}
	if !seen[f.Default] {
		return fmt.Errorf("selena artifact %s: default material %q is not in the file", f.label(), f.Default)
	}
	return nil
}

// Encode validates f and returns its canonical encoding: indented JSON with
// shader source left unescaped, ending in a newline. Equal Files encode to equal
// bytes.
func Encode(f File) ([]byte, error) {
	if err := f.Validate(); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(f); err != nil {
		return nil, fmt.Errorf("selena artifact %s: %w", f.label(), err)
	}
	return buf.Bytes(), nil
}

// Decode parses and validates an artifact file.
func Decode(data []byte) (File, error) {
	var f File
	if err := json.Unmarshal(data, &f); err != nil {
		return File{}, fmt.Errorf("selena artifact: %w", err)
	}
	if err := f.Validate(); err != nil {
		return File{}, err
	}
	return f, nil
}

func (f File) label() string {
	if f.Source.Path != "" {
		return f.Source.Path
	}
	if f.Source.SHA256 != "" && len(f.Source.SHA256) >= 12 {
		return "sha256:" + f.Source.SHA256[:12]
	}
	return "<unnamed>"
}
