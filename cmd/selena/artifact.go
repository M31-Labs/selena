package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"m31labs.dev/selena/artifact"
	"m31labs.dev/selena/artifact/builder"
	"m31labs.dev/selena/materiallib"
	"m31labs.dev/selena/result"
)

const artifactUsage = "selena artifact [-o out.sel.json] [-targets wgsl,gles] [-materials A,B] [-modules brdf,lighting] [-specular-mrt] [-first-instance-location N] [-path label] [-check] <file.sel>"

// artifactCommand writes or checks a deterministic artifact file for one .sel
// source. Without -o it writes <file.sel>.json next to the source; "-o -"
// writes to stdout. With -check it writes nothing and fails when the existing
// artifact no longer matches the source or the linked compiler.
func artifactCommand(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("artifact", flag.ContinueOnError)
	out := fs.String("o", "", "output path (default <file.sel>.json; - for stdout)")
	targetList := fs.String("targets", "", "comma-separated targets (default all: wgsl,glsl,metal,gles)")
	materialList := fs.String("materials", "", "comma-separated material names (default every material)")
	moduleList := fs.String("modules", "", "comma-separated materiallib modules to link")
	specular := fs.Bool("specular-mrt", false, "add the color/specular fragment variant to mesh materials")
	firstInstance := fs.Int("first-instance-location", 0, "location of the first instance attribute (default: after the vertex attributes)")
	pathLabel := fs.String("path", "", "source path recorded in the file (default the file's base name)")
	checkOnly := fs.Bool("check", false, "check the existing artifact instead of writing it")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: %s", artifactUsage)
	}
	file := fs.Arg(0)
	source, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	dest := *out
	if dest == "" {
		dest = artifact.PathFor(file)
	}
	if *checkOnly {
		if dest == "-" {
			return fmt.Errorf("selena artifact -check needs a file, not stdout")
		}
		data, err := os.ReadFile(dest)
		if err != nil {
			return err
		}
		shipped, err := artifact.Decode(data)
		if err != nil {
			return err
		}
		if err := builder.Check(shipped, source); err != nil {
			return attachSource(file, source, err)
		}
		fmt.Fprintf(stdout, "artifact: %s matches %s\n", dest, file)
		return nil
	}
	targets, err := artifactTargets(*targetList)
	if err != nil {
		return err
	}
	label := *pathLabel
	if label == "" {
		label = filepath.ToSlash(filepath.Base(file))
	}
	built, err := builder.Build(source, builder.Options{
		Path:        label,
		Materials:   splitList(*materialList),
		Targets:     targets,
		SpecularMRT: *specular,
		Modules:     artifactModules(*moduleList),

		FirstInstanceLocation: *firstInstance,
	})
	if err != nil {
		return attachSource(file, source, err)
	}
	data, err := artifact.Encode(built)
	if err != nil {
		return err
	}
	if dest == "-" {
		_, err := stdout.Write(data)
		return err
	}
	if existing, err := os.ReadFile(dest); err == nil && bytes.Equal(existing, data) {
		fmt.Fprintf(stdout, "artifact: %s (unchanged)\n", dest)
		return nil
	}
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "artifact: %s (%d materials, %d bytes)\n", dest, len(built.Materials), len(data))
	return nil
}

func artifactTargets(list string) ([]result.Target, error) {
	names := splitList(list)
	if names == nil {
		return nil, nil
	}
	out := make([]result.Target, 0, len(names))
	for _, name := range names {
		target, ok := emitTargets[name]
		if !ok {
			return nil, fmt.Errorf("unknown target %q (want one of: wgsl, glsl, metal, gles)", name)
		}
		out = append(out, target)
	}
	return out, nil
}

func artifactModules(list string) []materiallib.Module {
	names := splitList(list)
	if names == nil {
		return nil
	}
	out := make([]materiallib.Module, len(names))
	for i, name := range names {
		out[i] = materiallib.Module(name)
	}
	return out
}

// splitList splits a comma-separated flag value, dropping blanks. An empty
// value returns nil so callers keep their defaults.
func splitList(list string) []string {
	var out []string
	for _, item := range strings.Split(list, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}
