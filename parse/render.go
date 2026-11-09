package parse

import (
	gts "github.com/odvcencio/gotreesitter"

	"m31labs.dev/selena/hir"
)

// renderBlock parses `render { key value ... }` into its settings. It keeps the
// words as written; lower validates them so a bad word reports a diagnostic code.
func (w *walker) renderBlock(n *gts.Node) (hir.RenderBlock, error) {
	rb := hir.RenderBlock{Span: w.span(n)}
	for i := 0; i < n.NamedChildCount(); i++ {
		c := n.NamedChild(i)
		if w.Type(c) != "render_setting" {
			continue
		}
		key, value := w.Field(c, "key"), w.Field(c, "value")
		rb.Settings = append(rb.Settings, hir.RenderSetting{
			Key:       w.Text(key),
			Value:     w.Text(value),
			Span:      w.span(c),
			KeySpan:   w.span(key),
			ValueSpan: w.span(value),
		})
	}
	return rb, nil
}
