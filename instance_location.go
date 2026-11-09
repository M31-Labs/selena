package selena

import (
	"fmt"

	"m31labs.dev/selena/bindings"
	"m31labs.dev/selena/ir"
)

// maxVertexAttributes is WebGPU's default maxVertexAttributes: every location
// must be below it.
const maxVertexAttributes = 16

// placeInstanceAttributes moves the instance attributes of mod and layout to
// start at first. Zero keeps them directly after the vertex attributes and
// changes nothing. A material with no instance attributes ignores first.
func placeInstanceAttributes(mod *ir.Module, layout *bindings.Layout, first int) error {
	n := len(mod.InstanceAttributes)
	if first == 0 || n == 0 {
		return nil
	}
	if first < len(mod.Attributes) {
		return fmt.Errorf("first instance location %d is below the %d vertex attributes; use at least %d", first, len(mod.Attributes), len(mod.Attributes))
	}
	if last := first + n - 1; last >= maxVertexAttributes {
		return fmt.Errorf("first instance location %d puts the last of %d instance attributes at location %d; locations must stay below %d", first, n, last, maxVertexAttributes)
	}
	mod.InstanceBase = first
	for i := range layout.InstanceAttributes {
		layout.InstanceAttributes[i].Location = first + i
	}
	return nil
}
