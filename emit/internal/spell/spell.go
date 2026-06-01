package spell

import "strings"

// Builtins maps Selena builtin names to a backend function spelling.
type Builtins map[string]string

// Call renders a backend builtin call. Unknown names intentionally fall back to
// the Selena name so experimental builtins can still flow through diagnostics.
func Call(name string, args []string, builtins Builtins) string {
	spelled := name
	if v, ok := builtins[name]; ok {
		spelled = v
	}
	return spelled + "(" + strings.Join(args, ", ") + ")"
}
