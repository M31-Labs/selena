package lsp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"

	"m31labs.dev/selena/lower"
)

const sample = `// tint scales a color.
fn tint(c: vec3, k: float) -> vec3 {
    return c * k
}

material Base {
    param baseColor : color = rgb(0.8, 0.4, 0.2)
    param light : Sun
    context {
        cameraPos : vec3
        fog       : vec4
    }
    surface(geo) -> color {
        let n = normalize(geo.worldNormal)
        let lit = baseColor * (light.ambient + max(dot(n, light.dir), 0.0))
        return mlFog(tint(lit, 1.0), geo.worldPos)
    }
}

material Child extends Base {
    param glow : float = 0.5
    surface(geo) -> color {
        return super.surface(geo) * glow
    }
}
`

// at returns the position of the n-th occurrence (0-based) of needle, plus
// delta bytes.
func at(t *testing.T, src, needle string, n, delta int) Position {
	t.Helper()
	offset := -1
	for i := 0; i <= n; i++ {
		next := strings.Index(src[offset+1:], needle)
		if next < 0 {
			t.Fatalf("occurrence %d of %q not found", n, needle)
		}
		offset += 1 + next
	}
	return positionAt([]byte(src), offset+delta)
}

func open(t *testing.T, src string) (*Session, []Diagnostic) {
	t.Helper()
	s := NewSession(Options{})
	return s, s.Update("file:///tmp/m.sel", src)
}

func TestBuiltinDocsCoverEveryBuiltin(t *testing.T) {
	want := lower.Builtins()
	got := sortedKeys(builtinDocs)
	if !slices.Equal(got, want) {
		t.Fatalf("builtin docs out of sync with the lowerer\n docs:    %v\n builtins: %v", got, want)
	}
}

func TestCleanFileHasNoDiagnosticsAndLinksLibrary(t *testing.T) {
	_, diags := open(t, sample)
	if diags == nil || len(diags) != 0 {
		t.Fatalf("diagnostics = %+v, want an empty set", diags)
	}
}

func TestDiagnosticsCarryCodesAndRanges(t *testing.T) {
	for name, tc := range map[string]struct {
		src, code, at string
	}{
		"unknown name": {src: "material M {\n    surface(geo) -> color {\n        return missing\n    }\n}\n", code: "SEL2001", at: "missing"},
		"syntax":       {src: "material M {\n    surface(geo) -> color {\n        return rgb(1.0, 0.0\n    }\n}\n", code: "SEL0001"},
		"engine type":  {src: "material M {\n    context {\n        cameraPos : float\n    }\n    surface(geo) -> color {\n        return rgb(cameraPos, 0.0, 0.0)\n    }\n}\n", code: "SEL1007"},
	} {
		t.Run(name, func(t *testing.T) {
			_, diags := open(t, tc.src)
			if len(diags) == 0 {
				t.Fatal("no diagnostics")
			}
			d := diags[0]
			if d.Code != tc.code || d.Severity != SeverityError || d.Source != "selena" {
				t.Fatalf("diagnostic = %+v, want code %s", d, tc.code)
			}
			if tc.at != "" {
				want := at(t, tc.src, tc.at, 0, 0)
				if d.Range.Start != want {
					t.Fatalf("range start = %+v, want %+v (%q)", d.Range.Start, want, tc.at)
				}
			}
		})
	}
}

func TestHover(t *testing.T) {
	s, _ := open(t, sample)
	for name, tc := range map[string]struct {
		pos  Position
		want []string
	}{
		"param":            {at(t, sample, "baseColor *", 0, 2), []string{"(param) baseColor : color"}},
		"local type":       {at(t, sample, "dot(n,", 0, 4), []string{"(local) let n: vec3"}},
		"builtin":          {at(t, sample, "normalize(", 0, 1), []string{"normalize(v: T) -> T", "Selena builtin"}},
		"engine input":     {at(t, sample, "cameraPos :", 0, 1), []string{"(context) cameraPos : vec3", "Camera position", "selena.engine.v1"}},
		"geometry field":   {at(t, sample, "worldNormal", 0, 2), []string{"geo.worldNormal : vec3"}},
		"record field":     {at(t, sample, "light.dir", 0, 7), []string{"light.dir : vec3", "Sun"}},
		"own function":     {at(t, sample, "tint(lit", 0, 1), []string{"fn tint(c: vec3, k: float) -> vec3", "tint scales a color"}},
		"library function": {at(t, sample, "mlFog(", 0, 1), []string{"fn mlFog(color: vec3, worldPos: vec3) -> vec3", "module `lighting`"}},
		"inherited param":  {at(t, sample, "glow\n", 0, 0), []string{"(param) glow : float"}},
		"material":         {at(t, sample, "extends Base", 0, 9), []string{"material Base"}},
	} {
		t.Run(name, func(t *testing.T) {
			h := s.Hover("file:///tmp/m.sel", tc.pos)
			if h == nil {
				t.Fatal("no hover")
			}
			for _, want := range tc.want {
				if !strings.Contains(h.Contents.Value, want) {
					t.Errorf("hover %q lacks %q", h.Contents.Value, want)
				}
			}
		})
	}
	if h := s.Hover("file:///tmp/m.sel", at(t, sample, "tint scales", 0, 1)); h != nil {
		t.Fatalf("hover inside a comment = %+v", h)
	}
}

func labels(items []CompletionItem) []string {
	out := make([]string, len(items))
	for i, item := range items {
		out[i] = item.Label
	}
	return out
}

func TestCompletion(t *testing.T) {
	const src = `material M {
    param base : color
    param sun : Sun
    context {
        sun
    }
    surface(geo) -> color {
        let k = 1.0
        let c = geo.
        let s = sun.
        return mi
    }
}
`
	s, _ := open(t, src)
	uri := "file:///tmp/m.sel"
	geo := labels(s.Completion(uri, at(t, src, "geo.\n", 0, 4)))
	for _, want := range []string{"worldNormal", "uv", "worldPos", "position"} {
		if !slices.Contains(geo, want) {
			t.Errorf("geo. completion lacks %q: %v", want, geo)
		}
	}
	if slices.Contains(geo, "pointUV") {
		t.Errorf("mesh geo. completion offers a points field")
	}
	sun := labels(s.Completion(uri, at(t, src, "sun.\n", 0, 4)))
	if !slices.Equal(sun, []string{"ambient", "dir"}) {
		t.Errorf("Sun field completion = %v", sun)
	}
	ctx := s.Completion(uri, at(t, src, "sun\n", 0, 3))
	var dir *CompletionItem
	for i := range ctx {
		if ctx[i].Label == "sunDir" {
			dir = &ctx[i]
		}
	}
	if dir == nil || dir.InsertText != "sunDir : vec3" {
		t.Fatalf("context completion lacks sunDir with its declaration: %v", labels(ctx))
	}
	scope := labels(s.Completion(uri, at(t, src, "mi\n", 0, 2)))
	for _, want := range []string{"min", "mix"} {
		if !slices.Contains(scope, want) {
			t.Errorf("scope completion for %q lacks %q: %v", "mi", want, scope)
		}
	}
	all := labels(s.Completion(uri, at(t, src, "return mi", 0, 7)))
	for _, want := range []string{"base", "sun", "k", "geo", "let", "vec3f", "mlFog", "mlFogFactor"} {
		if !slices.Contains(all, want) {
			t.Errorf("scope completion lacks %q", want)
		}
	}
	const typed = "material M {\n    param p : \n    surface(geo) -> color { return rgb(1.0, 1.0, 1.0) }\n}\n"
	s.Update(uri, typed)
	types := labels(s.Completion(uri, at(t, typed, "p : ", 0, 4)))
	for _, want := range []string{"float", "vec3", "texture2d", "Sun"} {
		if !slices.Contains(types, want) {
			t.Errorf("type completion lacks %q: %v", want, types)
		}
	}
}

func TestDefinition(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	s, _ := open(t, sample)
	uri := "file:///tmp/m.sel"
	for name, tc := range map[string]struct{ from, to Position }{
		"local":    {at(t, sample, "dot(n,", 0, 4), at(t, sample, "n = normalize", 0, 0)},
		"function": {at(t, sample, "tint(lit", 0, 1), at(t, sample, "tint(c:", 0, 0)},
		"param":    {at(t, sample, "baseColor *", 0, 1), at(t, sample, "baseColor :", 0, 0)},
		"material": {at(t, sample, "extends Base", 0, 9), at(t, sample, "Base {", 0, 0)},
	} {
		t.Run(name, func(t *testing.T) {
			locs := s.Definition(uri, tc.from)
			if len(locs) != 1 || locs[0].URI != uri || locs[0].Range.Start != tc.to {
				t.Fatalf("definition = %+v, want %s at %+v", locs, uri, tc.to)
			}
		})
	}
	locs := s.Definition(uri, at(t, sample, "mlFog(", 0, 1))
	if len(locs) != 1 || !strings.HasSuffix(locs[0].URI, "/selena/materiallib/lighting.sel") {
		t.Fatalf("library definition = %+v", locs)
	}
	path := strings.TrimPrefix(locs[0].URI, "file://")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	line := strings.Split(string(data), "\n")[locs[0].Range.Start.Line]
	if !strings.Contains(line, "fn mlFog(") {
		t.Fatalf("library definition points at %q", line)
	}
}

func TestSessionKeepsLastProgramThroughSyntaxErrors(t *testing.T) {
	s, _ := open(t, sample)
	broken := strings.Replace(sample, "return super.surface(geo) * glow", "return super.surface(geo) * glow +", 1)
	diags := s.Update("file:///tmp/m.sel", broken)
	if len(diags) == 0 || diags[0].Code != "SEL0001" {
		t.Fatalf("diagnostics = %+v, want a syntax error", diags)
	}
	h := s.Hover("file:///tmp/m.sel", at(t, broken, "baseColor *", 0, 1))
	if h == nil || !strings.Contains(h.Contents.Value, "(param) baseColor") {
		t.Fatalf("hover after a syntax error = %+v", h)
	}
}

func TestServeSpeaksJSONRPC(t *testing.T) {
	var in bytes.Buffer
	send := func(v any) {
		if err := WriteMessage(&in, v); err != nil {
			t.Fatal(err)
		}
	}
	uri := "file:///tmp/m.sel"
	send(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{}})
	send(map[string]any{"jsonrpc": "2.0", "method": "textDocument/didOpen", "params": map[string]any{
		"textDocument": map[string]any{"uri": uri, "languageId": "selena", "version": 1, "text": sample}}})
	send(map[string]any{"jsonrpc": "2.0", "id": 2, "method": "textDocument/hover", "params": map[string]any{
		"textDocument": map[string]any{"uri": uri}, "position": at(t, sample, "normalize(", 0, 1)}})
	send(map[string]any{"jsonrpc": "2.0", "id": 3, "method": "shutdown"})
	send(map[string]any{"jsonrpc": "2.0", "method": "exit"})
	var out bytes.Buffer
	if err := Serve(&in, &out); err != nil {
		t.Fatal(err)
	}
	r := bufio.NewReader(&out)
	var messages []map[string]any
	for {
		payload, err := ReadMessage(r)
		if err != nil {
			break
		}
		var m map[string]any
		if err := json.Unmarshal(payload, &m); err != nil {
			t.Fatal(err)
		}
		messages = append(messages, m)
	}
	if len(messages) != 4 {
		t.Fatalf("got %d messages: %v", len(messages), messages)
	}
	caps := messages[0]["result"].(map[string]any)["capabilities"].(map[string]any)
	if caps["completionProvider"] == nil || caps["hoverProvider"] != true {
		t.Fatalf("capabilities = %v", caps)
	}
	if messages[1]["method"] != "textDocument/publishDiagnostics" {
		t.Fatalf("second message = %v", messages[1])
	}
	hover := messages[2]["result"].(map[string]any)["contents"].(map[string]any)["value"].(string)
	if !strings.Contains(hover, "normalize(v: T) -> T") {
		t.Fatalf("hover = %q", hover)
	}
}
