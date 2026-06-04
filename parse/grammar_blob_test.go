package parse

import (
	"bytes"
	"testing"

	"github.com/odvcencio/gotreesitter/grammargen"
	"github.com/odvcencio/gotreesitter/taproot"

	"m31labs.dev/selena/grammar"
)

func TestGrammarBinIsCurrent(t *testing.T) {
	_, fresh, err := grammargen.GenerateLanguageAndBlob(grammar.SelenaGrammar())
	if err != nil {
		t.Fatalf("GenerateLanguageAndBlob: %v", err)
	}
	if !bytes.Equal(fresh, grammarBlob) {
		t.Fatalf("grammar.bin is stale (embedded %d bytes, regenerated %d bytes); run `go generate ./...`",
			len(grammarBlob), len(fresh))
	}
}

func TestEmbeddedGrammarBlobParses(t *testing.T) {
	if len(grammarBlob) == 0 {
		t.Fatal("grammar.bin embed is empty")
	}
	src := []byte(`material Blob {
    surface(geo) -> color { return rgb(1, 1, 1) }
}`)
	root, _, err := taproot.ParseFromBlob("selena-blob-test", grammarBlob, nil, src)
	if err != nil {
		t.Fatalf("ParseFromBlob: %v", err)
	}
	if root == nil || root.HasError() {
		t.Fatalf("embedded grammar parse failed")
	}
}
