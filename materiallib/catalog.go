package materiallib

import (
	"fmt"
	"slices"
	"strings"
)

// Modules returns every bundled module, sorted by name. Tooling such as the
// language server uses it to offer and resolve helpers it was not told about.
func Modules() []Module {
	entries, err := sources.ReadDir("modules")
	if err != nil {
		return nil
	}
	out := make([]Module, 0, len(entries))
	for _, entry := range entries {
		name, ok := strings.CutSuffix(entry.Name(), ".sel")
		if ok && !entry.IsDir() {
			out = append(out, Module(name))
		}
	}
	slices.Sort(out)
	return out
}

// Source returns a module's Selena source, the text Functions parses.
func Source(module Module) ([]byte, error) {
	if strings.ContainsAny(string(module), "/\\.") {
		return nil, fmt.Errorf("unknown material library module %q", module)
	}
	data, err := sources.ReadFile("modules/" + string(module) + ".sel")
	if err != nil {
		return nil, fmt.Errorf("unknown material library module %q", module)
	}
	return data, nil
}
