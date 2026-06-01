package lower

import (
	"fmt"

	"m31labs.dev/selena/hir"
)

const (
	CodeDuplicateParam  = "SEL1001"
	CodeDuplicateLocal  = "SEL1002"
	CodeUnsupportedType = "SEL1003"
	CodeUnknownName     = "SEL2001"
	CodeInvalidMember   = "SEL2002"
	CodeInvalidCall     = "SEL2003"
	CodeInvalidSwizzle  = "SEL2004"
	CodeTypeMismatch    = "SEL2005"
	CodeInvalidDefault  = "SEL3001"
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
