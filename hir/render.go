package hir

// RenderSetting is one `key value` line of a render block, kept as the words the
// author wrote so lowering can reject an unknown word with its source position.
type RenderSetting struct {
	Key       string
	Value     string
	Span      Span // the whole setting
	KeySpan   Span
	ValueSpan Span
}

// RenderBlock is the `render { blend alpha  depth off }` block of a screen
// material. Selena only records the pipeline state it declares; the host applies
// it. Settings keep source order; a repeated key is rejected during lowering.
type RenderBlock struct {
	Settings []RenderSetting
	Span     Span
}
