package parse

import (
	"strings"
	"testing"
)

// TestBracketArrayParamReportsAtDeclaration covers the discoverability half of
// the fixed-size array param feature.
//
// `param rects : vec4[8]` is the spelling most authors reach for first. The
// Selena parse table drops the `[8]` without marking an error, so the material
// used to lower with a plain scalar vec4 uniform and the first complaint landed
// at the USE site — `rects[i]` reported SEL2001 "rects is not a local array or
// uniform array param", pointing at correct code. The declaration is where the
// mistake is, and the correct spelling belongs in the message.
func TestBracketArrayParamReportsAtDeclaration(t *testing.T) {
	cases := []string{
		"param rects : vec4[8]",
		"param rects : vec4 [ 8 ]",
	}
	for _, decl := range cases {
		t.Run(decl, func(t *testing.T) {
			_, err := Program([]byte("material P kind post {\n    " + decl + `
    surface(post) -> color {
        return sceneColor(post.uv)
    }
}
`))
			if err == nil {
				t.Fatal("expected an error for the C-style array param spelling")
			}
			msg := err.Error()
			if !strings.Contains(msg, "array<vec4, 8>") {
				t.Fatalf("error does not suggest the correct spelling: %s", msg)
			}
			// The diagnostic must anchor at the declaration (line 2), not at a
			// later use site.
			if !strings.HasPrefix(msg, "2:") {
				t.Fatalf("error is not anchored at the declaration line: %s", msg)
			}
		})
	}
}

// TestArrayParamCorrectSpellingParses is the positive control for the test
// above: the documented spelling must keep working.
func TestArrayParamCorrectSpellingParses(t *testing.T) {
	p, err := Program([]byte(`material P kind post {
    param rects : array<vec4, 8>
    surface(post) -> color {
        return sceneColor(post.uv)
    }
}
`))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Materials) != 1 || len(p.Materials[0].Params) != 1 {
		t.Fatalf("parsed program = %+v", p)
	}
	got := p.Materials[0].Params[0]
	if !got.IsArray || got.ArraySize != 8 || got.Type != "vec4" {
		t.Fatalf("param = %+v, want vec4 array of 8", got)
	}
}

// TestCommentsBetweenMembersAreNotDroppedTokens guards the gap scan against
// false positives: comments are lexer extras and never appear in the tree, so
// a comment between two members shows up as a gap.
func TestCommentsBetweenMembersAreNotDroppedTokens(t *testing.T) {
	_, err := Program([]byte(`material P kind post {
    // a leading comment
    param k : float = 1.0
    // a trailing comment
    surface(post) -> color {
        return sceneColor(post.uv)
    }
    // a closing comment
}
`))
	if err != nil {
		t.Fatalf("comments in the material body were reported as dropped tokens: %v", err)
	}
}
