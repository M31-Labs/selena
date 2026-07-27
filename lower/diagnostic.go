package lower

import (
	"fmt"

	"m31labs.dev/selena/hir"
)

const (
	CodeDuplicateParam     = "SEL1001"
	CodeDuplicateLocal     = "SEL1002"
	CodeUnsupportedType    = "SEL1003"
	CodeReservedName       = "SEL1004"
	CodeUnsupportedFeat    = "SEL1005"
	CodeInterfaceCollision = "SEL1006"
	CodeUnknownName        = "SEL2001"
	CodeInvalidMember      = "SEL2002"
	CodeInvalidCall        = "SEL2003"
	CodeInvalidSwizzle     = "SEL2004"
	CodeTypeMismatch       = "SEL2005"
	CodeInvalidDefault     = "SEL3001"
)

// DiagnosticError is a lowerer error that can be anchored back to source.
type DiagnosticError struct {
	Code    string
	Span    hir.Span
	Message string
}

func (e *DiagnosticError) Error() string { return e.Message }

func diagnostic(code string, span hir.Span, format string, args ...any) error {
	return &DiagnosticError{Code: code, Span: span, Message: fmt.Sprintf(format, args...)}
}

// unknownFunctionHints maps a mistyped or unsupported call name to a short
// suggestion that names the real stdlib function.
var unknownFunctionHints = map[string]string{
	"color": "use rgb(r, g, b) or rgb(r, g, b, a) instead",
}

// unknownFunctionError reports a call naming a function that is neither a
// registered stdlib builtin (lower/stdlib.go) nor a user-defined function.
// Legitimate user-defined calls never reach this: inlineFuncs rewrites them
// into their bodies before typing and resolving run (lower.go:38,
// extends.go:83), so any name still unresolved here is a genuine unknown call.
//
// The code is CodeUnknownName (SEL2001), matching every other "the language
// does not know this name" diagnostic (lower/typer.go unknown name, unknown
// array); TestUnknownCallIsUnknownName (lower/post_surface_test.go) pins it.
func unknownFunctionError(name string, span hir.Span) error {
	if hint, ok := unknownFunctionHints[name]; ok {
		return diagnostic(CodeUnknownName, span, "unknown function %q: %s", name, hint)
	}
	return diagnostic(CodeUnknownName, span, "unknown function %q", name)
}
