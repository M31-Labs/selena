package lsp

import (
	"unicode/utf16"
	"unicode/utf8"

	"m31labs.dev/selena/hir"
)

// offsetAt converts an LSP position to a byte offset in src, clamped to the
// line it names and to the document.
func offsetAt(src []byte, pos Position) int {
	line, i := 0, 0
	for i < len(src) && line < pos.Line {
		if src[i] == '\n' {
			line++
		}
		i++
	}
	if line < pos.Line {
		return len(src)
	}
	units := 0
	for i < len(src) && src[i] != '\n' && units < pos.Character {
		r, size := utf8.DecodeRune(src[i:])
		units += utf16.RuneLen(r)
		i += size
	}
	return i
}

// positionAt converts a byte offset in src to an LSP position.
func positionAt(src []byte, offset int) Position {
	if offset > len(src) {
		offset = len(src)
	}
	var pos Position
	for i := 0; i < offset; {
		r, size := utf8.DecodeRune(src[i:])
		if r == '\n' {
			pos.Line++
			pos.Character = 0
		} else {
			pos.Character += utf16.RuneLen(r)
		}
		i += size
	}
	return pos
}

func rangeOf(src []byte, start, end int) Range {
	return Range{Start: positionAt(src, start), End: positionAt(src, end)}
}

// spanRange converts a parser span. A zero span maps to the first line.
func spanRange(src []byte, span hir.Span) Range {
	if span.IsZero() {
		return Range{End: Position{Character: 1}}
	}
	return rangeOf(src, span.Start.Offset, span.End.Offset)
}

func isIdentByte(b byte) bool {
	return b == '_' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9'
}

// wordAt returns the identifier that contains or ends at offset, with its
// byte range. ok is false when the cursor is not on an identifier.
func wordAt(src []byte, offset int) (word string, start, end int, ok bool) {
	if offset > len(src) {
		offset = len(src)
	}
	start, end = offset, offset
	for start > 0 && isIdentByte(src[start-1]) {
		start--
	}
	for end < len(src) && isIdentByte(src[end]) {
		end++
	}
	if start == end || src[start] >= '0' && src[start] <= '9' {
		return "", offset, offset, false
	}
	return string(src[start:end]), start, end, true
}

// prevNonSpace returns the byte before offset that is not a space or tab, and
// its index, or 0 and -1.
func prevNonSpace(src []byte, offset int) (byte, int) {
	for i := offset - 1; i >= 0; i-- {
		if src[i] != ' ' && src[i] != '\t' {
			return src[i], i
		}
	}
	return 0, -1
}

// nextNonSpace returns the first byte at or after offset that is not white
// space, or 0.
func nextNonSpace(src []byte, offset int) byte {
	for i := offset; i < len(src); i++ {
		switch src[i] {
		case ' ', '\t', '\n', '\r':
			continue
		}
		return src[i]
	}
	return 0
}

// inComment reports whether offset sits after `//` on its line.
func inComment(src []byte, offset int) bool {
	lineStart := offset
	for lineStart > 0 && src[lineStart-1] != '\n' {
		lineStart--
	}
	for i := lineStart; i+1 < offset && i+1 < len(src); i++ {
		if src[i] == '/' && src[i+1] == '/' {
			return true
		}
	}
	return false
}

// stripComments blanks `//` comments, keeping offsets, so lexical scans do
// not read words from prose.
func stripComments(src []byte) []byte {
	out := append([]byte(nil), src...)
	for i := 0; i+1 < len(out); i++ {
		if out[i] == '/' && out[i+1] == '/' {
			for i < len(out) && out[i] != '\n' {
				out[i] = ' '
				i++
			}
		}
	}
	return out
}
