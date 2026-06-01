package selena

import (
	"errors"
	"fmt"

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
				Message:  err.Error(),
				Range:    le.Span,
			}},
		}
	}
	return err
}
