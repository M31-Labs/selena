package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"strings"

	"m31labs.dev/selena/bindings"
)

// Session holds the open .sel documents of one client.
type Session struct {
	opts Options
	docs map[string]*analysis
}

// NewSession returns an empty session.
func NewSession(opts Options) *Session {
	return &Session{opts: opts, docs: map[string]*analysis{}}
}

// IsSelena reports whether a document URI names a Selena source.
func IsSelena(uri string) bool { return strings.HasSuffix(strings.ToLower(uri), ".sel") }

// Update stores a document's text, analyzes it and returns its diagnostics.
// The result is never nil, so it publishes an empty set when the file is clean.
func (s *Session) Update(uri, text string) []Diagnostic {
	a := analyze([]byte(text), s.opts)
	if prev := s.docs[uri]; prev != nil && !a.parsed && prev.parsed && len(a.program.Materials) < len(prev.program.Materials) {
		// Keep the last parsed program for hover and completion while the
		// user types through a syntax error; diagnostics are the new ones.
		keep := *prev
		keep.src, keep.diags = a.src, a.diags
		a = &keep
	}
	s.docs[uri] = a
	if a.diags == nil {
		return []Diagnostic{}
	}
	return a.diags
}

// Close forgets a document.
func (s *Session) Close(uri string) { delete(s.docs, uri) }

// Hover answers textDocument/hover, or returns nil.
func (s *Session) Hover(uri string, pos Position) *Hover {
	if a := s.docs[uri]; a != nil {
		return a.hover(pos)
	}
	return nil
}

// Completion answers textDocument/completion.
func (s *Session) Completion(uri string, pos Position) []CompletionItem {
	if a := s.docs[uri]; a != nil {
		if items := a.completion(pos); items != nil {
			return items
		}
	}
	return []CompletionItem{}
}

// Definition answers textDocument/definition.
func (s *Session) Definition(uri string, pos Position) []Location {
	if a := s.docs[uri]; a != nil {
		if locs := a.definition(uri, pos); locs != nil {
			return locs
		}
	}
	return []Location{}
}

// Capabilities is the server capability object this package implements.
func Capabilities() map[string]any {
	return map[string]any{
		"textDocumentSync":   map[string]any{"openClose": true, "change": 1},
		"hoverProvider":      true,
		"definitionProvider": true,
		"completionProvider": map[string]any{"triggerCharacters": []string{".", ":"}},
	}
}

func pathURI(path string) string {
	return (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String()
}

type request struct {
	ID     json.RawMessage `json:"id,omitempty"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

type docParams struct {
	TextDocument struct {
		URI  string `json:"uri"`
		Text string `json:"text"`
	} `json:"textDocument"`
	ContentChanges []struct {
		Text string `json:"text"`
	} `json:"contentChanges"`
	Position Position `json:"position"`
}

// Serve runs the language server over a JSON-RPC stream until the client
// sends exit or closes the input.
func Serve(in io.Reader, out io.Writer) error {
	s := NewSession(Options{})
	r := bufio.NewReader(in)
	for {
		payload, err := ReadMessage(r)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		var req request
		if json.Unmarshal(payload, &req) != nil {
			continue
		}
		var p docParams
		_ = json.Unmarshal(req.Params, &p)
		uri := p.TextDocument.URI
		switch req.Method {
		case "initialize":
			err = WriteMessage(out, response(req.ID, map[string]any{
				"capabilities": Capabilities(),
				"serverInfo":   map[string]any{"name": "selena-lsp", "version": bindings.LanguageVersion},
			}))
		case "shutdown":
			err = WriteMessage(out, response(req.ID, nil))
		case "exit":
			return nil
		case "textDocument/didOpen":
			err = publish(out, uri, s.Update(uri, p.TextDocument.Text))
		case "textDocument/didChange":
			if n := len(p.ContentChanges); n > 0 {
				err = publish(out, uri, s.Update(uri, p.ContentChanges[n-1].Text))
			}
		case "textDocument/didClose":
			s.Close(uri)
			err = publish(out, uri, []Diagnostic{})
		case "textDocument/hover":
			err = WriteMessage(out, response(req.ID, s.Hover(uri, p.Position)))
		case "textDocument/completion":
			err = WriteMessage(out, response(req.ID, s.Completion(uri, p.Position)))
		case "textDocument/definition":
			err = WriteMessage(out, response(req.ID, s.Definition(uri, p.Position)))
		default:
			if len(req.ID) > 0 {
				err = WriteMessage(out, response(req.ID, nil))
			}
		}
		if err != nil {
			return err
		}
	}
}

func response(id json.RawMessage, result any) map[string]any {
	return map[string]any{"jsonrpc": "2.0", "id": id, "result": result}
}

func publish(out io.Writer, uri string, diags []Diagnostic) error {
	return WriteMessage(out, map[string]any{"jsonrpc": "2.0", "method": "textDocument/publishDiagnostics",
		"params": map[string]any{"uri": uri, "diagnostics": diags}})
}

// ReadMessage reads one Content-Length framed message.
func ReadMessage(r *bufio.Reader) ([]byte, error) {
	length := -1
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		if name, value, ok := strings.Cut(line, ":"); ok && strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
			if _, err := fmt.Sscanf(strings.TrimSpace(value), "%d", &length); err != nil {
				return nil, err
			}
		}
	}
	if length < 0 {
		return nil, io.ErrUnexpectedEOF
	}
	payload := make([]byte, length)
	_, err := io.ReadFull(r, payload)
	return payload, err
}

// WriteMessage writes one Content-Length framed JSON message.
func WriteMessage(w io.Writer, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Content-Length: %d\r\n\r\n", len(data)); err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}
