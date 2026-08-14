package parse

import "testing"

// TestCommentsParseInEveryPosition guards the comment rule's visibility.
//
// `comment` was once `_comment`. A leading underscore makes a rule hidden, so
// comment tokens never appeared as nodes and their bytes belonged to no child
// in the tree. gotreesitter v0.49.0 added a leaf-tiling invariant that declines
// a subtree whose span contains bytes no child covers, and hidden extras are
// exactly that shape: from v0.49.0 onward, every .sel file containing a `//`
// comment failed with SEL0001 while comment-free sources still parsed.
//
// Keep the rule visible. If someone renames it back to `_comment`, or drops it
// from SetExtras, these cases fail again.
func TestCommentsParseInEveryPosition(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{
			name: "no comment",
			src: `material B {
    surface(geo) -> color { return rgb(1, 1, 1) }
}
`,
		},
		{
			name: "leading comment",
			src: `// leading comment
material B {
    surface(geo) -> color { return rgb(1, 1, 1) }
}
`,
		},
		{
			name: "several leading comments",
			src: `// one
// two
// three

material B {
    surface(geo) -> color { return rgb(1, 1, 1) }
}
`,
		},
		{
			name: "comment inside a body",
			src: `material B {
    surface(geo) -> color {
        // inner comment
        return rgb(1, 1, 1)
    }
}
`,
		},
		{
			name: "trailing comment after a statement",
			src: `material B {
    surface(geo) -> color {
        let c = rgb(1, 1, 1) // trailing
        return c
    }
}
`,
		},
		{
			name: "comment between declarations",
			src: `material A {
    surface(geo) -> color { return rgb(1, 1, 1) }
}
// between
material B {
    surface(geo) -> color { return rgb(0, 0, 0) }
}
`,
		},
		{
			name: "comment containing a slash",
			src: `// a // b / c
material B {
    surface(geo) -> color { return rgb(1, 1, 1) }
}
`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Material([]byte(tc.src)); err != nil {
				t.Fatalf("parse failed: %v\nsource:\n%s", err, tc.src)
			}
		})
	}
}
