package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"sort"
	"strings"
	"time"

	prismvalidate "m31labs.dev/prism/validate"
	"m31labs.dev/selena"
	"m31labs.dev/selena/ir"
)

type graphReport struct {
	GraphVersion int            `json:"graph_version"`
	SourcePath   string         `json:"source_path,omitempty"`
	Material     string         `json:"material"`
	Counts       graphCounts    `json:"counts"`
	HIR          hirSnapshot    `json:"hir"`
	IR           irSnapshot     `json:"ir"`
	Layout       any            `json:"layout"`
	Artifacts    []artifactInfo `json:"artifacts"`
}

type graphCounts struct {
	Functions  int `json:"functions"`
	Materials  int `json:"materials"`
	Uniforms   int `json:"uniforms"`
	Attributes int `json:"attributes"`
	Varyings   int `json:"varyings"`
	Textures   int `json:"textures"`
	Artifacts  int `json:"artifacts"`
}

type hirSnapshot struct {
	Functions []string `json:"functions,omitempty"`
	Materials []string `json:"materials,omitempty"`
	Selected  string   `json:"selected"`
	Extends   string   `json:"extends,omitempty"`
	Params    []string `json:"params,omitempty"`
}

type irSnapshot struct {
	Name       string    `json:"name"`
	Uniforms   []binding `json:"uniforms,omitempty"`
	Attributes []binding `json:"attributes,omitempty"`
	Varyings   []binding `json:"varyings,omitempty"`
	Textures   []string  `json:"textures,omitempty"`
	Vertex     stageInfo `json:"vertex"`
	Fragment   stageInfo `json:"fragment"`
}

type binding struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type stageInfo struct {
	Statements int    `json:"statements"`
	Output     string `json:"output"`
}

type artifactInfo struct {
	Target        selena.Target `json:"target"`
	SourceBytes   int           `json:"source_bytes,omitempty"`
	VertexBytes   int           `json:"vertex_bytes,omitempty"`
	FragmentBytes int           `json:"fragment_bytes,omitempty"`
}

func graph(args []string) error {
	fs := flag.NewFlagSet("graph", flag.ContinueOnError)
	format := fs.String("format", "json", "output format: json or dot")
	if err := fs.Parse(args); err != nil {
		return err
	}
	file, material, err := parseInputMaterial(fs.Args(), "selena graph [--format json|dot] <file.sel> [material]")
	if err != nil {
		return err
	}
	prog, res, err := compileResolved(file, material, nil)
	if err != nil {
		return err
	}
	report := newGraphReport(prog, res)
	switch *format {
	case "json":
		return writeJSON(os.Stdout, report)
	case "dot":
		fmt.Print(graphDOT(report))
		return nil
	default:
		return fmt.Errorf("unsupported graph format %q", *format)
	}
}

func newGraphReport(prog resolvedProgram, res selena.Result) graphReport {
	report := graphReport{
		GraphVersion: 1,
		SourcePath:   prog.File,
		Material:     res.Material.Name,
		Layout:       res.Layout,
	}
	report.Counts = graphCounts{
		Functions:  len(res.Program.Funcs),
		Materials:  len(res.Program.Materials),
		Uniforms:   len(res.Module.Uniforms),
		Attributes: len(res.Module.Attributes),
		Varyings:   len(res.Module.Varyings),
		Textures:   len(res.Module.Textures),
		Artifacts:  len(res.Artifacts),
	}
	for _, fn := range res.Program.Funcs {
		report.HIR.Functions = append(report.HIR.Functions, fn.Name)
	}
	for _, mat := range res.Program.Materials {
		report.HIR.Materials = append(report.HIR.Materials, mat.Name)
	}
	report.HIR.Selected = res.Material.Name
	report.HIR.Extends = res.Material.Extends
	for _, param := range res.Material.Params {
		report.HIR.Params = append(report.HIR.Params, param.Name+":"+string(param.Type))
	}
	report.IR = snapshotIR(res.Module)
	for _, artifact := range res.Artifacts {
		report.Artifacts = append(report.Artifacts, artifactSummary(artifact))
	}
	return report
}

func snapshotIR(mod ir.Module) irSnapshot {
	out := irSnapshot{
		Name:     mod.Name,
		Vertex:   stageInfo{Statements: len(mod.Vertex.Body), Output: fmt.Sprintf("%T", mod.Vertex.Output)},
		Fragment: stageInfo{Statements: len(mod.Fragment.Body), Output: fmt.Sprintf("%T", mod.Fragment.Output)},
	}
	out.Uniforms = snapshotBindings(mod.Uniforms)
	out.Attributes = snapshotBindings(mod.Attributes)
	out.Varyings = snapshotBindings(mod.Varyings)
	for _, texture := range mod.Textures {
		out.Textures = append(out.Textures, texture.Name)
	}
	return out
}

func snapshotBindings(in []ir.Binding) []binding {
	out := make([]binding, 0, len(in))
	for _, b := range in {
		out = append(out, binding{Name: b.Name, Type: string(b.Type)})
	}
	return out
}

func artifactSummary(a selena.Artifact) artifactInfo {
	return artifactInfo{
		Target:        a.Target,
		SourceBytes:   len(a.Source),
		VertexBytes:   len(a.Vertex),
		FragmentBytes: len(a.Fragment),
	}
}

func graphDOT(report graphReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "digraph %s {\n", sanitizeIdent(report.Material))
	b.WriteString("  material [label=\"material\", shape=box];\n")
	for _, name := range []string{"vertex", "fragment"} {
		fmt.Fprintf(&b, "  %s [label=%q, shape=component];\n", name, name)
		fmt.Fprintf(&b, "  material -> %s;\n", name)
	}
	for _, artifact := range report.Artifacts {
		target := sanitizeIdent(string(artifact.Target))
		fmt.Fprintf(&b, "  target_%s [label=%q, shape=note];\n", target, string(artifact.Target))
		fmt.Fprintf(&b, "  material -> target_%s;\n", target)
	}
	b.WriteString("}\n")
	return b.String()
}

type shaderManifest struct {
	ManifestVersion int           `json:"manifest_version"`
	CreatedAt       string        `json:"created_at,omitempty"`
	SourcePath      string        `json:"source_path,omitempty"`
	Material        string        `json:"material"`
	Targets         []string      `json:"targets"`
	SourceCount     int           `json:"source_count"`
	Sources         []shaderEntry `json:"sources"`
}

type shaderEntry struct {
	Target      selena.Target     `json:"target"`
	Stage       string            `json:"stage"`
	SourceFile  string            `json:"source_file"`
	SourceBytes int               `json:"source_bytes"`
	Validation  *shaderValidation `json:"validation,omitempty"`
}

type shaderValidation struct {
	Entries       []string `json:"entries,omitempty"`
	EntryChecked  bool     `json:"entry_checked,omitempty"`
	ToolSkipped   bool     `json:"tool_skipped,omitempty"`
	ToolError     string   `json:"tool_error,omitempty"`
	ToolOutputLen int      `json:"tool_output_len,omitempty"`
}

func shaders(args []string) error {
	fs := flag.NewFlagSet("shaders", flag.ContinueOnError)
	targetFilter := fs.String("target", "", "target to extract; empty extracts all")
	outDir := fs.String("out", "shaders", "directory for extracted shader sources")
	validateSources := fs.Bool("validate", false, "record shader validation status")
	if err := fs.Parse(args); err != nil {
		return err
	}
	file, material, err := parseInputMaterial(fs.Args(), "selena shaders [--target wgsl|glsl|metal|gles] [--out dir] [--validate] <file.sel> [material]")
	if err != nil {
		return err
	}
	targets, err := selectedTargets(*targetFilter)
	if err != nil {
		return err
	}
	prog, res, err := compileResolved(file, material, targets)
	if err != nil {
		return err
	}
	manifest, err := writeShaderSources(res, *outDir, "", *validateSources)
	if err != nil {
		return err
	}
	manifest.SourcePath = prog.File
	if err := writeJSONFile(filepath.Join(*outDir, "manifest.json"), manifest); err != nil {
		return err
	}
	fmt.Printf("wrote %d shader source(s) -> %s\n", manifest.SourceCount, *outDir)
	return nil
}

func compileBundle(args []string) error {
	fs := flag.NewFlagSet("compile", flag.ContinueOnError)
	bundleDir := fs.String("bundle", "", "write inspection bundle sidecar directory")
	validateSources := fs.Bool("validate-shaders", false, "record shader validation status in the bundle manifest")
	if err := fs.Parse(args); err != nil {
		return err
	}
	file, material, err := parseInputMaterial(fs.Args(), "selena compile --bundle dir [--validate-shaders] <file.sel> [material]")
	if err != nil {
		return err
	}
	if *bundleDir == "" {
		return fmt.Errorf("usage: selena compile --bundle dir [--validate-shaders] <file.sel> [material]")
	}
	prog, res, err := compileResolved(file, material, nil)
	if err != nil {
		return err
	}
	if err := writeCompileBundle(*bundleDir, prog, res, *validateSources); err != nil {
		return err
	}
	fmt.Printf("bundle: %s\n", *bundleDir)
	return nil
}

func writeCompileBundle(dir string, prog resolvedProgram, res selena.Result, validateSources bool) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if len(prog.Source) > 0 {
		if err := os.WriteFile(filepath.Join(dir, "source.sel"), prog.Source, 0o644); err != nil {
			return err
		}
	}
	if err := writeJSONFile(filepath.Join(dir, "graph.json"), newGraphReport(prog, res)); err != nil {
		return err
	}
	shaderManifest, err := writeShaderSources(res, filepath.Join(dir, "shaders"), "shaders", validateSources)
	if err != nil {
		return err
	}
	manifest := struct {
		BundleVersion int           `json:"bundle_version"`
		CreatedAt     string        `json:"created_at"`
		SourcePath    string        `json:"source_path,omitempty"`
		Material      string        `json:"material"`
		Targets       []string      `json:"targets"`
		SourceCount   int           `json:"source_count"`
		Sources       []shaderEntry `json:"sources"`
	}{
		BundleVersion: 1,
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
		SourcePath:    prog.File,
		Material:      res.Material.Name,
		Targets:       shaderManifest.Targets,
		SourceCount:   shaderManifest.SourceCount,
		Sources:       shaderManifest.Sources,
	}
	return writeJSONFile(filepath.Join(dir, "manifest.json"), manifest)
}

func writeShaderSources(res selena.Result, outDir, manifestPrefix string, validateSources bool) (shaderManifest, error) {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return shaderManifest{}, err
	}
	manifest := shaderManifest{
		ManifestVersion: 1,
		CreatedAt:       time.Now().UTC().Format(time.RFC3339),
		Material:        res.Material.Name,
	}
	for _, artifact := range res.Artifacts {
		for _, source := range artifactStageSources(artifact) {
			filename := shaderFilename(artifact.Target, source.Stage)
			if err := os.WriteFile(filepath.Join(outDir, filename), []byte(source.Source), 0o644); err != nil {
				return shaderManifest{}, err
			}
			sourceFile := filename
			if manifestPrefix != "" {
				sourceFile = filepath.ToSlash(filepath.Join(manifestPrefix, filename))
			}
			entry := shaderEntry{
				Target:      artifact.Target,
				Stage:       source.Stage,
				SourceFile:  sourceFile,
				SourceBytes: len(source.Source),
			}
			if validateSources {
				entry.Validation = validateShaderSource(artifact.Target, source.Stage, source.Source)
			}
			manifest.Sources = append(manifest.Sources, entry)
			manifest.SourceCount++
		}
		manifest.Targets = append(manifest.Targets, string(artifact.Target))
	}
	sort.Strings(manifest.Targets)
	return manifest, nil
}

type stageSource struct {
	Stage  string
	Source string
}

func artifactStageSources(a selena.Artifact) []stageSource {
	if a.Source != "" {
		return []stageSource{{Stage: "module", Source: a.Source}}
	}
	return []stageSource{
		{Stage: "vertex", Source: a.Vertex},
		{Stage: "fragment", Source: a.Fragment},
	}
}

func validateShaderSource(target selena.Target, stage, source string) *shaderValidation {
	validation := &shaderValidation{Entries: expectedEntries(target, stage)}
	validation.EntryChecked = entriesPresent(source, validation.Entries)
	tool, ext, ok := validatorForShader(target, stage)
	if !ok {
		validation.ToolSkipped = true
		return validation
	}
	res, err := prismvalidate.Run(tool, source, ext, nil)
	validation.ToolSkipped = res.Skipped
	validation.ToolOutputLen = len(res.Output)
	if err != nil {
		validation.ToolError = err.Error()
	}
	return validation
}

func expectedEntries(target selena.Target, stage string) []string {
	switch target {
	case selena.TargetWGSL, selena.TargetMetal:
		return []string{"vertexMain", "fragmentMain"}
	case selena.TargetGLSL, selena.TargetGLES:
		return []string{"main"}
	default:
		return nil
	}
}

func entriesPresent(source string, entries []string) bool {
	for _, entry := range entries {
		if !strings.Contains(source, entry) {
			return false
		}
	}
	return len(entries) > 0
}

func validatorForShader(target selena.Target, stage string) (string, string, bool) {
	switch target {
	case selena.TargetWGSL:
		return "naga", ".wgsl", true
	case selena.TargetGLSL, selena.TargetGLES:
		if stage == "vertex" {
			return "glslangValidator", ".vert", true
		}
		return "glslangValidator", ".frag", true
	default:
		return "", "", false
	}
}

func shaderFilename(target selena.Target, stage string) string {
	switch target {
	case selena.TargetWGSL:
		return "wgsl.wgsl"
	case selena.TargetMetal:
		return "metal.metal"
	case selena.TargetGLSL:
		return "glsl." + stage + ".glsl"
	case selena.TargetGLES:
		return "gles." + stage + ".glsl"
	default:
		return string(target) + "." + stage + ".txt"
	}
}

func selectedTargets(filter string) ([]selena.Target, error) {
	if filter == "" {
		return nil, nil
	}
	target, ok := emitTargets[filter]
	if !ok {
		return nil, fmt.Errorf("unknown target %q (want one of: wgsl, glsl, metal, gles)", filter)
	}
	return []selena.Target{target}, nil
}

func parseInputMaterial(args []string, usage string) (string, string, error) {
	if len(args) < 1 || len(args) > 2 {
		return "", "", fmt.Errorf("usage: %s", usage)
	}
	material := ""
	if len(args) == 2 {
		material = args[1]
	}
	return args[0], material, nil
}

func compileResolved(file, material string, targets []selena.Target) (resolvedProgram, selena.Result, error) {
	prog, err := resolveProgram(file)
	if err != nil {
		return resolvedProgram{}, selena.Result{}, err
	}
	res, err := selena.CompileProgram(prog.Program, selena.CompileOptions{
		Material: material,
		Targets:  targets,
	})
	if err != nil {
		return resolvedProgram{}, selena.Result{}, attachResolvedSource(prog, err)
	}
	return prog, res, nil
}

func doctor(args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: selena doctor")
	}
	fmt.Println("selena: dev")
	fmt.Printf("go: %s %s/%s\n", goruntime.Version(), goruntime.GOOS, goruntime.GOARCH)
	fmt.Println("targets: wgsl glsl metal gles")
	fmt.Println("tools:")
	for _, tool := range []string{"naga", "glslangValidator", "xcrun"} {
		if path, err := exec.LookPath(tool); err == nil {
			fmt.Printf("  %s: %s\n", tool, path)
		} else {
			fmt.Printf("  %s: unavailable\n", tool)
		}
	}
	return nil
}

func writeJSONFile(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return writeJSON(file, value)
}

func writeJSON(w *os.File, value any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}

func sanitizeIdent(value string) string {
	if value == "" {
		return "unnamed"
	}
	var b strings.Builder
	for i, r := range value {
		if r == '_' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (i > 0 && r >= '0' && r <= '9') {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('_')
	}
	return b.String()
}
