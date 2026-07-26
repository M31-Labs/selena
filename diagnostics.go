package selena

import (
	"errors"
	"fmt"
	"strings"

	"m31labs.dev/selena/hir"
	"m31labs.dev/selena/lower"
	"m31labs.dev/selena/parse"
)

// Severity classifies a compiler diagnostic.
type Severity string

const (
	SeverityError Severity = "error"
)

// Diagnostic is a structured compiler message. Range is zero when source
// information is not available, such as when callers hand-build HIR.
type Diagnostic struct {
	Code     string
	Severity Severity
	Message  string
	Range    hir.Span
	Hint     string
}

// CompileError wraps one or more diagnostics while preserving the original
// error for errors.Is/errors.As callers.
type CompileError struct {
	Diagnostics []Diagnostic
	Err         error
}

func (e *CompileError) Error() string {
	if len(e.Diagnostics) == 0 {
		return e.Err.Error()
	}
	d := e.Diagnostics[0]
	if !d.Range.IsZero() {
		return fmt.Sprintf("%s at %d:%d: %s", d.Code, d.Range.Start.Line, d.Range.Start.Column, d.Message)
	}
	return fmt.Sprintf("%s: %s", d.Code, d.Message)
}

func (e *CompileError) Unwrap() error { return e.Err }

func compileError(err error) error {
	if err == nil {
		return nil
	}
	var pe *parse.Error
	if errors.As(err, &pe) {
		return &CompileError{
			Err: err,
			Diagnostics: []Diagnostic{{
				Code:     "SEL0001",
				Severity: SeverityError,
				Message:  pe.Message,
				Range:    pe.Span,
				Hint:     diagnosticHint("SEL0001", pe.Message),
			}},
		}
	}
	var le *lower.DiagnosticError
	if errors.As(err, &le) {
		return &CompileError{
			Err: err,
			Diagnostics: []Diagnostic{{
				Code:     le.Code,
				Severity: SeverityError,
				Message:  le.Message,
				Range:    le.Span,
				Hint:     diagnosticHint(le.Code, le.Message),
			}},
		}
	}
	return err
}

func diagnosticHint(code, message string) string {
	// A few messages carry a more specific remedy than their code's general
	// hint. Keep these ahead of the code switch.
	switch {
	case strings.Contains(message, "unknown function"):
		return "Check the spelling against the Selena builtin list, or declare a top-level `fn` with this name."
	case strings.Contains(message, "a fixed-size array param is spelled"):
		return "Write a fixed-size array param as `param name : array<T, N>`; the C-style `T[N]` form is not accepted."
	case strings.Contains(message, "array index must be int or uint"):
		return "Selena integer literals carry an `i` suffix: write `for (var i = 0i; i < 8i; i = i + 1i)` so the counter indexes the array directly."
	}
	switch code {
	case "SEL0001":
		return "Check the surrounding braces, parentheses, arrows, and return expression."
	case "SEL1001":
		return "Rename one parameter or remove the duplicate declaration."
	case "SEL1002":
		return "Rename the local binding or choose a name that does not collide with generated interface names."
	case "SEL1003":
		return "Use one of the currently supported parameter types: float, vec2, vec3, vec4, mat3, mat4, color, Sun, texture2d, or array<T, N>."
	case "SEL1004":
		return "Choose a name that is not a shader keyword, generated binding, or Selena stdlib builtin."
	case "SEL1005":
		return "This feature is not available here; author vertex()/varying/statefield only in mesh materials, and keep vertex() bodies to let bindings and varying writes."
	case "SEL2001":
		return "Declare a material param or let binding with this name, or correct the identifier."
	case "SEL2002":
		return "Check the record field name; available geometry fields are currently position, normal, uv, and worldNormal."
	case "SEL2003":
		return "Check the function name, argument count, and whether super.surface is only used in an extends material."
	case "SEL2004":
		return "Use only components that exist on the vector, and do not mix xyzw with rgba in one swizzle."
	case "SEL2005":
		return "Make the operand and argument types match, or introduce an explicit scalar/vector conversion."
	case "SEL3001":
		return "Use a constant scalar or vector default that matches the parameter type."
	default:
		return ""
	}
}
