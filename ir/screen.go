package ir

// Screen materials: a fullscreen pass that draws an author-written color with
// alpha into a rectangle the host chooses. This file holds the screen additions
// to the IR so the shared ir.go only needs the Kind comment.

// KindScreen is a screen pass. The emitter generates a fullscreen triangle
// vertex stage (WGSL and Metal from the vertex index, GLSL ES 3.00 from
// gl_VertexID, GLSL ES 1.00 from a host clip-space quad); the author body is
// the fragment stage. The v_uv varying is 0..1 across the target rectangle with
// the origin at the top left, on every target.
const KindScreen Kind = "screen"

// StageUsesRef reports whether stage reads the uniform or local called name.
func StageUsesRef(stage Stage, name string) bool {
	return stageMatches(stage, func(e Expr) bool {
		r, ok := e.(Ref)
		return ok && r.Name == name
	})
}
