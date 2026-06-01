package main

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"m31labs.dev/selena"
	"m31labs.dev/selena/parse"
)

type sourceError struct {
	file   string
	source []byte
	err    error
}

func (e *sourceError) Error() string {
	diags := diagnosticsFromError(e.err)
	if len(diags) == 0 {
		return e.err.Error()
	}
	d := diags[0]
	if !d.Range.IsZero() {
		return fmt.Sprintf("%s at %d:%d: %s", d.Code, d.Range.Start.Line, d.Range.Start.Column, d.Message)
	}
	return fmt.Sprintf("%s: %s", d.Code, d.Message)
}
func (e *sourceError) Unwrap() error { return e.err }

func attachResolvedSource(prog resolvedProgram, err error) error {
	if err == nil || prog.Source == nil {
		return err
	}
	return attachSource(prog.File, prog.Source, err)
}

func attachSource(file string, source []byte, err error) error {
	if err == nil {
		return nil
	}
	if len(diagnosticsFromError(err)) == 0 {
		return err
	}
	return &sourceError{file: file, source: source, err: err}
}

func printCommandError(w io.Writer, err error) {
	fmt.Fprintln(w, "selena:", err)
	var se *sourceError
	if !errors.As(err, &se) {
		return
	}
	renderDiagnostics(w, se.file, se.source, se.err)
}

func renderDiagnostics(w io.Writer, file string, source []byte, err error) {
	for _, d := range diagnosticsFromError(err) {
		renderDiagnostic(w, file, source, d)
	}
}

func diagnosticsFromError(err error) []selena.Diagnostic {
	var ce *selena.CompileError
	if errors.As(err, &ce) {
		return ce.Diagnostics
	}
	var pe *parse.Error
	if errors.As(err, &pe) {
		return []selena.Diagnostic{{
			Code:     "SEL0001",
			Severity: selena.SeverityError,
			Message:  pe.Message,
			Range:    pe.Span,
			Hint:     "Check the surrounding braces, parentheses, arrows, and return expression.",
		}}
	}
	return nil
}

func renderDiagnostic(w io.Writer, file string, source []byte, d selena.Diagnostic) {
	if d.Range.IsZero() || len(source) == 0 {
		if d.Hint != "" {
			fmt.Fprintf(w, "hint: %s\n", d.Hint)
		}
		return
	}
	line, ok := sourceLine(source, d.Range.Start.Line)
	if !ok {
		return
	}

	lineNo := d.Range.Start.Line
	col := max(d.Range.Start.Column, 1)
	width := highlightWidth(d, line)
	prefixWidth := len(strconv.Itoa(lineNo))

	fmt.Fprintf(w, "  --> %s:%d:%d\n", file, lineNo, col)
	fmt.Fprintf(w, "%*s |\n", prefixWidth, "")
	fmt.Fprintf(w, "%*d | %s\n", prefixWidth, lineNo, line)
	fmt.Fprintf(w, "%*s | %s%s %s\n", prefixWidth, "", strings.Repeat(" ", col-1), strings.Repeat("^", width), d.Message)
	if d.Hint != "" {
		fmt.Fprintf(w, "hint: %s\n", d.Hint)
	}
}

func sourceLine(source []byte, line int) (string, bool) {
	if line < 1 {
		return "", false
	}
	lines := strings.Split(string(source), "\n")
	if line > len(lines) {
		return "", false
	}
	return lines[line-1], true
}

func highlightWidth(d selena.Diagnostic, line string) int {
	if d.Range.End.Line == d.Range.Start.Line && d.Range.End.Column > d.Range.Start.Column {
		return max(d.Range.End.Column-d.Range.Start.Column, 1)
	}
	if d.Range.Start.Column > 0 && d.Range.Start.Column <= utf8.RuneCountInString(line) {
		return 1
	}
	return 1
}
