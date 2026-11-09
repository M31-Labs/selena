// Package lsp is the Selena language server. It publishes compiler
// diagnostics for .sel files and answers hover, completion and
// go-to-definition requests over the Language Server Protocol.
//
// Run it with `selena lsp`, or embed a Session in another server: `gosx lsp`
// routes .sel documents to one. A Session is not safe for concurrent use.
//
// Hover shows the type of params, context fields, locals and geometry
// fields, the registry entry of engine inputs (bindings.EngineInputs), and the
// signature and documentation of builtins, material library helpers and the
// file's own functions. Completion offers the names that are valid at the
// cursor. Definition jumps to declarations in the file and to material
// library helpers, whose sources it writes to the user cache directory.
//
// Helpers from the material library resolve without configuration: a file
// that calls a helper links the module that defines it, the way `gosx build`
// links the modules named in gosx.config.json.
package lsp

// Position is a zero-based line and UTF-16 character offset, as LSP defines it.
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// Range is a half-open span of positions.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Location is a range in a document.
type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

// LSP diagnostic severities.
const (
	SeverityError   = 1
	SeverityWarning = 2
)

// Diagnostic is one compiler message for a document.
type Diagnostic struct {
	Range    Range  `json:"range"`
	Severity int    `json:"severity"`
	Code     string `json:"code,omitempty"`
	Source   string `json:"source"`
	Message  string `json:"message"`
}

// MarkupContent is Markdown text for hover and completion documentation.
type MarkupContent struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

// Hover is the answer to textDocument/hover.
type Hover struct {
	Contents MarkupContent `json:"contents"`
	Range    *Range        `json:"range,omitempty"`
}

// LSP completion item kinds used by the server.
const (
	CompletionFunction      = 3
	CompletionField         = 5
	CompletionVariable      = 6
	CompletionProperty      = 10
	CompletionKeyword       = 14
	CompletionStruct        = 22
	CompletionTypeParameter = 25
)

// CompletionItem is one completion candidate.
type CompletionItem struct {
	Label         string         `json:"label"`
	Kind          int            `json:"kind"`
	Detail        string         `json:"detail,omitempty"`
	Documentation *MarkupContent `json:"documentation,omitempty"`
	InsertText    string         `json:"insertText,omitempty"`
}

func markdown(text string) MarkupContent { return MarkupContent{Kind: "markdown", Value: text} }
